package xproto

import "encoding/binary"

// X11 event type codes.
const (
	EvKeyPress         = 2
	EvKeyRelease       = 3
	EvButtonPress      = 4
	EvButtonRelease    = 5
	EvMotionNotify     = 6
	EvEnterNotify      = 7
	EvLeaveNotify      = 8
	EvFocusIn          = 9
	EvFocusOut         = 10
	EvKeymapNotify     = 11
	EvExpose           = 12
	EvGraphicsExposure = 13
	EvNoExposure       = 14
	EvVisibilityNotify = 15
	EvCreateNotify     = 16
	EvDestroyNotify    = 17
	EvUnmapNotify      = 18
	EvMapNotify        = 19
	EvMapRequest       = 20
	EvReparentNotify   = 21
	EvConfigureNotify  = 22
	EvSelectionRequest = 30
	EvSelectionNotify  = 31
	EvSelectionClear   = 29
	EvPropertyNotify   = 28
	EvClientMessage    = 33
	EvMappingNotify    = 34
)

// Event is the interface implemented by all X events.
type Event interface{ x11event() }

type KeyPressEvent struct {
	Sequence   uint16
	Detail     uint8 // keycode
	Time       uint32
	Root       uint32
	Event      uint32
	Child      uint32
	RootX      int16
	RootY      int16
	EventX     int16
	EventY     int16
	State      uint16
	SameScreen bool
	SendEvent  bool
}

func (KeyPressEvent) x11event() {}

// ButtonPressEvent, ButtonReleaseEvent, and MotionNotifyEvent share the same
// 32-byte wire layout as KeyPressEvent — Detail carries the button code (1
// = left, 2 = middle, 3 = right, 4/5 = scroll) for buttons, or a hint flag
// for motion (only meaningful with PointerMotionHint mask).
type ButtonPressEvent struct {
	Sequence   uint16
	Detail     uint8
	Time       uint32
	Root       uint32
	Event      uint32
	Child      uint32
	RootX      int16
	RootY      int16
	EventX     int16
	EventY     int16
	State      uint16
	SameScreen bool
	SendEvent  bool
}

func (ButtonPressEvent) x11event() {}

type ButtonReleaseEvent struct {
	Sequence   uint16
	Detail     uint8
	Time       uint32
	Root       uint32
	Event      uint32
	Child      uint32
	RootX      int16
	RootY      int16
	EventX     int16
	EventY     int16
	State      uint16
	SameScreen bool
	SendEvent  bool
}

func (ButtonReleaseEvent) x11event() {}

type MotionNotifyEvent struct {
	Sequence   uint16
	Detail     uint8
	Time       uint32
	Root       uint32
	Event      uint32
	Child      uint32
	RootX      int16
	RootY      int16
	EventX     int16
	EventY     int16
	State      uint16
	SameScreen bool
	SendEvent  bool
}

func (MotionNotifyEvent) x11event() {}

type ExposeEvent struct {
	Sequence uint16
	Window   uint32
	X, Y     uint16
	Width    uint16
	Height   uint16
	Count    uint16
}

func (ExposeEvent) x11event() {}

type FocusInEvent struct {
	Sequence uint16
	Detail   uint8
	Window   uint32
	Mode     uint8
}

func (FocusInEvent) x11event() {}

type FocusOutEvent struct {
	Sequence uint16
	Detail   uint8
	Window   uint32
	Mode     uint8
}

func (FocusOutEvent) x11event() {}

type VisibilityNotifyEvent struct {
	Sequence uint16
	Window   uint32
	State    uint8
}

func (VisibilityNotifyEvent) x11event() {}

type DestroyNotifyEvent struct {
	Sequence uint16
	Event    uint32
	Window   uint32
}

func (DestroyNotifyEvent) x11event() {}

type SelectionNotifyEvent struct {
	Sequence  uint16
	Time      uint32
	Requestor uint32
	Selection uint32
	Target    uint32
	Property  uint32 // 0 = None means convert failed
}

func (SelectionNotifyEvent) x11event() {}

type MappingNotifyEvent struct {
	Sequence    uint16
	Request     uint8
	FirstKeycode uint8
	Count       uint8
}

func (MappingNotifyEvent) x11event() {}

type ConfigureNotifyEvent struct {
	Sequence         uint16
	Event, Window    uint32
	AboveSibling     uint32
	X, Y             int16
	Width, Height    uint16
	BorderWidth      uint16
	OverrideRedirect bool
}

func (ConfigureNotifyEvent) x11event() {}

type UnknownEvent struct {
	Type uint8
	Raw  [32]byte
}

func (UnknownEvent) x11event() {}

// decodeEvent parses the 32-byte event header (sub-types may extend, but
// dmenu-relevant events all fit in 32 bytes).
func decodeEvent(b []byte, bo binary.ByteOrder) Event {
	typ := b[0] & 0x7f
	sendEvent := b[0]&0x80 != 0
	switch typ {
	case EvKeyPress:
		ev := KeyPressEvent{
			Sequence:   bo.Uint16(b[2:4]),
			Detail:     b[1],
			Time:       bo.Uint32(b[4:8]),
			Root:       bo.Uint32(b[8:12]),
			Event:      bo.Uint32(b[12:16]),
			Child:      bo.Uint32(b[16:20]),
			RootX:      int16(bo.Uint16(b[20:22])),
			RootY:      int16(bo.Uint16(b[22:24])),
			EventX:     int16(bo.Uint16(b[24:26])),
			EventY:     int16(bo.Uint16(b[26:28])),
			State:      bo.Uint16(b[28:30]),
			SameScreen: b[30] != 0,
			SendEvent:  sendEvent,
		}
		return ev
	case EvKeyRelease:
		// dmenu doesn't act on releases; drop them so they don't double-fire.
		return nil
	case EvButtonPress:
		return ButtonPressEvent{
			Sequence:   bo.Uint16(b[2:4]),
			Detail:     b[1],
			Time:       bo.Uint32(b[4:8]),
			Root:       bo.Uint32(b[8:12]),
			Event:      bo.Uint32(b[12:16]),
			Child:      bo.Uint32(b[16:20]),
			RootX:      int16(bo.Uint16(b[20:22])),
			RootY:      int16(bo.Uint16(b[22:24])),
			EventX:     int16(bo.Uint16(b[24:26])),
			EventY:     int16(bo.Uint16(b[26:28])),
			State:      bo.Uint16(b[28:30]),
			SameScreen: b[30] != 0,
			SendEvent:  sendEvent,
		}
	case EvButtonRelease:
		return ButtonReleaseEvent{
			Sequence:   bo.Uint16(b[2:4]),
			Detail:     b[1],
			Time:       bo.Uint32(b[4:8]),
			Root:       bo.Uint32(b[8:12]),
			Event:      bo.Uint32(b[12:16]),
			Child:      bo.Uint32(b[16:20]),
			RootX:      int16(bo.Uint16(b[20:22])),
			RootY:      int16(bo.Uint16(b[22:24])),
			EventX:     int16(bo.Uint16(b[24:26])),
			EventY:     int16(bo.Uint16(b[26:28])),
			State:      bo.Uint16(b[28:30]),
			SameScreen: b[30] != 0,
			SendEvent:  sendEvent,
		}
	case EvMotionNotify:
		return MotionNotifyEvent{
			Sequence:   bo.Uint16(b[2:4]),
			Detail:     b[1],
			Time:       bo.Uint32(b[4:8]),
			Root:       bo.Uint32(b[8:12]),
			Event:      bo.Uint32(b[12:16]),
			Child:      bo.Uint32(b[16:20]),
			RootX:      int16(bo.Uint16(b[20:22])),
			RootY:      int16(bo.Uint16(b[22:24])),
			EventX:     int16(bo.Uint16(b[24:26])),
			EventY:     int16(bo.Uint16(b[26:28])),
			State:      bo.Uint16(b[28:30]),
			SameScreen: b[30] != 0,
			SendEvent:  sendEvent,
		}
	case EvExpose:
		return ExposeEvent{
			Sequence: bo.Uint16(b[2:4]),
			Window:   bo.Uint32(b[4:8]),
			X:        bo.Uint16(b[8:10]),
			Y:        bo.Uint16(b[10:12]),
			Width:    bo.Uint16(b[12:14]),
			Height:   bo.Uint16(b[14:16]),
			Count:    bo.Uint16(b[16:18]),
		}
	case EvFocusIn:
		return FocusInEvent{
			Sequence: bo.Uint16(b[2:4]),
			Detail:   b[1],
			Window:   bo.Uint32(b[4:8]),
			Mode:     b[8],
		}
	case EvFocusOut:
		return FocusOutEvent{
			Sequence: bo.Uint16(b[2:4]),
			Detail:   b[1],
			Window:   bo.Uint32(b[4:8]),
			Mode:     b[8],
		}
	case EvVisibilityNotify:
		return VisibilityNotifyEvent{
			Sequence: bo.Uint16(b[2:4]),
			Window:   bo.Uint32(b[4:8]),
			State:    b[8],
		}
	case EvDestroyNotify:
		return DestroyNotifyEvent{
			Sequence: bo.Uint16(b[2:4]),
			Event:    bo.Uint32(b[4:8]),
			Window:   bo.Uint32(b[8:12]),
		}
	case EvSelectionNotify:
		return SelectionNotifyEvent{
			Sequence:  bo.Uint16(b[2:4]),
			Time:      bo.Uint32(b[4:8]),
			Requestor: bo.Uint32(b[8:12]),
			Selection: bo.Uint32(b[12:16]),
			Target:    bo.Uint32(b[16:20]),
			Property:  bo.Uint32(b[20:24]),
		}
	case EvMappingNotify:
		return MappingNotifyEvent{
			Sequence:     bo.Uint16(b[2:4]),
			Request:      b[4],
			FirstKeycode: b[5],
			Count:        b[6],
		}
	case EvConfigureNotify:
		return ConfigureNotifyEvent{
			Sequence:         bo.Uint16(b[2:4]),
			Event:            bo.Uint32(b[4:8]),
			Window:           bo.Uint32(b[8:12]),
			AboveSibling:     bo.Uint32(b[12:16]),
			X:                int16(bo.Uint16(b[16:18])),
			Y:                int16(bo.Uint16(b[18:20])),
			Width:            bo.Uint16(b[20:22]),
			Height:           bo.Uint16(b[22:24]),
			BorderWidth:      bo.Uint16(b[24:26]),
			OverrideRedirect: b[26] != 0,
		}
	default:
		u := UnknownEvent{Type: typ}
		copy(u.Raw[:], b)
		return u
	}
}
