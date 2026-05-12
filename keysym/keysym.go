// Package keysym defines the subset of X11 keysym constants and conversions
// needed by hypermenu. Keysyms below 0x80 map to ASCII; Latin-1 occupies
// 0x80..0xff; the rest of dmenu's surface is small (arrows, return, tab,
// home/end, etc.).
package keysym

// Latin-1 / ASCII helpers.
const (
	KSpace     = 0x0020
	KExclam    = 0x0021
	KQuestion  = 0x003F
	KSlash     = 0x002F
	KBackslash = 0x005C
	KBracketLeft  = 0x005B
	KBracketRight = 0x005D
	KGrave     = 0x0060
)

// Function/control keys (subset).
const (
	KBackSpace = 0xFF08
	KTab       = 0xFF09
	KReturn    = 0xFF0D
	KEscape    = 0xFF1B
	KDelete    = 0xFFFF

	KHome  = 0xFF50
	KLeft  = 0xFF51
	KUp    = 0xFF52
	KRight = 0xFF53
	KDown  = 0xFF54
	KPrior = 0xFF55 // PageUp
	KNext  = 0xFF56 // PageDown
	KEnd   = 0xFF57

	KKPSpace  = 0xFF80
	KKPTab    = 0xFF89
	KKPEnter  = 0xFF8D
	KKPHome   = 0xFF95
	KKPLeft   = 0xFF96
	KKPUp     = 0xFF97
	KKPRight  = 0xFF98
	KKPDown   = 0xFF99
	KKPPrior  = 0xFF9A
	KKPNext   = 0xFF9B
	KKPEnd    = 0xFF9C
	KKPDelete = 0xFF9F

	KShiftL   = 0xFFE1
	KShiftR   = 0xFFE2
	KControlL = 0xFFE3
	KControlR = 0xFFE4
	KCapsLock = 0xFFE5
	KAltL     = 0xFFE9
	KAltR     = 0xFFEA
	KMetaL    = 0xFFE7
	KMetaR    = 0xFFE8
	KSuperL   = 0xFFEB
	KSuperR   = 0xFFEC
)

// Modifier mask bits (matches X core).
const (
	ShiftMask   = 1 << 0
	LockMask    = 1 << 1
	ControlMask = 1 << 2
	Mod1Mask    = 1 << 3 // typically Alt
	Mod2Mask    = 1 << 4
	Mod3Mask    = 1 << 5
	Mod4Mask    = 1 << 6 // typically Super
	Mod5Mask    = 1 << 7
)

// ToRune converts a keysym to a Unicode code point if possible.
// Returns 0 if the keysym is a non-printable control key.
// Implements the standard X11 keysym → UCS rule:
//   - 0x20..0x7e and 0xa0..0xff are direct Latin-1.
//   - 0x01000000 | u maps to Unicode u.
//   - other special function keysyms map to 0.
func ToRune(ks uint32) rune {
	if ks >= 0x20 && ks <= 0x7e {
		return rune(ks)
	}
	if ks >= 0xa0 && ks <= 0xff {
		return rune(ks)
	}
	if ks&0xff000000 == 0x01000000 {
		return rune(ks & 0x00ffffff)
	}
	return 0
}

// IsKeypadEquivalent returns the non-keypad keysym for keypad navigation keys
// so callers can fold the two flavours together.
func IsKeypadEquivalent(ks uint32) (uint32, bool) {
	switch ks {
	case KKPSpace:
		return KSpace, true
	case KKPTab:
		return KTab, true
	case KKPEnter:
		return KReturn, true
	case KKPHome:
		return KHome, true
	case KKPLeft:
		return KLeft, true
	case KKPUp:
		return KUp, true
	case KKPRight:
		return KRight, true
	case KKPDown:
		return KDown, true
	case KKPPrior:
		return KPrior, true
	case KKPNext:
		return KNext, true
	case KKPEnd:
		return KEnd, true
	case KKPDelete:
		return KDelete, true
	}
	return ks, false
}
