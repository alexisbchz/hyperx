// Package drw is the drawing layer for hypermenu. It mirrors the surface of
// dmenu's drw.c (rectangles, text, color schemes, mapping) but is backed by an
// in-memory RGBA framebuffer that is uploaded to an X11 pixmap via PutImage
// after each frame.
package drw

import (
	"image"
	"image/color"

	"github.com/alexisbchz/hyperx/fontcfg"
	"github.com/alexisbchz/hyperx/xproto"
)

// Color scheme indices, matching dmenu's enum.
const (
	ColFg = 0
	ColBg = 1
)

// Scheme is a (fg, bg) colour pair plus the X11 pixel values used by
// rectangle and clear operations on the GC.
type Scheme struct {
	Fg, Bg       color.RGBA
	FgPixel, BgPixel uint32
}

// Drw is the drawing context. It owns an off-screen pixmap on the server, a
// matching CPU-side RGBA buffer, and a GC for blitting between them and the
// destination window.
type Drw struct {
	C       *xproto.Conn
	Root    uint32
	Visual  uint32
	Depth   uint8
	W, H    uint16
	Pixmap  uint32
	GC      uint32

	Image   *image.RGBA // CPU framebuffer (RGBA), width = W, height = H

	Fonts   *FontSet
	Scheme  *Scheme

	DB      *fontcfg.DB
}

// New constructs a drawing context. The caller must already have computed the
// visual and depth (typically the default screen's RootVisual at RootDepth).
func New(c *xproto.Conn, root uint32, w, h uint16, db *fontcfg.DB) *Drw {
	depth := c.Screen.RootDepth
	visual := c.Screen.RootVisual
	pix := c.NewID()
	gc := c.NewID()
	c.CreatePixmap(pix, root, depth, w, h)
	c.CreateGC(gc, root, xproto.GCGraphicsExposures, 0)
	return &Drw{
		C:      c,
		Root:   root,
		Visual: visual,
		Depth:  depth,
		W:      w,
		H:      h,
		Pixmap: pix,
		GC:     gc,
		Image:  image.NewRGBA(image.Rect(0, 0, int(w), int(h))),
		DB:     db,
	}
}

// Resize reallocates the off-screen pixmap and CPU buffer.
func (d *Drw) Resize(w, h uint16) {
	if w == d.W && h == d.H {
		return
	}
	d.W = w
	d.H = h
	d.C.FreePixmap(d.Pixmap)
	d.Pixmap = d.C.NewID()
	d.C.CreatePixmap(d.Pixmap, d.Root, d.Depth, w, h)
	d.Image = image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
}

// Free releases pixmap and GC.
func (d *Drw) Free() {
	d.C.FreePixmap(d.Pixmap)
	d.C.FreeGC(d.GC)
}

// SetScheme switches the active colour scheme.
func (d *Drw) SetScheme(s *Scheme) { d.Scheme = s }

// Rect fills (or outlines) a rectangle using the active scheme.
// invert chooses Bg as the foreground (matches dmenu's drw_rect signature).
func (d *Drw) Rect(x, y int, w, h int, filled, invert bool) {
	if d.Scheme == nil {
		return
	}
	var c color.RGBA
	if invert {
		c = d.Scheme.Bg
	} else {
		c = d.Scheme.Fg
	}
	r := image.Rect(x, y, x+w, y+h)
	if filled {
		FillRect(d.Image, r, c)
	} else {
		// 1-pixel outline.
		FillRect(d.Image, image.Rect(x, y, x+w, y+1), c)
		FillRect(d.Image, image.Rect(x, y+h-1, x+w, y+h), c)
		FillRect(d.Image, image.Rect(x, y, x+1, y+h), c)
		FillRect(d.Image, image.Rect(x+w-1, y, x+w, y+h), c)
	}
}

// Clear fills the entire framebuffer with the active scheme's background.
func (d *Drw) Clear() {
	if d.Scheme == nil {
		return
	}
	FillRect(d.Image, d.Image.Bounds(), d.Scheme.Bg)
}

// TextWidth measures the rendered width of s (without padding).
func (d *Drw) TextWidth(s string) int {
	if d.Fonts == nil {
		return 0
	}
	return d.Fonts.MeasureString(s)
}

// TextWidthClamp returns the width of s clamped to n pixels.
func (d *Drw) TextWidthClamp(s string, n int) int {
	if d.Fonts == nil {
		return 0
	}
	w, _ := d.Fonts.MeasureStringClamp(s, n)
	if w > n {
		return n
	}
	return w
}

// FontHeight returns the line height of the primary font.
func (d *Drw) FontHeight() int {
	if d.Fonts == nil {
		return 0
	}
	return d.Fonts.Height()
}

// Text draws an item-like text region: clears (x,y,w,h) with bg, then
// renders s with horizontal padding lpad, vertically centred. If s overflows
// w, an ellipsis is rendered. invert swaps fg/bg. Returns the new x cursor.
func (d *Drw) Text(x, y int, w, h int, lpad int, s string, invert bool) int {
	if d.Scheme == nil || d.Fonts == nil || w == 0 {
		return x
	}
	fg := d.Scheme.Fg
	bg := d.Scheme.Bg
	if invert {
		fg, bg = bg, fg
	}
	FillRect(d.Image, image.Rect(x, y, x+w, y+h), bg)
	if w < lpad {
		return x + w
	}
	innerX := x + lpad
	innerW := w - lpad
	// Compute clamp; if overflow, append ellipsis.
	textWidth := d.Fonts.MeasureString(s)
	drawn := s
	if textWidth > innerW {
		ellipsis := "..."
		ew := d.Fonts.MeasureString(ellipsis)
		room := innerW - ew
		if room < 0 {
			room = 0
		}
		_, fit := d.Fonts.MeasureStringClamp(s, room)
		drawn = s[:fit] + ellipsis
	}
	baseline := y + (h-d.Fonts.Height())/2 + d.Fonts.Primary.Ascent
	d.Fonts.DrawString(d.Image, innerX, baseline, drawn, fg)
	return x + w
}

// Map uploads the framebuffer rectangle (x,y,w,h) to win via the pixmap.
// dmenu calls this drw_map.
func (d *Drw) Map(win uint32, x, y, w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	// Upload only the dirty rectangle as RGBA (we use 32-bit BGRA which matches
	// X's ZPixmap layout for depth-24 little-endian).
	stride := d.Image.Stride
	src := d.Image.Pix
	// Build a packed buffer for the requested rect.
	row := make([]byte, w*4*h)
	for j := 0; j < h; j++ {
		srcRow := src[(y+j)*stride+x*4 : (y+j)*stride+(x+w)*4]
		// Convert RGBA -> BGRA (X11 default truecolor stores BGRX in
		// little-endian for depth 24).
		for i := 0; i < w; i++ {
			r := srcRow[i*4+0]
			g := srcRow[i*4+1]
			b := srcRow[i*4+2]
			// alpha discarded
			row[(j*w+i)*4+0] = b
			row[(j*w+i)*4+1] = g
			row[(j*w+i)*4+2] = r
			row[(j*w+i)*4+3] = 0
		}
	}
	d.C.PutImage(2, /* ZPixmap */
		d.Pixmap, d.GC,
		uint16(w), uint16(h),
		int16(x), int16(y),
		0, d.Depth,
		row)
	d.C.CopyArea(d.Pixmap, win, d.GC, int16(x), int16(y), int16(x), int16(y), uint16(w), uint16(h))
}
