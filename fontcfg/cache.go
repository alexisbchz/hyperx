package fontcfg

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// indexVersion bumps whenever the on-disk format changes.
const indexVersion = 1

// fingerprintDirs walks dirs collecting (path, mtime) of every directory
// (not files) and returns an FNV-64 hex digest. A new font added by any
// reasonable package manager bumps the containing directory's mtime, so
// any add/remove invalidates the cache. Files modified in-place without a
// directory mtime bump (vanishingly rare) won't invalidate — users can
// delete the cache file by hand if needed.
//
// This walk only touches directory entries, so it's ~1ms even on a system
// with hundreds of font subdirs.
func fingerprintDirs(dirs []string) string {
	h := fnv.New64a()
	for _, root := range dirs {
		if _, err := os.Stat(root); err != nil {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			fmt.Fprintf(h, "%s\x00%d\n", path, info.ModTime().UnixNano())
			return nil
		})
	}
	var out [8]byte
	sum := h.Sum64()
	for i := range out {
		out[7-i] = byte(sum >> (i * 8))
	}
	return hex.EncodeToString(out[:])
}

func indexPath() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "hypermenu", "fonts.idx"), nil
}

// loadIndex reads a persisted DB whose fingerprint matches fp. Returns
// (nil, nil) when the file is missing or stale — callers should rebuild.
func loadIndex(path, fp string) (*DB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	if !sc.Scan() {
		return nil, nil
	}
	header := sc.Text()
	var ver int
	var gotFP string
	if _, err := fmt.Sscanf(header, "# hypermenu fonts.idx v%d fp=%s", &ver, &gotFP); err != nil {
		return nil, nil
	}
	if ver != indexVersion || gotFP != fp {
		return nil, nil
	}

	db := &DB{byFam: make(map[string][]*FontInfo)}
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 5 {
			continue
		}
		w, err1 := strconv.Atoi(parts[3])
		s, err2 := strconv.Atoi(parts[4])
		if err1 != nil || err2 != nil {
			continue
		}
		fi := &FontInfo{
			Path:      parts[0],
			Family:    parts[1],
			SubFamily: parts[2],
			Weight:    w,
			Slant:     s,
		}
		db.Fonts = append(db.Fonts, fi)
		key := strings.ToLower(fi.Family)
		db.byFam[key] = append(db.byFam[key], fi)
	}
	if err := sc.Err(); err != nil || len(db.Fonts) == 0 {
		return nil, nil
	}
	return db, nil
}

func saveIndex(path, fp string, db *DB) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "fonts.idx.*")
	if err != nil {
		return err
	}
	w := bufio.NewWriter(tmp)
	fmt.Fprintf(w, "# hypermenu fonts.idx v%d fp=%s\n", indexVersion, fp)
	for _, fi := range db.Fonts {
		// Tabs aren't valid in font paths or family names in practice;
		// skip the rare offender rather than escape.
		if strings.ContainsAny(fi.Path, "\t\n") ||
			strings.ContainsAny(fi.Family, "\t\n") ||
			strings.ContainsAny(fi.SubFamily, "\t\n") {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\n",
			fi.Path, fi.Family, fi.SubFamily, fi.Weight, fi.Slant)
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
