package drw

import "testing"

func TestParseColorHexRGB(t *testing.T) {
	c, err := ParseColor("#abc")
	if err != nil {
		t.Fatalf("parse #abc: %v", err)
	}
	if c.R != 0xaa || c.G != 0xbb || c.B != 0xcc || c.A != 0xff {
		t.Errorf("#abc -> %v", c)
	}
}

func TestParseColorHexRRGGBB(t *testing.T) {
	c, err := ParseColor("#005577")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.R != 0x00 || c.G != 0x55 || c.B != 0x77 || c.A != 0xff {
		t.Errorf("#005577 -> %v", c)
	}
}

func TestParseColorName(t *testing.T) {
	c, err := ParseColor("black")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.R != 0 || c.G != 0 || c.B != 0 {
		t.Errorf("black -> %v", c)
	}
}

func TestParseColorBad(t *testing.T) {
	if _, err := ParseColor("not-a-color"); err == nil {
		t.Errorf("expected error for invalid color")
	}
}
