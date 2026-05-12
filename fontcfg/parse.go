package fontcfg

import (
	"encoding/binary"
	"io"
	"os"
	"unicode/utf16"
)

// readNameTable opens path and extracts the Family (name ID 1) and Subfamily
// (name ID 2) strings from the TTF/OTF 'name' table without parsing the rest
// of the font. Returns ok=false for collections (ttcf), unsupported magics,
// or malformed/truncated tables.
//
// This is the hot path for the font-directory scan: a full sfnt.Parse reads
// the entire file and builds offset tables for every OpenType table, which
// costs ~150µs per font and ~250MB of I/O across the system fonts. The
// numbers we actually use are 4 bytes of magic, the table directory (16
// bytes × ~20 tables), and a few KB of name data — typically well under
// 4KB per file.
func readNameTable(path string) (family, subfamily string, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", false
	}
	defer f.Close()

	var hdr [12]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return "", "", false
	}
	switch binary.BigEndian.Uint32(hdr[0:4]) {
	case 0x00010000, // TrueType
		0x4F54544F, // 'OTTO' (CFF / OpenType)
		0x74727565, // 'true'
		0x74797031: // 'typ1'
	default:
		// ttcf collections and unknown formats: skip (matches existing
		// behaviour — sfnt.Parse rejects them too).
		return "", "", false
	}
	numTables := int(binary.BigEndian.Uint16(hdr[4:6]))
	if numTables == 0 || numTables > 64 {
		return "", "", false
	}

	dir := make([]byte, numTables*16)
	if _, err := io.ReadFull(f, dir); err != nil {
		return "", "", false
	}
	var nameOff, nameLen uint32
	for i := range numTables {
		rec := dir[i*16:]
		if binary.BigEndian.Uint32(rec[0:4]) == 0x6E616D65 /* 'name' */ {
			nameOff = binary.BigEndian.Uint32(rec[8:12])
			nameLen = binary.BigEndian.Uint32(rec[12:16])
			break
		}
	}
	if nameOff == 0 || nameLen < 6 || nameLen > 1<<20 {
		return "", "", false
	}

	if _, err := f.Seek(int64(nameOff), io.SeekStart); err != nil {
		return "", "", false
	}
	nt := make([]byte, nameLen)
	if _, err := io.ReadFull(f, nt); err != nil {
		return "", "", false
	}

	count := int(binary.BigEndian.Uint16(nt[2:4]))
	strOff := int(binary.BigEndian.Uint16(nt[4:6]))
	if 6+count*12 > len(nt) || strOff > len(nt) {
		return "", "", false
	}

	var (
		famBest, subBest int = -1, -1
		famB, subB       []byte
		famP, subP       uint16
	)
	for i := range count {
		rec := nt[6+i*12 : 6+(i+1)*12]
		nameID := binary.BigEndian.Uint16(rec[6:8])
		if nameID != 1 && nameID != 2 {
			continue
		}
		platformID := binary.BigEndian.Uint16(rec[0:2])
		langID := binary.BigEndian.Uint16(rec[4:6])
		length := int(binary.BigEndian.Uint16(rec[8:10]))
		offset := int(binary.BigEndian.Uint16(rec[10:12]))
		end := strOff + offset + length
		if end > len(nt) || length == 0 {
			continue
		}
		// Priority: prefer Windows en-US, then any Windows, then Unicode,
		// then Mac English, then anything.
		var pri int
		switch {
		case platformID == 3 && langID == 0x0409:
			pri = 4
		case platformID == 3:
			pri = 3
		case platformID == 0:
			pri = 2
		case platformID == 1 && langID == 0:
			pri = 1
		default:
			pri = 0
		}
		b := nt[strOff+offset : end]
		if nameID == 1 && pri > famBest {
			famBest, famB, famP = pri, b, platformID
		}
		if nameID == 2 && pri > subBest {
			subBest, subB, subP = pri, b, platformID
		}
	}
	if famBest < 0 {
		return "", "", false
	}
	family = decodeNameString(famP, famB)
	if subBest >= 0 {
		subfamily = decodeNameString(subP, subB)
	}
	return family, subfamily, family != ""
}

// decodeNameString converts the raw bytes of a name record to a Go string.
// Windows (platform 3) and Unicode (platform 0) records are UTF-16BE; Mac
// (platform 1) is MacRoman in theory, but for our purposes ASCII-stripping
// is good enough — we only consume these strings for family matching.
func decodeNameString(platformID uint16, b []byte) string {
	if platformID == 3 || platformID == 0 {
		if len(b)%2 != 0 {
			return ""
		}
		u := make([]uint16, len(b)/2)
		for i := range u {
			u[i] = binary.BigEndian.Uint16(b[i*2 : i*2+2])
		}
		return string(utf16.Decode(u))
	}
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c >= 0x20 && c < 0x7F {
			out = append(out, c)
		}
	}
	return string(out)
}
