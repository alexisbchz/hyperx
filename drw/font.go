package drw

import (
	"image"
	"image/color"
	"image/draw"
	"sync"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/alexisbchz/hyperx/fontcfg"
)

// Font is a sized scalable font.
type Font struct {
	Info    *fontcfg.FontInfo
	Size    float64 // points
	DPI     float64
	face    font.Face
	metrics font.Metrics
	Ascent  int // px
	Descent int // px
	Height  int // px (ascent+descent, mirrors Xft's font->ascent + font->descent)

	mu        sync.Mutex
	glyphMu   sync.Mutex
	glyphAdv  map[rune]int // pixel advance cache
	hasRune   map[rune]bool
}

// LoadFont opens a font face from a fontcfg.FontInfo at the given point size.
func LoadFont(info *fontcfg.FontInfo, size, dpi float64) (*Font, error) {
	if dpi == 0 {
		dpi = 96
	}
	sf, err := info.Load()
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(sf, &opentype.FaceOptions{
		Size:    size,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	m := face.Metrics()
	asc := (m.Ascent.Round())
	desc := (m.Descent.Round())
	return &Font{
		Info:     info,
		Size:     size,
		DPI:      dpi,
		face:     face,
		metrics:  m,
		Ascent:   asc,
		Descent:  desc,
		Height:   asc + desc,
		glyphAdv: make(map[rune]int),
		hasRune:  make(map[rune]bool),
	}, nil
}

// Has reports whether the font has a glyph for r.
func (f *Font) Has(r rune) bool {
	f.glyphMu.Lock()
	if v, ok := f.hasRune[r]; ok {
		f.glyphMu.Unlock()
		return v
	}
	f.glyphMu.Unlock()
	sf, err := f.Info.Load()
	if err != nil {
		return false
	}
	var buf sfnt.Buffer
	idx, err := sf.GlyphIndex(&buf, r)
	has := err == nil && idx != 0
	f.glyphMu.Lock()
	f.hasRune[r] = has
	f.glyphMu.Unlock()
	return has
}

// Advance returns the horizontal advance for r in pixels.
func (f *Font) Advance(r rune) int {
	f.glyphMu.Lock()
	if v, ok := f.glyphAdv[r]; ok {
		f.glyphMu.Unlock()
		return v
	}
	f.glyphMu.Unlock()
	adv, ok := f.face.GlyphAdvance(r)
	if !ok {
		adv = fixed.I(0)
	}
	px := adv.Round()
	f.glyphMu.Lock()
	f.glyphAdv[r] = px
	f.glyphMu.Unlock()
	return px
}

// MeasureString returns the rendered width of s in pixels using this font.
// Multi-font fallback isn't applied here: callers wanting fallback measure
// each rune through FontSet.
func (f *Font) MeasureString(s string) int {
	w := 0
	for _, r := range s {
		w += f.Advance(r)
	}
	return w
}

// DrawString rasterizes s onto img with foreground colour fg, baseline at (x,y).
// Returns the new x cursor position.
func (f *Font) DrawString(img *image.RGBA, x, y int, s string, fg color.RGBA) int {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(fg),
		Face: f.face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
	return d.Dot.X.Round()
}

// FontSet is an ordered list of fonts for fallback. The first is the primary;
// subsequent fonts cover glyphs the primary lacks.
type FontSet struct {
	Primary *Font
	Fallback []*Font // searched in order
	DB       *fontcfg.DB
	dpi      float64

	// glyph fallback cache: rune → font (nil = primary).
	fbMu sync.Mutex
	fb   map[rune]*Font
}

// NewFontSet creates a FontSet rooted at primary. fb is used to look up
// fallback fonts for individual runes lazily.
func NewFontSet(primary *Font, db *fontcfg.DB) *FontSet {
	return &FontSet{
		Primary: primary,
		DB:      db,
		dpi:     primary.DPI,
		fb:      make(map[rune]*Font),
	}
}

// AddFallback appends a manual fallback font (e.g. for explicit -fn arguments
// with multiple families).
func (fs *FontSet) AddFallback(f *Font) {
	fs.Fallback = append(fs.Fallback, f)
}

// fontFor returns the font that should render r.
func (fs *FontSet) fontFor(r rune) *Font {
	if fs.Primary.Has(r) {
		return fs.Primary
	}
	for _, f := range fs.Fallback {
		if f.Has(r) {
			return f
		}
	}
	fs.fbMu.Lock()
	if cached, ok := fs.fb[r]; ok {
		fs.fbMu.Unlock()
		if cached == nil {
			return fs.Primary
		}
		return cached
	}
	fs.fbMu.Unlock()
	if fs.DB != nil {
		if info := fs.DB.MatchForRune(r); info != nil {
			f, err := LoadFont(info, fs.Primary.Size, fs.dpi)
			if err == nil {
				fs.fbMu.Lock()
				fs.fb[r] = f
				fs.fbMu.Unlock()
				fs.Fallback = append(fs.Fallback, f)
				return f
			}
		}
	}
	fs.fbMu.Lock()
	fs.fb[r] = nil
	fs.fbMu.Unlock()
	return fs.Primary
}

// Height returns the primary font's height (matches dmenu's font->h).
func (fs *FontSet) Height() int { return fs.Primary.Height }

// MeasureString returns the pixel width of s using fallback fonts per rune.
func (fs *FontSet) MeasureString(s string) int {
	w := 0
	for _, r := range s {
		w += fs.fontFor(r).Advance(r)
	}
	return w
}

// MeasureStringClamp returns the pixel width that fits within n pixels and
// the byte index up to which the string fits (UTF-8-safe).
func (fs *FontSet) MeasureStringClamp(s string, n int) (width, fit int) {
	w := 0
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		adv := fs.fontFor(r).Advance(r)
		if w+adv > n {
			break
		}
		w += adv
		i += size
	}
	return w, i
}

// DrawString rasterizes s onto img at (x, baselineY) using fallback fonts as
// needed. Returns the final x cursor.
func (fs *FontSet) DrawString(img *image.RGBA, x, baselineY int, s string, fg color.RGBA) int {
	for _, r := range s {
		f := fs.fontFor(r)
		x = f.DrawString(img, x, baselineY, string(r), fg)
	}
	return x
}

// FillRect fills a rectangle on img with c.
func FillRect(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(img, r.Intersect(img.Bounds()), &image.Uniform{C: c}, image.Point{}, draw.Src)
}
