// Package xproto is a minimal, pure-Go X11 protocol client.
//
// Only the subset of the X11 core protocol that dmenu uses is implemented.
// No cgo, no third-party deps.
package xproto

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Conn is an X11 server connection.
type Conn struct {
	c    net.Conn
	br   *bufio.Reader
	bo   binary.ByteOrder // always native; we negotiate little-endian
	mu   sync.Mutex       // serializes writes / sequence updates
	seq  uint16           // last sequence sent (wraps every 65536)
	pend map[uint16]chan replyOrErr
	pmu  sync.Mutex

	// Setup info
	Setup    *Setup
	Screen   *Screen // default screen
	ScreenIx int

	// Event delivery (events have no sequence-based reply, they stream in)
	events chan Event
	errs   chan error
	done   chan struct{}

	// Resource ID allocation
	idBase, idMask uint32
	idNext         uint32

	// Cached request data
	keyboardMap *KeyboardMapping
	modMap      *ModifierMapping
}

type replyOrErr struct {
	reply []byte // raw 32+ bytes; nil if error
	err   *Error
}

// Error is an X protocol error from the server.
type Error struct {
	Code     uint8
	Sequence uint16
	BadValue uint32
	MinorOp  uint16
	MajorOp  uint8
}

func (e *Error) Error() string {
	return fmt.Sprintf("X11 error: code=%d seq=%d bad=0x%x major=%d minor=%d",
		e.Code, e.Sequence, e.BadValue, e.MajorOp, e.MinorOp)
}

// Dial connects to the X server given by $DISPLAY (or display arg).
// Format: [hostname]:display[.screen]
func Dial(display string) (*Conn, error) {
	if display == "" {
		display = os.Getenv("DISPLAY")
	}
	if display == "" {
		return nil, errors.New("DISPLAY not set")
	}

	host, dispNum, screenNum, err := parseDisplay(display)
	if err != nil {
		return nil, err
	}

	var nc net.Conn
	if host == "" || host == "unix" {
		// Try abstract first (Linux), then filesystem socket.
		sock := fmt.Sprintf("/tmp/.X11-unix/X%d", dispNum)
		nc, err = net.Dial("unix", sock)
		if err != nil {
			abs := "@/tmp/.X11-unix/X" + strconv.Itoa(dispNum)
			nc, err = net.Dial("unix", abs)
		}
	} else {
		nc, err = net.Dial("tcp", fmt.Sprintf("%s:%d", host, 6000+dispNum))
	}
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	c := &Conn{
		c:        nc,
		br:       bufio.NewReaderSize(nc, 32768),
		bo:       binary.LittleEndian,
		pend:     make(map[uint16]chan replyOrErr),
		events:   make(chan Event, 64),
		errs:     make(chan error, 8),
		done:     make(chan struct{}),
		ScreenIx: screenNum,
	}

	if err := c.handshake(host, dispNum); err != nil {
		nc.Close()
		return nil, err
	}

	go c.reader()
	return c, nil
}

func parseDisplay(d string) (host string, disp int, screen int, err error) {
	// e.g. ":0", ":0.0", "hostname:0", "unix:0"
	i := strings.LastIndex(d, ":")
	if i < 0 {
		return "", 0, 0, fmt.Errorf("bad DISPLAY %q", d)
	}
	host = d[:i]
	rest := d[i+1:]
	if dot := strings.Index(rest, "."); dot >= 0 {
		disp, err = strconv.Atoi(rest[:dot])
		if err != nil {
			return
		}
		screen, err = strconv.Atoi(rest[dot+1:])
		return
	}
	disp, err = strconv.Atoi(rest)
	return
}

// Close terminates the connection.
func (c *Conn) Close() error {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	return c.c.Close()
}

// ----------------------------------------------------------------------------
// Handshake / setup
// ----------------------------------------------------------------------------

// Setup describes the X server connection setup reply.
type Setup struct {
	ProtocolMajor, ProtocolMinor uint16
	ReleaseNumber                uint32
	ResourceIDBase, ResourceIDMask uint32
	MotionBufferSize             uint32
	MaximumRequestLength         uint16
	ImageByteOrder               uint8
	BitmapFormatBitOrder         uint8
	BitmapFormatScanlineUnit     uint8
	BitmapFormatScanlinePad      uint8
	MinKeycode, MaxKeycode       uint8
	Vendor                       string
	PixmapFormats                []PixmapFormat
	Screens                      []Screen
}

type PixmapFormat struct {
	Depth, BitsPerPixel, ScanlinePad uint8
}

type Screen struct {
	Root, DefaultColormap                  uint32
	WhitePixel, BlackPixel                 uint32
	CurrentInputMasks                      uint32
	WidthInPixels, HeightInPixels          uint16
	WidthInMillimeters, HeightInMillimeters uint16
	MinInstalledMaps, MaxInstalledMaps     uint16
	RootVisual                             uint32
	BackingStores                          uint8
	SaveUnders                             uint8
	RootDepth                              uint8
	Depths                                 []Depth
}

type Depth struct {
	Depth   uint8
	Visuals []Visual
}

type Visual struct {
	ID                                uint32
	Class                             uint8
	BitsPerRGBValue                   uint8
	ColormapEntries                   uint16
	RedMask, GreenMask, BlueMask      uint32
}

func (c *Conn) handshake(host string, disp int) error {
	authName, authData := readAuthCookie(host, disp)

	var buf bytes.Buffer
	// Setup request, little-endian
	buf.WriteByte('l')
	buf.WriteByte(0)                                          // unused
	binary.Write(&buf, c.bo, uint16(11))                      // protocol-major
	binary.Write(&buf, c.bo, uint16(0))                       // protocol-minor
	binary.Write(&buf, c.bo, uint16(len(authName)))           // auth name len
	binary.Write(&buf, c.bo, uint16(len(authData)))           // auth data len
	binary.Write(&buf, c.bo, uint16(0))                       // unused
	buf.WriteString(authName)
	writePad(&buf, len(authName))
	buf.Write(authData)
	writePad(&buf, len(authData))

	if _, err := c.c.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("setup write: %w", err)
	}

	header := make([]byte, 8)
	if _, err := io.ReadFull(c.br, header); err != nil {
		return fmt.Errorf("setup read: %w", err)
	}
	status := header[0]
	additional := c.bo.Uint16(header[6:8])
	body := make([]byte, int(additional)*4)
	if _, err := io.ReadFull(c.br, body); err != nil {
		return fmt.Errorf("setup body read: %w", err)
	}

	if status == 0 {
		reasonLen := header[1]
		reason := body[:int(reasonLen)]
		return fmt.Errorf("X server refused connection: %s", string(reason))
	}
	if status == 2 {
		return errors.New("X server requires further authentication")
	}
	// status == 1: success
	c.Setup = parseSetup(header, body, c.bo)
	c.idBase = c.Setup.ResourceIDBase
	c.idMask = c.Setup.ResourceIDMask
	c.idNext = 0
	if c.ScreenIx >= len(c.Setup.Screens) {
		c.ScreenIx = 0
	}
	c.Screen = &c.Setup.Screens[c.ScreenIx]
	return nil
}

func parseSetup(hdr, body []byte, bo binary.ByteOrder) *Setup {
	// Layout of body (X11 setup reply, bytes after the 8-byte header):
	//   0..3    release-number
	//   4..7    resource-id-base
	//   8..11   resource-id-mask
	//   12..15  motion-buffer-size
	//   16..17  vendor-len
	//   18..19  maximum-request-length
	//   20      number-of-roots (screens)
	//   21      number-of-pixmap-formats
	//   22      image-byte-order
	//   23      bitmap-format-bit-order
	//   24      bitmap-format-scanline-unit
	//   25      bitmap-format-scanline-pad
	//   26      min-keycode
	//   27      max-keycode
	//   28..31  unused
	//   32..    vendor (vendor-len bytes, padded to 4), then formats, then screens.
	s := &Setup{
		ProtocolMajor:            bo.Uint16(hdr[2:4]),
		ProtocolMinor:            bo.Uint16(hdr[4:6]),
		ReleaseNumber:            bo.Uint32(body[0:4]),
		ResourceIDBase:           bo.Uint32(body[4:8]),
		ResourceIDMask:           bo.Uint32(body[8:12]),
		MotionBufferSize:         bo.Uint32(body[12:16]),
		MaximumRequestLength:     bo.Uint16(body[18:20]),
		ImageByteOrder:           body[22],
		BitmapFormatBitOrder:     body[23],
		BitmapFormatScanlineUnit: body[24],
		BitmapFormatScanlinePad:  body[25],
		MinKeycode:               body[26],
		MaxKeycode:               body[27],
	}
	vendorLen := bo.Uint16(body[16:18])
	nScreens := body[20]
	nFormats := body[21]
	off := 32
	s.Vendor = string(body[off : off+int(vendorLen)])
	off += int(vendorLen)
	off = align4(off)
	for i := uint8(0); i < nFormats; i++ {
		s.PixmapFormats = append(s.PixmapFormats, PixmapFormat{
			Depth:        body[off],
			BitsPerPixel: body[off+1],
			ScanlinePad:  body[off+2],
		})
		off += 8
	}
	for i := uint8(0); i < nScreens; i++ {
		sc := Screen{
			Root:                bo.Uint32(body[off : off+4]),
			DefaultColormap:     bo.Uint32(body[off+4 : off+8]),
			WhitePixel:          bo.Uint32(body[off+8 : off+12]),
			BlackPixel:          bo.Uint32(body[off+12 : off+16]),
			CurrentInputMasks:   bo.Uint32(body[off+16 : off+20]),
			WidthInPixels:       bo.Uint16(body[off+20 : off+22]),
			HeightInPixels:      bo.Uint16(body[off+22 : off+24]),
			WidthInMillimeters:  bo.Uint16(body[off+24 : off+26]),
			HeightInMillimeters: bo.Uint16(body[off+26 : off+28]),
			MinInstalledMaps:    bo.Uint16(body[off+28 : off+30]),
			MaxInstalledMaps:    bo.Uint16(body[off+30 : off+32]),
			RootVisual:          bo.Uint32(body[off+32 : off+36]),
			BackingStores:       body[off+36],
			SaveUnders:          body[off+37],
			RootDepth:           body[off+38],
		}
		nDepths := body[off+39]
		off += 40
		for j := uint8(0); j < nDepths; j++ {
			d := Depth{Depth: body[off]}
			nVisuals := bo.Uint16(body[off+2 : off+4])
			off += 8
			for k := uint16(0); k < nVisuals; k++ {
				d.Visuals = append(d.Visuals, Visual{
					ID:              bo.Uint32(body[off : off+4]),
					Class:           body[off+4],
					BitsPerRGBValue: body[off+5],
					ColormapEntries: bo.Uint16(body[off+6 : off+8]),
					RedMask:         bo.Uint32(body[off+8 : off+12]),
					GreenMask:       bo.Uint32(body[off+12 : off+16]),
					BlueMask:        bo.Uint32(body[off+16 : off+20]),
				})
				off += 24
			}
			sc.Depths = append(sc.Depths, d)
		}
		s.Screens = append(s.Screens, sc)
	}
	return s
}

// VisualByID looks up a visual by ID across all depths/screens.
func (c *Conn) VisualByID(id uint32) (*Visual, uint8) {
	for _, sc := range c.Setup.Screens {
		for _, d := range sc.Depths {
			for i := range d.Visuals {
				if d.Visuals[i].ID == id {
					return &d.Visuals[i], d.Depth
				}
			}
		}
	}
	return nil, 0
}

// ----------------------------------------------------------------------------
// Resource ID allocation
// ----------------------------------------------------------------------------

// NewID allocates a fresh X resource identifier.
func (c *Conn) NewID() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Find the lowest set bit in mask, then increment within mask.
	id := (c.idNext + 1) & c.idMask
	if id == 0 {
		// Wrap is unlikely for our short-lived menu; just panic if exhausted.
		panic("xproto: resource ID space exhausted")
	}
	c.idNext = id
	return c.idBase | id
}

// ----------------------------------------------------------------------------
// Reader goroutine: dispatches replies/errors/events.
// ----------------------------------------------------------------------------

func (c *Conn) reader() {
	defer close(c.events)
	for {
		head := make([]byte, 32)
		if _, err := io.ReadFull(c.br, head); err != nil {
			select {
			case c.errs <- err:
			default:
			}
			return
		}
		switch head[0] {
		case 0: // Error
			e := &Error{
				Code:     head[1],
				Sequence: c.bo.Uint16(head[2:4]),
				BadValue: c.bo.Uint32(head[4:8]),
				MinorOp:  c.bo.Uint16(head[8:10]),
				MajorOp:  head[10],
			}
			c.deliver(e.Sequence, nil, e)
		case 1: // Reply
			seq := c.bo.Uint16(head[2:4])
			extra := c.bo.Uint32(head[4:8]) // in 4-byte units, beyond the 32-byte head
			body := make([]byte, 32+int(extra)*4)
			copy(body, head)
			if extra > 0 {
				if _, err := io.ReadFull(c.br, body[32:]); err != nil {
					select {
					case c.errs <- err:
					default:
					}
					return
				}
			}
			c.deliver(seq, body, nil)
		default:
			// Event: head[0] = type | (sendEvent ? 0x80 : 0)
			ev := decodeEvent(head, c.bo)
			if ev != nil {
				select {
				case c.events <- ev:
				case <-c.done:
					return
				}
			}
		}
	}
}

func (c *Conn) deliver(seq uint16, reply []byte, err *Error) {
	c.pmu.Lock()
	ch, ok := c.pend[seq]
	if ok {
		delete(c.pend, seq)
	}
	c.pmu.Unlock()
	if ok {
		ch <- replyOrErr{reply: reply, err: err}
		return
	}
	// Unmatched error => surface it.
	if err != nil {
		select {
		case c.errs <- err:
		default:
		}
	}
}

// Events returns a receive-only channel of incoming X events.
func (c *Conn) Events() <-chan Event { return c.events }

// Errs returns a channel of asynchronous protocol errors.
func (c *Conn) Errs() <-chan error { return c.errs }

// ----------------------------------------------------------------------------
// Low-level send / send+reply
// ----------------------------------------------------------------------------

// Send writes a fully-encoded request (length field already set) and returns its sequence number.
func (c *Conn) Send(req []byte) uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	if _, err := c.c.Write(req); err != nil {
		select {
		case c.errs <- err:
		default:
		}
	}
	return c.seq
}

// SendReply sends a request that has a reply and blocks until the reply arrives.
func (c *Conn) SendReply(req []byte) ([]byte, error) {
	ch := make(chan replyOrErr, 1)
	c.mu.Lock()
	c.seq++
	seq := c.seq
	c.pmu.Lock()
	c.pend[seq] = ch
	c.pmu.Unlock()
	_, werr := c.c.Write(req)
	c.mu.Unlock()
	if werr != nil {
		return nil, werr
	}
	r := <-ch
	if r.err != nil {
		return nil, r.err
	}
	return r.reply, nil
}

// Sync forces a server round-trip (GetInputFocus) so any pending errors surface.
func (c *Conn) Sync() error {
	// GetInputFocus: opcode 43, 1 word.
	buf := make([]byte, 4)
	buf[0] = 43
	buf[1] = 0
	c.bo.PutUint16(buf[2:4], 1)
	_, err := c.SendReply(buf)
	return err
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func writePad(w io.Writer, n int) {
	if r := n & 3; r != 0 {
		w.Write(make([]byte, 4-r))
	}
}

func padLen(n int) int {
	if r := n & 3; r != 0 {
		return 4 - r
	}
	return 0
}

func align4(n int) int { return (n + 3) &^ 3 }

// ----------------------------------------------------------------------------
// Xauthority parsing (MIT-MAGIC-COOKIE-1 only)
// ----------------------------------------------------------------------------

func readAuthCookie(host string, disp int) (string, []byte) {
	path := os.Getenv("XAUTHORITY")
	if path == "" {
		u, err := user.Current()
		if err != nil {
			return "", nil
		}
		path = filepath.Join(u.HomeDir, ".Xauthority")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil
	}
	hostname, _ := os.Hostname()
	dispStr := strconv.Itoa(disp)
	off := 0
	for off < len(data) {
		if off+2 > len(data) {
			break
		}
		family := binary.BigEndian.Uint16(data[off : off+2])
		off += 2
		readStr := func() (string, bool) {
			if off+2 > len(data) {
				return "", false
			}
			l := int(binary.BigEndian.Uint16(data[off : off+2]))
			off += 2
			if off+l > len(data) {
				return "", false
			}
			s := string(data[off : off+l])
			off += l
			return s, true
		}
		addr, ok := readStr()
		if !ok {
			break
		}
		num, ok := readStr()
		if !ok {
			break
		}
		name, ok := readStr()
		if !ok {
			break
		}
		cookie, ok := readStr()
		if !ok {
			break
		}
		// Family 256=Local (unix). Match hostname.
		// Family 0=IPv4, 6=IPv6, 252=Wild
		matchHost := family == 256 && (host == "" || host == "unix") && addr == hostname
		matchHost = matchHost || (family == 252) // wildcard
		if matchHost && (num == "" || num == dispStr) {
			return name, []byte(cookie)
		}
	}
	return "", nil
}
