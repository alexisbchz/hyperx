package xproto

// KeyboardMapping is the result of GetKeyboardMapping.
type KeyboardMapping struct {
	KeysymsPerKeycode uint8
	MinKeycode        uint8
	// Keysyms[(keycode - MinKeycode) * KeysymsPerKeycode + col]
	Keysyms []uint32
}

// GetKeyboardMapping (opcode 101) returns the keycode → keysym mapping.
func (c *Conn) GetKeyboardMapping() (*KeyboardMapping, error) {
	first := c.Setup.MinKeycode
	count := c.Setup.MaxKeycode - first + 1
	r := newReq(101, 0)
	r.u8(first)
	r.u8(count)
	r.u16(c.bo, 0)
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return nil, err
	}
	kpkc := rep[1]
	n := int(count) * int(kpkc)
	ks := make([]uint32, n)
	for i := 0; i < n; i++ {
		ks[i] = c.bo.Uint32(rep[32+i*4 : 36+i*4])
	}
	km := &KeyboardMapping{
		KeysymsPerKeycode: kpkc,
		MinKeycode:        first,
		Keysyms:           ks,
	}
	c.keyboardMap = km
	return km, nil
}

// Lookup translates a (keycode, state) pair into a keysym.
// Implements a simplified version of XLookupKeysym: tries shift column if
// Shift is held, else returns the unshifted keysym. Falls back to column 0.
func (km *KeyboardMapping) Lookup(keycode uint8, state uint16) uint32 {
	if km == nil || keycode < km.MinKeycode {
		return 0
	}
	row := int(keycode-km.MinKeycode) * int(km.KeysymsPerKeycode)
	if row+int(km.KeysymsPerKeycode) > len(km.Keysyms) {
		return 0
	}
	cols := int(km.KeysymsPerKeycode)
	get := func(i int) uint32 {
		if i < cols {
			return km.Keysyms[row+i]
		}
		return 0
	}
	const (
		ShiftMask = 1 << 0
		LockMask  = 1 << 1
	)
	shift := state&ShiftMask != 0
	caps := state&LockMask != 0

	k0 := get(0)
	k1 := get(1)

	// Effective shift: shift XOR caps for alpha keysyms.
	useShift := shift
	if caps {
		// Only flip for ASCII letters
		if (k0 >= 'a' && k0 <= 'z') || (k0 >= 'A' && k0 <= 'Z') {
			useShift = !shift
		}
	}
	if useShift && k1 != 0 {
		return k1
	}
	if k0 != 0 {
		return k0
	}
	return k1
}

// ModifierMapping is the result of GetModifierMapping.
// Modifiers are in order: Shift, Lock, Control, Mod1..Mod5.
type ModifierMapping struct {
	KeycodesPerModifier uint8
	Keycodes            [8][]uint8
}

// GetModifierMapping (opcode 119).
func (c *Conn) GetModifierMapping() (*ModifierMapping, error) {
	r := newReq(119, 0)
	rep, err := c.SendReply(r.finish(c.bo))
	if err != nil {
		return nil, err
	}
	kpm := rep[1]
	mm := &ModifierMapping{KeycodesPerModifier: kpm}
	for i := 0; i < 8; i++ {
		row := make([]uint8, kpm)
		copy(row, rep[32+i*int(kpm):32+(i+1)*int(kpm)])
		mm.Keycodes[i] = row
	}
	c.modMap = mm
	return mm, nil
}
