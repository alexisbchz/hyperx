package drw

import (
	"errors"
	"image/color"
	"strconv"
	"strings"
)

// ParseColor accepts "#RGB", "#RRGGBB", "#RRRRGGGGBBBB" or a small set of X11
// color names. Returns an RGBA color with full alpha.
func ParseColor(s string) (color.RGBA, error) {
	if name, ok := x11ColorNames[strings.ToLower(s)]; ok {
		s = name
	}
	if len(s) > 0 && s[0] == '#' {
		hex := s[1:]
		switch len(hex) {
		case 3:
			r, err := strconv.ParseUint(hex[0:1], 16, 8)
			if err != nil {
				return color.RGBA{}, err
			}
			g, err := strconv.ParseUint(hex[1:2], 16, 8)
			if err != nil {
				return color.RGBA{}, err
			}
			b, err := strconv.ParseUint(hex[2:3], 16, 8)
			if err != nil {
				return color.RGBA{}, err
			}
			return color.RGBA{R: uint8(r) * 17, G: uint8(g) * 17, B: uint8(b) * 17, A: 0xff}, nil
		case 6:
			r, err := strconv.ParseUint(hex[0:2], 16, 8)
			if err != nil {
				return color.RGBA{}, err
			}
			g, err := strconv.ParseUint(hex[2:4], 16, 8)
			if err != nil {
				return color.RGBA{}, err
			}
			b, err := strconv.ParseUint(hex[4:6], 16, 8)
			if err != nil {
				return color.RGBA{}, err
			}
			return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xff}, nil
		case 12:
			r, err := strconv.ParseUint(hex[0:4], 16, 16)
			if err != nil {
				return color.RGBA{}, err
			}
			g, err := strconv.ParseUint(hex[4:8], 16, 16)
			if err != nil {
				return color.RGBA{}, err
			}
			b, err := strconv.ParseUint(hex[8:12], 16, 16)
			if err != nil {
				return color.RGBA{}, err
			}
			return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}, nil
		}
	}
	return color.RGBA{}, errors.New("drw: cannot parse color " + s)
}

// Pixel returns the 32-bit BGRA value (X11 little-endian truecolor depth 24/32).
func Pixel(c color.RGBA) uint32 {
	return uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
}

// A small subset of X11 color names used by themes.
var x11ColorNames = map[string]string{
	"black":   "#000000",
	"white":   "#ffffff",
	"red":     "#ff0000",
	"green":   "#00ff00",
	"blue":    "#0000ff",
	"cyan":    "#00ffff",
	"magenta": "#ff00ff",
	"yellow":  "#ffff00",
	"gray":    "#bebebe",
	"grey":    "#bebebe",
}
