package fonts

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Chinese, Japanese and Korean characters, which no font of the reader has, are drawn by the
// CJK font of the text's family: Noto Serif CJK for serif text, Noto Sans CJK otherwise.
func TestCJKFallbacks(t *testing.T) {
	for _, serif := range []bool{false, true} {
		want := cjkSans.name
		if serif {
			want = cjkSerif.name
		}
		fallbacks, err := OpenFallbacks(Regular, 40, serif)
		if err != nil {
			t.Fatal(err)
		}
		for _, character := range "永한中" {
			found := ""
			for _, fallback := range fallbacks {
				if fallback.Has(character) {
					found = fallback.Name
					break
				}
			}
			if found != want {
				t.Errorf("serif %v: %c drawn by %q, want %q", serif, character, found, want)
			}
		}
	}
}

// A bold CJK font is the regular one emboldened: its strokes are wider, its advances the same.
func TestCJKBold(t *testing.T) {
	regular, err := OpenFallbacks(Regular, 40, false)
	if err != nil {
		t.Fatal(err)
	}
	bold, err := OpenFallbacks(Bold, 40, false)
	if err != nil {
		t.Fatal(err)
	}
	regularGlyph, boldGlyph := regular[0].glyph('永'), bold[0].glyph('永')
	if boldGlyph.mask.Rect.Dx() <= regularGlyph.mask.Rect.Dx() || boldGlyph.advance != regularGlyph.advance {
		t.Errorf("bold 永 %v wide, advance %v; regular %v wide, advance %v", boldGlyph.mask.Rect.Dx(), boldGlyph.advance, regularGlyph.mask.Rect.Dx(), regularGlyph.advance)
	}
}

// The CJK font is decompressed into the cache the first time, then read from it: the file is
// written once.
func TestUnpackFontCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fonts", cjkSans.name)
	first, err := unpackFont(cjkSans.name, dir)
	if err != nil {
		t.Fatal(err)
	}
	size, err := unpackedSize(cjkSans.name)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() != int64(size) || len(first) != size {
		t.Fatalf("cache %v (%v), unpacked %d bytes, want %d", info, err, len(first), size)
	}
	if _, err := Parse(cjkSans.name, first); err != nil {
		t.Errorf("unpacked font: %v", err)
	}
	written := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, written, written); err != nil {
		t.Fatal(err)
	}
	second, err := unpackFont(cjkSans.name, dir)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || !info.ModTime().Equal(written) {
		t.Errorf("cache written again: %v, %v", info, err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("the cached font differs from the unpacked one")
	}
}

// A cached font of the wrong size, written in part when the reader went off, is written again.
func TestUnpackFontRewritesPartialCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fonts", cjkSans.name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("OTTO and then nothing"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := unpackFont(cjkSans.name, dir)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() != int64(len(data)) {
		t.Errorf("cache %v (%v), want %d bytes", info, err, len(data))
	}
	if _, err := Parse(cjkSans.name, data); err != nil {
		t.Errorf("unpacked font: %v", err)
	}
}

// A font file is read and parsed once: its sizes, and the CJK font's, share the parsed file.
func TestFontFileParsedOnce(t *testing.T) {
	small, err := Open(SansSerif, Regular, 20)
	if err != nil {
		t.Fatal(err)
	}
	large, err := Open(SansSerif, Regular, 40)
	if err != nil {
		t.Fatal(err)
	}
	if small.file != large.file {
		t.Errorf("sizes 20 and 40 parsed apart")
	}
	smallFallbacks, err := OpenFallbacks(Regular, 20, true)
	if err != nil {
		t.Fatal(err)
	}
	largeFallbacks, err := OpenFallbacks(Bold, 40, true)
	if err != nil {
		t.Fatal(err)
	}
	if smallFallbacks[0].file != largeFallbacks[0].file || smallFallbacks[0].file != cjkSerif.parsed() {
		t.Errorf("the CJK font's sizes parsed apart")
	}
}
