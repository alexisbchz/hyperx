package xproto

import (
	"encoding/binary"
	"fmt"
)

// Window/event masks.
const (
	EventMaskKeyPress           = 1 << 0
	EventMaskKeyRelease         = 1 << 1
	EventMaskButtonPress        = 1 << 2
	EventMaskButtonRelease      = 1 << 3
	EventMaskEnterWindow        = 1 << 4
	EventMaskLeaveWindow        = 1 << 5
	EventMaskPointerMotion      = 1 << 6
	EventMaskPointerMotionHint  = 1 << 7
	EventMaskButton1Motion      = 1 << 8
	EventMaskButton2Motion      = 1 << 9
	EventMaskButton3Motion      = 1 << 10
	EventMaskButton4Motion      = 1 << 11
	EventMaskButton5Motion      = 1 << 12
	EventMaskButtonMotion       = 1 << 13
	EventMaskKeymapState        = 1 << 14
	EventMaskExposure           = 1 << 15
	EventMaskVisibility         = 1 << 16
	EventMaskStructureNotify    = 1 << 17
	EventMaskSubstructureNotify = 1 << 19
	EventMaskFocusChange        = 1 << 21
	EventMaskPropertyChange     = 1 << 22
)

// CW (CreateWindow value mask) bits.
const (
	CWBackPixel        = 1 << 1
	CWBorderPixel      = 1 << 3
	CWOverrideRedirect = 1 << 9
	CWEventMask        = 1 << 11
	CWColormap         = 1 << 13
)

// GC value mask bits. Order matters — when emitting a value list to the wire,
// values must appear in ascending bit order.
const (
	GCFunction          = 1 << 0
	GCPlaneMask         = 1 << 1
	GCForeground        = 1 << 2
	GCBackground        = 1 << 3
	GCLineWidth         = 1 << 4
	GCLineStyle         = 1 << 5
	GCCapStyle          = 1 << 6
	GCJoinStyle         = 1 << 7
	GCFillStyle         = 1 << 8
	GCFillRule          = 1 << 9
	GCTile              = 1 << 10
	GCStipple           = 1 << 11
	GCTileStippleXOrig  = 1 << 12
	GCTileStippleYOrig  = 1 << 13
	GCFont              = 1 << 14
	GCSubwindowMode     = 1 << 15
	GCGraphicsExposures = 1 << 16
	GCClipXOrigin       = 1 << 17
	GCClipYOrigin       = 1 << 18
	GCClipMask          = 1 << 19
	GCDashOffset        = 1 << 20
	GCDashList          = 1 << 21
	GCArcMode           = 1 << 22
)

// GC function ops.
const (
	GXclear        = 0
	GXand          = 1
	GXandReverse   = 2
	GXcopy         = 3
	GXandInverted  = 4
	GXnoop         = 5
	GXxor          = 6
	GXor           = 7
	GXnor          = 8
	GXequiv        = 9
	GXinvert       = 10
	GXorReverse    = 11
	GXcopyInverted = 12
	GXorInverted   = 13
	GXnand         = 14
	GXset          = 15
)

// Subwindow mode (GCSubwindowMode value).
const (
	ClipByChildren   = 0
	IncludeInferiors = 1
)

// GetImage format.
const (
	ImageFormatBitmap   = 0
	ImageFormatXYPixmap = 1
	ImageFormatZPixmap  = 2
)

// GrabPointer / GrabKeyboard status codes.
const (
	GrabSuccess        = 0
	GrabAlreadyGrabbed = 1
	GrabInvalidTime    = 2
	GrabNotViewable    = 3
	GrabFrozen         = 4
)

// PropMode for ChangeProperty.
const (
	PropReplace = 0
	PropPrepend = 1
	PropAppend  = 2
)

// Atoms (predefined).
const (
	AtomNone       = 0
	AtomPrimary    = 1
	AtomSecondary  = 2
	AtomArc        = 3
	AtomAtom       = 4
	AtomBitmap     = 5
	AtomCardinal   = 6
	AtomColormap   = 7
	AtomCursor     = 8
	AtomString     = 31
	AtomWmName     = 39
	AtomWmIconName = 37
	AtomWmClass    = 67
)

// Grab modes.
const (
	GrabModeSync  = 0
	GrabModeAsync = 1
)

// Revert-to.
const (
	RevertToNone        = 0
	RevertToPointerRoot = 1
	RevertToParent      = 2
)

// Time, Window special values.
const (
	CurrentTime = 0
	WindowNone  = 0
)

// SendBuf encodes a request. Length in 4-byte units is written into bytes 2-3.
type sendBuf struct {
	b []byte
}

func newReq(op uint8, data uint8) *sendBuf {
	return &sendBuf{b: []byte{op, data, 0, 0}}
}

func (s *sendBuf) u8(v uint8)   { s.b = append(s.b, v) }
func (s *sendBuf) u16(bo binary.ByteOrder, v uint16) {
	var t [2]byte
	bo.PutUint16(t[:], v)
	s.b = append(s.b, t[:]...)
}
func (s *sendBuf) u32(bo binary.ByteOrder, v uint32) {
	var t [4]byte
	bo.PutUint32(t[:], v)
	s.b = append(s.b, t[:]...)
}
func (s *sendBuf) bytes(p []byte) { s.b = append(s.b, p...) }
func (s *sendBuf) pad(n int) {
	if r := n & 3; r != 0 {
		s.b = append(s.b, make([]byte, 4-r)...)
	}
}
func (s *sendBuf) finish(bo binary.ByteOrder) []byte {
	if len(s.b)%4 != 0 {
		panic(fmt.Sprintf("xproto: request length %d not multiple of 4", len(s.b)))
	}
	bo.PutUint16(s.b[2:4], uint16(len(s.b)/4))
	return s.b
}

// ----------------------------------------------------------------------------
// InternAtom (opcode 16)
// ----------------------------------------------------------------------------

func (c *Conn) InternAtom(name string, onlyIfExists bool) (uint32, error) {
	r := newReq(16, btoi(onlyIfExists))
	r.u16(c.bo, uint16(len(name)))
	r.u16(c.bo, 0)
	r.bytes([]byte(name))
	r.pad(len(name))
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return 0, err
	}
	return c.bo.Uint32(rep[8:12]), nil
}

func btoi(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// ----------------------------------------------------------------------------
// QueryExtension (opcode 98)
// ----------------------------------------------------------------------------

// ExtensionInfo describes a server extension.
type ExtensionInfo struct {
	Present      bool
	MajorOpcode  uint8
	FirstEvent   uint8
	FirstError   uint8
}

func (c *Conn) QueryExtension(name string) (*ExtensionInfo, error) {
	r := newReq(98, 0)
	r.u16(c.bo, uint16(len(name)))
	r.u16(c.bo, 0)
	r.bytes([]byte(name))
	r.pad(len(name))
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return nil, err
	}
	return &ExtensionInfo{
		Present:     rep[8] != 0,
		MajorOpcode: rep[9],
		FirstEvent:  rep[10],
		FirstError:  rep[11],
	}, nil
}

// ----------------------------------------------------------------------------
// CreateWindow (opcode 1)
// ----------------------------------------------------------------------------

type CreateWindowReq struct {
	WID                       uint32
	Parent                    uint32
	Depth                     uint8
	X, Y                      int16
	Width, Height             uint16
	BorderWidth               uint16
	Class                     uint16 // 0=CopyFromParent, 1=InputOutput, 2=InputOnly
	Visual                    uint32 // 0 = CopyFromParent
	BackPixel                 uint32
	BorderPixel               uint32
	Colormap                  uint32
	OverrideRedirect          bool
	EventMask                 uint32
	SetBackPixel              bool
	SetBorderPixel            bool
	SetOverrideRedirect       bool
	SetEventMask              bool
	SetColormap               bool
}

func (c *Conn) CreateWindow(req CreateWindowReq) error {
	r := newReq(1, req.Depth)
	r.u32(c.bo, req.WID)
	r.u32(c.bo, req.Parent)
	r.u16(c.bo, uint16(req.X))
	r.u16(c.bo, uint16(req.Y))
	r.u16(c.bo, req.Width)
	r.u16(c.bo, req.Height)
	r.u16(c.bo, req.BorderWidth)
	r.u16(c.bo, req.Class)
	r.u32(c.bo, req.Visual)
	var mask uint32
	var vals []uint32
	if req.SetBackPixel {
		mask |= CWBackPixel
		vals = append(vals, req.BackPixel)
	}
	if req.SetBorderPixel {
		mask |= CWBorderPixel
		vals = append(vals, req.BorderPixel)
	}
	if req.SetOverrideRedirect {
		mask |= CWOverrideRedirect
		var v uint32
		if req.OverrideRedirect {
			v = 1
		}
		vals = append(vals, v)
	}
	if req.SetEventMask {
		mask |= CWEventMask
		vals = append(vals, req.EventMask)
	}
	if req.SetColormap {
		mask |= CWColormap
		vals = append(vals, req.Colormap)
	}
	r.u32(c.bo, mask)
	for _, v := range vals {
		r.u32(c.bo, v)
	}
	c.Send(r.finish(c.bo))
	return nil
}

// ----------------------------------------------------------------------------
// ChangeWindowAttributes (opcode 2)
// ----------------------------------------------------------------------------

// ChangeWindowAttributes updates a window's attributes. mask uses the same
// CW* bits as CreateWindow; the values list must match the bits in mask.
func (c *Conn) ChangeWindowAttributes(w uint32, mask uint32, values ...uint32) {
	r := newReq(2, 0)
	r.u32(c.bo, w)
	r.u32(c.bo, mask)
	for _, v := range values {
		r.u32(c.bo, v)
	}
	c.Send(r.finish(c.bo))
}

// SelectInput is a convenience that only updates the event mask.
func (c *Conn) SelectInput(w uint32, mask uint32) {
	c.ChangeWindowAttributes(w, CWEventMask, mask)
}

// ----------------------------------------------------------------------------
// ReparentWindow (opcode 7)
// ----------------------------------------------------------------------------

func (c *Conn) ReparentWindow(window, parent uint32, x, y int16) {
	r := newReq(7, 0)
	r.u32(c.bo, window)
	r.u32(c.bo, parent)
	r.u16(c.bo, uint16(x))
	r.u16(c.bo, uint16(y))
	c.Send(r.finish(c.bo))
}

// ----------------------------------------------------------------------------
// MapWindow (8), UnmapWindow (10), MapRaised approximated via MapWindow.
// ----------------------------------------------------------------------------

func (c *Conn) MapWindow(w uint32) {
	r := newReq(8, 0)
	r.u32(c.bo, w)
	c.Send(r.finish(c.bo))
}

func (c *Conn) UnmapWindow(w uint32) {
	r := newReq(10, 0)
	r.u32(c.bo, w)
	c.Send(r.finish(c.bo))
}

// ConfigureWindow (opcode 12). Mask bits: X=1,Y=2,W=4,H=8,BW=16,Sibling=32,StackMode=64.
func (c *Conn) ConfigureWindow(w uint32, mask uint16, values ...uint32) {
	r := newReq(12, 0)
	r.u32(c.bo, w)
	r.u16(c.bo, mask)
	r.u16(c.bo, 0)
	for _, v := range values {
		r.u32(c.bo, v)
	}
	c.Send(r.finish(c.bo))
}

// RaiseWindow uses ConfigureWindow stack-mode = Above (0).
func (c *Conn) RaiseWindow(w uint32) {
	c.ConfigureWindow(w, 0x40, 0) // stack-mode=Above
}

// ----------------------------------------------------------------------------
// ChangeProperty (opcode 18)
// ----------------------------------------------------------------------------

func (c *Conn) ChangeProperty(mode uint8, window, property, typ uint32, format uint8, data []byte) {
	r := newReq(18, mode)
	r.u32(c.bo, window)
	r.u32(c.bo, property)
	r.u32(c.bo, typ)
	r.u8(format)
	r.u8(0)
	r.u8(0)
	r.u8(0)
	// length in units of format
	unit := int(format) / 8
	if unit == 0 {
		unit = 1
	}
	r.u32(c.bo, uint32(len(data)/unit))
	r.bytes(data)
	r.pad(len(data))
	c.Send(r.finish(c.bo))
}

// SetClassHint sets WM_CLASS to "instance\0class\0".
func (c *Conn) SetClassHint(window uint32, instance, class string) {
	buf := append([]byte(instance), 0)
	buf = append(buf, []byte(class)...)
	buf = append(buf, 0)
	c.ChangeProperty(PropReplace, window, AtomWmClass, AtomString, 8, buf)
}

// ----------------------------------------------------------------------------
// GetProperty (opcode 20)
// ----------------------------------------------------------------------------

type GetPropertyReply struct {
	Format     uint8
	Type       uint32
	BytesAfter uint32
	Value      []byte
}

func (c *Conn) GetProperty(delete bool, window, property, typ uint32, longOffset, longLength uint32) (*GetPropertyReply, error) {
	r := newReq(20, btoi(delete))
	r.u32(c.bo, window)
	r.u32(c.bo, property)
	r.u32(c.bo, typ)
	r.u32(c.bo, longOffset)
	r.u32(c.bo, longLength)
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return nil, err
	}
	format := rep[1]
	tp := c.bo.Uint32(rep[8:12])
	ba := c.bo.Uint32(rep[12:16])
	nItems := c.bo.Uint32(rep[16:20])
	unit := int(format) / 8
	val := append([]byte(nil), rep[32:32+int(nItems)*unit]...)
	return &GetPropertyReply{Format: format, Type: tp, BytesAfter: ba, Value: val}, nil
}

// ----------------------------------------------------------------------------
// CreatePixmap (53), FreePixmap (54)
// ----------------------------------------------------------------------------

func (c *Conn) CreatePixmap(pid, drawable uint32, depth uint8, w, h uint16) {
	r := newReq(53, depth)
	r.u32(c.bo, pid)
	r.u32(c.bo, drawable)
	r.u16(c.bo, w)
	r.u16(c.bo, h)
	c.Send(r.finish(c.bo))
}

func (c *Conn) FreePixmap(pid uint32) {
	r := newReq(54, 0)
	r.u32(c.bo, pid)
	c.Send(r.finish(c.bo))
}

// ----------------------------------------------------------------------------
// CreateGC (55), ChangeGC (56), FreeGC (60)
// ----------------------------------------------------------------------------

func (c *Conn) CreateGC(gc, drawable uint32, mask uint32, values ...uint32) {
	r := newReq(55, 0)
	r.u32(c.bo, gc)
	r.u32(c.bo, drawable)
	r.u32(c.bo, mask)
	for _, v := range values {
		r.u32(c.bo, v)
	}
	c.Send(r.finish(c.bo))
}

func (c *Conn) ChangeGC(gc uint32, mask uint32, values ...uint32) {
	r := newReq(56, 0)
	r.u32(c.bo, gc)
	r.u32(c.bo, mask)
	for _, v := range values {
		r.u32(c.bo, v)
	}
	c.Send(r.finish(c.bo))
}

func (c *Conn) FreeGC(gc uint32) {
	r := newReq(60, 0)
	r.u32(c.bo, gc)
	c.Send(r.finish(c.bo))
}

// ----------------------------------------------------------------------------
// PolyFillRectangle (70), PolyRectangle (67), CopyArea (62)
// ----------------------------------------------------------------------------

type Rect struct {
	X, Y          int16
	Width, Height uint16
}

func (c *Conn) PolyFillRectangle(drawable, gc uint32, rects []Rect) {
	r := newReq(70, 0)
	r.u32(c.bo, drawable)
	r.u32(c.bo, gc)
	for _, rc := range rects {
		r.u16(c.bo, uint16(rc.X))
		r.u16(c.bo, uint16(rc.Y))
		r.u16(c.bo, rc.Width)
		r.u16(c.bo, rc.Height)
	}
	c.Send(r.finish(c.bo))
}

func (c *Conn) PolyRectangle(drawable, gc uint32, rects []Rect) {
	r := newReq(67, 0)
	r.u32(c.bo, drawable)
	r.u32(c.bo, gc)
	for _, rc := range rects {
		r.u16(c.bo, uint16(rc.X))
		r.u16(c.bo, uint16(rc.Y))
		r.u16(c.bo, rc.Width)
		r.u16(c.bo, rc.Height)
	}
	c.Send(r.finish(c.bo))
}

func (c *Conn) CopyArea(src, dst, gc uint32, srcX, srcY, dstX, dstY int16, w, h uint16) {
	r := newReq(62, 0)
	r.u32(c.bo, src)
	r.u32(c.bo, dst)
	r.u32(c.bo, gc)
	r.u16(c.bo, uint16(srcX))
	r.u16(c.bo, uint16(srcY))
	r.u16(c.bo, uint16(dstX))
	r.u16(c.bo, uint16(dstY))
	r.u16(c.bo, w)
	r.u16(c.bo, h)
	c.Send(r.finish(c.bo))
}

// ----------------------------------------------------------------------------
// PutImage (opcode 72)
// format: 0=Bitmap, 1=XYPixmap, 2=ZPixmap
// ----------------------------------------------------------------------------

func (c *Conn) PutImage(format uint8, drawable, gc uint32, w, h uint16, dstX, dstY int16,
	leftPad, depth uint8, data []byte) {
	// Request is variable; chunk if larger than max request length.
	maxReq := int(c.Setup.MaximumRequestLength) * 4
	if maxReq == 0 {
		maxReq = 256 * 1024
	}
	headerLen := 24
	bytesPerLine := int(w) * int(depth) / 8 // assume depth divisible by 8 for 24/32
	if int(depth) == 24 {
		bytesPerLine = int(w) * 4 // Z-format depth 24 is usually 32 bpp
	}
	maxRows := (maxReq - headerLen) / bytesPerLine
	if maxRows < 1 {
		maxRows = 1
	}
	row := 0
	for row < int(h) {
		rows := int(h) - row
		if rows > maxRows {
			rows = maxRows
		}
		chunk := data[row*bytesPerLine : (row+rows)*bytesPerLine]
		r := newReq(72, format)
		r.u32(c.bo, drawable)
		r.u32(c.bo, gc)
		r.u16(c.bo, w)
		r.u16(c.bo, uint16(rows))
		r.u16(c.bo, uint16(dstX))
		r.u16(c.bo, uint16(dstY)+uint16(row))
		r.u8(leftPad)
		r.u8(depth)
		r.u16(c.bo, 0)
		r.bytes(chunk)
		r.pad(len(chunk))
		c.Send(r.finish(c.bo))
		row += rows
	}
}

// ----------------------------------------------------------------------------
// GrabKeyboard (31), UngrabKeyboard (32)
// ----------------------------------------------------------------------------

func (c *Conn) GrabKeyboard(window uint32, ownerEvents bool, pointerMode, keyboardMode uint8, time uint32) (uint8, error) {
	r := newReq(31, btoi(ownerEvents))
	r.u32(c.bo, window)
	r.u32(c.bo, time)
	r.u8(pointerMode)
	r.u8(keyboardMode)
	r.u16(c.bo, 0)
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return 0, err
	}
	return rep[1], nil // 0 = success
}

func (c *Conn) UngrabKeyboard(time uint32) {
	r := newReq(32, 0)
	r.u32(c.bo, time)
	c.Send(r.finish(c.bo))
}

// ----------------------------------------------------------------------------
// GetInputFocus (43), SetInputFocus (42)
// ----------------------------------------------------------------------------

type FocusReply struct {
	RevertTo uint8
	Focus    uint32
}

func (c *Conn) GetInputFocus() (*FocusReply, error) {
	r := newReq(43, 0)
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return nil, err
	}
	return &FocusReply{RevertTo: rep[1], Focus: c.bo.Uint32(rep[8:12])}, nil
}

func (c *Conn) SetInputFocus(window uint32, revertTo uint8, time uint32) {
	r := newReq(42, revertTo)
	r.u32(c.bo, window)
	r.u32(c.bo, time)
	c.Send(r.finish(c.bo))
}

// ----------------------------------------------------------------------------
// QueryTree (15), GetGeometry (14), GetWindowAttributes (3), QueryPointer (38)
// ----------------------------------------------------------------------------

type QueryTreeReply struct {
	Root, Parent uint32
	Children     []uint32
}

func (c *Conn) QueryTree(w uint32) (*QueryTreeReply, error) {
	r := newReq(15, 0)
	r.u32(c.bo, w)
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return nil, err
	}
	root := c.bo.Uint32(rep[8:12])
	parent := c.bo.Uint32(rep[12:16])
	n := c.bo.Uint16(rep[16:18])
	children := make([]uint32, n)
	for i := uint16(0); i < n; i++ {
		children[i] = c.bo.Uint32(rep[32+int(i)*4 : 36+int(i)*4])
	}
	return &QueryTreeReply{Root: root, Parent: parent, Children: children}, nil
}

type GeometryReply struct {
	Depth         uint8
	Root          uint32
	X, Y          int16
	Width, Height uint16
	BorderWidth   uint16
}

func (c *Conn) GetGeometry(drawable uint32) (*GeometryReply, error) {
	r := newReq(14, 0)
	r.u32(c.bo, drawable)
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return nil, err
	}
	return &GeometryReply{
		Depth:       rep[1],
		Root:        c.bo.Uint32(rep[8:12]),
		X:           int16(c.bo.Uint16(rep[12:14])),
		Y:           int16(c.bo.Uint16(rep[14:16])),
		Width:       c.bo.Uint16(rep[16:18]),
		Height:      c.bo.Uint16(rep[18:20]),
		BorderWidth: c.bo.Uint16(rep[20:22]),
	}, nil
}

type WindowAttributes struct {
	BackingStore    uint8
	Visual          uint32
	Class           uint16
	BitGravity      uint8
	WinGravity      uint8
	BackingPlanes   uint32
	BackingPixel    uint32
	SaveUnder       uint8
	MapIsInstalled  uint8
	MapState        uint8
	OverrideRedirect uint8
	Colormap        uint32
	AllEventMasks   uint32
	YourEventMask   uint32
	DoNotPropagate  uint16
}

func (c *Conn) GetWindowAttributes(w uint32) (*WindowAttributes, error) {
	r := newReq(3, 0)
	r.u32(c.bo, w)
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return nil, err
	}
	return &WindowAttributes{
		BackingStore:     rep[1],
		Visual:           c.bo.Uint32(rep[8:12]),
		Class:            c.bo.Uint16(rep[12:14]),
		BitGravity:       rep[14],
		WinGravity:       rep[15],
		BackingPlanes:    c.bo.Uint32(rep[16:20]),
		BackingPixel:     c.bo.Uint32(rep[20:24]),
		SaveUnder:        rep[24],
		MapIsInstalled:   rep[25],
		MapState:         rep[26],
		OverrideRedirect: rep[27],
		Colormap:         c.bo.Uint32(rep[28:32]),
		AllEventMasks:    c.bo.Uint32(rep[32:36]),
		YourEventMask:    c.bo.Uint32(rep[36:40]),
		DoNotPropagate:   c.bo.Uint16(rep[40:42]),
	}, nil
}

type PointerReply struct {
	SameScreen bool
	Root, Child uint32
	RootX, RootY, WinX, WinY int16
	Mask uint16
}

func (c *Conn) QueryPointer(w uint32) (*PointerReply, error) {
	r := newReq(38, 0)
	r.u32(c.bo, w)
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return nil, err
	}
	return &PointerReply{
		SameScreen: rep[1] != 0,
		Root:       c.bo.Uint32(rep[8:12]),
		Child:      c.bo.Uint32(rep[12:16]),
		RootX:      int16(c.bo.Uint16(rep[16:18])),
		RootY:      int16(c.bo.Uint16(rep[18:20])),
		WinX:       int16(c.bo.Uint16(rep[20:22])),
		WinY:       int16(c.bo.Uint16(rep[22:24])),
		Mask:       c.bo.Uint16(rep[24:26]),
	}, nil
}

// ----------------------------------------------------------------------------
// ConvertSelection (24)
// ----------------------------------------------------------------------------

func (c *Conn) ConvertSelection(requestor, selection, target, property, time uint32) {
	r := newReq(24, 0)
	r.u32(c.bo, requestor)
	r.u32(c.bo, selection)
	r.u32(c.bo, target)
	r.u32(c.bo, property)
	r.u32(c.bo, time)
	c.Send(r.finish(c.bo))
}

// ----------------------------------------------------------------------------
// AllocColor (84) — used to alloc a pixel from an RGB triple on the default colormap.
// ----------------------------------------------------------------------------

type ColorReply struct {
	R, G, B uint16
	Pixel   uint32
}

func (c *Conn) AllocColor(cmap uint32, r, g, b uint16) (*ColorReply, error) {
	rq := newReq(84, 0)
	rq.u32(c.bo, cmap)
	rq.u16(c.bo, r)
	rq.u16(c.bo, g)
	rq.u16(c.bo, b)
	rq.u16(c.bo, 0)
	rep, err := c.SendReply(rq.finish(c.bo))
	if err != nil {
		return nil, err
	}
	return &ColorReply{
		R:     c.bo.Uint16(rep[8:10]),
		G:     c.bo.Uint16(rep[10:12]),
		B:     c.bo.Uint16(rep[12:14]),
		Pixel: c.bo.Uint32(rep[16:20]),
	}, nil
}

// ----------------------------------------------------------------------------
// TranslateCoordinates (opcode 40)
// ----------------------------------------------------------------------------

type TranslateReply struct {
	SameScreen bool
	Child      uint32
	DstX, DstY int16
}

func (c *Conn) TranslateCoordinates(src, dst uint32, srcX, srcY int16) (*TranslateReply, error) {
	r := newReq(40, 0)
	r.u32(c.bo, src)
	r.u32(c.bo, dst)
	r.u16(c.bo, uint16(srcX))
	r.u16(c.bo, uint16(srcY))
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return nil, err
	}
	return &TranslateReply{
		SameScreen: rep[1] != 0,
		Child:      c.bo.Uint32(rep[8:12]),
		DstX:       int16(c.bo.Uint16(rep[12:14])),
		DstY:       int16(c.bo.Uint16(rep[14:16])),
	}, nil
}

// ----------------------------------------------------------------------------
// GrabPointer (26), UngrabPointer (27)
// ----------------------------------------------------------------------------

type GrabPointerReq struct {
	OwnerEvents       bool
	GrabWindow        uint32
	EventMask         uint16
	PointerMode       uint8 // GrabModeSync / GrabModeAsync
	KeyboardMode      uint8
	ConfineTo, Cursor uint32 // 0 for None
	Time              uint32 // CurrentTime = 0
}

func (c *Conn) GrabPointer(req GrabPointerReq) (uint8, error) {
	r := newReq(26, btoi(req.OwnerEvents))
	r.u32(c.bo, req.GrabWindow)
	r.u16(c.bo, req.EventMask)
	r.u8(req.PointerMode)
	r.u8(req.KeyboardMode)
	r.u32(c.bo, req.ConfineTo)
	r.u32(c.bo, req.Cursor)
	r.u32(c.bo, req.Time)
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return 0, err
	}
	return rep[1], nil
}

func (c *Conn) UngrabPointer(time uint32) {
	r := newReq(27, 0)
	r.u32(c.bo, time)
	c.Send(r.finish(c.bo))
}

// ----------------------------------------------------------------------------
// OpenFont (45), CloseFont (46), CreateGlyphCursor (94), FreeCursor (95)
// ----------------------------------------------------------------------------

// OpenFont loads a server font by name (e.g. "cursor" for the standard glyph
// cursor font). The resulting fid can be passed to CreateGlyphCursor.
func (c *Conn) OpenFont(fid uint32, name string) {
	r := newReq(45, 0)
	r.u32(c.bo, fid)
	r.u16(c.bo, uint16(len(name)))
	r.u16(c.bo, 0)
	r.bytes([]byte(name))
	r.pad(len(name))
	c.Send(r.finish(c.bo))
}

func (c *Conn) CloseFont(fid uint32) {
	r := newReq(46, 0)
	r.u32(c.bo, fid)
	c.Send(r.finish(c.bo))
}

// CreateGlyphCursor builds a cursor from a glyph in a font (commonly the
// "cursor" font; standard cursor glyphs are at even-numbered character codes
// in the X cursor-font convention — see <X11/cursorfont.h>, e.g. XC_crosshair=30).
func (c *Conn) CreateGlyphCursor(cid, sourceFont, maskFont uint32, sourceChar, maskChar uint16, fr, fg, fb, br, bg, bb uint16) {
	r := newReq(94, 0)
	r.u32(c.bo, cid)
	r.u32(c.bo, sourceFont)
	r.u32(c.bo, maskFont)
	r.u16(c.bo, sourceChar)
	r.u16(c.bo, maskChar)
	r.u16(c.bo, fr)
	r.u16(c.bo, fg)
	r.u16(c.bo, fb)
	r.u16(c.bo, br)
	r.u16(c.bo, bg)
	r.u16(c.bo, bb)
	c.Send(r.finish(c.bo))
}

func (c *Conn) FreeCursor(cid uint32) {
	r := newReq(95, 0)
	r.u32(c.bo, cid)
	c.Send(r.finish(c.bo))
}

// ----------------------------------------------------------------------------
// GetImage (opcode 73)
// ----------------------------------------------------------------------------

type GetImageReply struct {
	Depth  uint8
	Visual uint32
	Data   []byte
}

func (c *Conn) GetImage(format uint8, drawable uint32, x, y int16, width, height uint16, planeMask uint32) (*GetImageReply, error) {
	r := newReq(73, format)
	r.u32(c.bo, drawable)
	r.u16(c.bo, uint16(x))
	r.u16(c.bo, uint16(y))
	r.u16(c.bo, width)
	r.u16(c.bo, height)
	r.u32(c.bo, planeMask)
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return nil, err
	}
	extraWords := c.bo.Uint32(rep[4:8])
	dataLen := int(extraWords) * 4
	data := make([]byte, dataLen)
	if dataLen > 0 {
		copy(data, rep[32:32+dataLen])
	}
	return &GetImageReply{
		Depth:  rep[1],
		Visual: c.bo.Uint32(rep[8:12]),
		Data:   data,
	}, nil
}
