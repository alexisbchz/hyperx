// Package fontcfg is a tiny in-process fontconfig replacement that scans
// well-known font directories, parses TTF/OTF name tables via
// golang.org/x/image/font/sfnt, and resolves "family:size=N" patterns.
//
// It is sufficient for dmenu's needs (default monospace + per-codepoint
// fallback). It does not aim for full fontconfig compatibility.
package fontcfg

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/image/font/sfnt"
)

// Pattern is a parsed font request.
type Pattern struct {
	Family string  // e.g. "monospace", "DejaVu Sans Mono"
	Size   float64 // in points; 0 means unspecified
	Bold   bool
	Italic bool
}

// FontInfo is metadata for a single font file.
type FontInfo struct {
	Path    string
	Family  string
	SubFamily string // "Regular", "Bold", "Italic", ...
	Weight  int     // OS/2 weight class, 0 if unknown
	Slant   int     // 0 = roman, 1 = italic
	// internal: cached font (lazy)
	mu   sync.Mutex
	font *sfnt.Font
}

// DB is a font database built by scanning font dirs.
type DB struct {
	Fonts []*FontInfo
	byFam map[string][]*FontInfo // lower-case family
}

// Generic family aliases (lower-case → ranked list of family names to try).
var genericAliases = map[string][]string{
	"monospace": {
		"DejaVu Sans Mono",
		"Liberation Mono",
		"Noto Sans Mono",
		"Source Code Pro",
		"Hack",
		"Inconsolata",
		"FreeMono",
		"Courier New",
		"Courier",
	},
	"sans-serif": {
		"DejaVu Sans",
		"Liberation Sans",
		"Noto Sans",
		"Open Sans",
		"Roboto",
		"FreeSans",
		"Arial",
		"Helvetica",
	},
	"sans": {
		"DejaVu Sans", "Liberation Sans", "Noto Sans", "FreeSans", "Arial", "Helvetica",
	},
	"serif": {
		"DejaVu Serif", "Liberation Serif", "Noto Serif", "FreeSerif", "Times New Roman", "Times",
	},
	"emoji": {
		"Noto Color Emoji", "Twemoji", "Apple Color Emoji", "Segoe UI Emoji",
	},
}

// ParsePattern parses "family:size=N[:style=Bold][:weight=bold][:slant=italic]".
// Multiple families separated by "," can be given before the first ":"; the
// first is used as the primary.
func ParsePattern(s string) *Pattern {
	p := &Pattern{}
	parts := strings.Split(s, ":")
	if len(parts) > 0 {
		fam := strings.TrimSpace(strings.Split(parts[0], ",")[0])
		p.Family = fam
	}
	for _, opt := range parts[1:] {
		opt = strings.TrimSpace(opt)
		if eq := strings.IndexByte(opt, '='); eq > 0 {
			key := strings.ToLower(strings.TrimSpace(opt[:eq]))
			val := strings.TrimSpace(opt[eq+1:])
			switch key {
			case "size", "pixelsize":
				f, err := strconv.ParseFloat(val, 64)
				if err == nil {
					p.Size = f
					if key == "pixelsize" {
						// Approximate: 1pt ≈ 96/72 px → invert: pt = px * 72/96.
						p.Size = f * 72.0 / 96.0
					}
				}
			case "style":
				lv := strings.ToLower(val)
				if strings.Contains(lv, "bold") {
					p.Bold = true
				}
				if strings.Contains(lv, "italic") || strings.Contains(lv, "oblique") {
					p.Italic = true
				}
			case "weight":
				if strings.EqualFold(val, "bold") {
					p.Bold = true
				}
			case "slant":
				if strings.EqualFold(val, "italic") || strings.EqualFold(val, "oblique") {
					p.Italic = true
				}
			}
		} else {
			// bare modifier word
			lv := strings.ToLower(opt)
			if strings.Contains(lv, "bold") {
				p.Bold = true
			}
			if strings.Contains(lv, "italic") {
				p.Italic = true
			}
		}
	}
	if p.Size == 0 {
		p.Size = 10
	}
	return p
}

// DefaultDirs are the directories scanned by Load when no argument is given.
func DefaultDirs() []string {
	dirs := []string{
		"/usr/share/fonts",
		"/usr/local/share/fonts",
		"/usr/share/X11/fonts",
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs,
			filepath.Join(home, ".fonts"),
			filepath.Join(home, ".local/share/fonts"))
	}
	return dirs
}

// Load scans the given directories (or DefaultDirs() if none) for TTF/OTF
// files and builds a font database. Results are cached on disk and reused
// when the font directories' mtimes haven't changed.
func Load(dirs ...string) (*DB, error) {
	if len(dirs) == 0 {
		dirs = DefaultDirs()
	}
	fp := fingerprintDirs(dirs)
	if path, err := indexPath(); err == nil {
		if db, err := loadIndex(path, fp); err == nil && db != nil {
			return db, nil
		}
	}
	db, err := scan(dirs)
	if err != nil {
		return nil, err
	}
	if path, err := indexPath(); err == nil {
		_ = saveIndex(path, fp, db)
	}
	return db, nil
}

func scan(dirs []string) (*DB, error) {
	db := &DB{byFam: make(map[string][]*FontInfo)}
	for _, d := range dirs {
		if _, err := os.Stat(d); err != nil {
			continue
		}
		_ = filepath.WalkDir(d, func(path string, dent fs.DirEntry, err error) error {
			if err != nil || dent.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".ttf" && ext != ".otf" {
				return nil
			}
			fi := readFontInfo(path)
			if fi == nil {
				return nil
			}
			db.Fonts = append(db.Fonts, fi)
			key := strings.ToLower(fi.Family)
			db.byFam[key] = append(db.byFam[key], fi)
			return nil
		})
	}
	if len(db.Fonts) == 0 {
		return nil, errors.New("fontcfg: no fonts found in scanned directories")
	}
	return db, nil
}

func readFontInfo(path string) *FontInfo {
	fam, sub, ok := readNameTable(path)
	if !ok {
		return nil
	}
	return &FontInfo{
		Path:      path,
		Family:    fam,
		SubFamily: sub,
		Weight:    weightFromSubfamily(sub),
		Slant:     slantFromSubfamily(sub),
	}
}

func weightFromSubfamily(sub string) int {
	low := strings.ToLower(sub)
	switch {
	case strings.Contains(low, "bold"):
		return 700
	case strings.Contains(low, "light"):
		return 300
	default:
		return 400
	}
}

func slantFromSubfamily(sub string) int {
	low := strings.ToLower(sub)
	if strings.Contains(low, "italic") || strings.Contains(low, "oblique") {
		return 1
	}
	return 0
}

// Match returns the best font matching the pattern, or nil if none was found.
// If the pattern's family is a generic alias, each alternative is tried in
// order. The first match returns a font; if the family is found but the
// requested style is missing, the closest variant is returned.
func (db *DB) Match(p *Pattern) *FontInfo {
	// Build the list of families to try.
	var fams []string
	low := strings.ToLower(p.Family)
	if al, ok := genericAliases[low]; ok {
		fams = al
	}
	fams = append([]string{p.Family}, fams...)

	for _, fam := range fams {
		key := strings.ToLower(fam)
		cands := db.byFam[key]
		if len(cands) == 0 {
			// fuzzy: prefix match on family
			for k, list := range db.byFam {
				if strings.HasPrefix(k, key) {
					cands = append(cands, list...)
				}
			}
		}
		if len(cands) == 0 {
			continue
		}
		// Rank: prefer exact bold/italic match, else regular.
		sort.SliceStable(cands, func(i, j int) bool {
			return scoreStyle(cands[i], p) < scoreStyle(cands[j], p)
		})
		return cands[0]
	}
	// Last resort: any font.
	if len(db.Fonts) > 0 {
		return db.Fonts[0]
	}
	return nil
}

func scoreStyle(f *FontInfo, p *Pattern) int {
	want := 0
	if p.Bold {
		want |= 1
	}
	if p.Italic {
		want |= 2
	}
	got := 0
	if f.Weight >= 600 {
		got |= 1
	}
	if f.Slant > 0 {
		got |= 2
	}
	// Hamming-style distance.
	d := want ^ got
	return bitsSet(d)
}

func bitsSet(x int) int {
	n := 0
	for x != 0 {
		n += x & 1
		x >>= 1
	}
	return n
}

// MatchForRune scans the DB for any font containing the given rune. Returns
// nil if none does. Used for glyph fallback.
func (db *DB) MatchForRune(r rune) *FontInfo {
	for _, f := range db.Fonts {
		ff, err := f.Load()
		if err != nil {
			continue
		}
		var buf sfnt.Buffer
		idx, err := ff.GlyphIndex(&buf, r)
		if err == nil && idx != 0 {
			return f
		}
	}
	return nil
}

// Load returns the parsed sfnt.Font, lazily reparsing if it was dropped.
func (f *FontInfo) Load() (*sfnt.Font, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.font != nil {
		return f.font, nil
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, err
	}
	ff, err := sfnt.Parse(data)
	if err != nil {
		return nil, err
	}
	f.font = ff
	return ff, nil
}
