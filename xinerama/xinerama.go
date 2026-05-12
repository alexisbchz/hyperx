// Package xinerama implements the minimal subset of the XINERAMA extension
// needed to enumerate per-monitor geometry.
package xinerama

import (
	"encoding/binary"
	"fmt"

	"github.com/alexisbchz/hyperx/xproto"
)

// Screen describes one Xinerama monitor.
type Screen struct {
	XOrg, YOrg     int16
	Width, Height  uint16
}

// Query asks the server for Xinerama screens. Returns nil, nil when the
// extension isn't available or is inactive — callers should fall back to
// using the root window geometry.
func Query(c *xproto.Conn) ([]Screen, error) {
	info, err := c.QueryExtension("XINERAMA")
	if err != nil {
		return nil, err
	}
	if !info.Present {
		return nil, nil
	}
	op := info.MajorOpcode

	// Xinerama:IsActive (minor opcode 4), request length 1.
	bo := binary.LittleEndian
	req := []byte{op, 4, 1, 0}
	rep, err := c.SendReply(req)
	if err != nil {
		return nil, fmt.Errorf("xinerama IsActive: %w", err)
	}
	active := bo.Uint32(rep[8:12])
	if active == 0 {
		return nil, nil
	}

	// Xinerama:QueryScreens (minor opcode 5), request length 1.
	req2 := []byte{op, 5, 1, 0}
	rep2, err := c.SendReply(req2)
	if err != nil {
		return nil, fmt.Errorf("xinerama QueryScreens: %w", err)
	}
	n := bo.Uint32(rep2[8:12])
	if n == 0 {
		return nil, nil
	}
	out := make([]Screen, n)
	for i := uint32(0); i < n; i++ {
		off := 32 + int(i)*8
		out[i] = Screen{
			XOrg:   int16(bo.Uint16(rep2[off : off+2])),
			YOrg:   int16(bo.Uint16(rep2[off+2 : off+4])),
			Width:  bo.Uint16(rep2[off+4 : off+6]),
			Height: bo.Uint16(rep2[off+6 : off+8]),
		}
	}
	return out, nil
}
