package fonts

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
)

// Chinese, Japanese and Korean characters, where the firmware has no CJK font (the tests' case),
// are drawn by the program's CJK font of the text's family: Noto Serif CJK for serif text, Noto
// Sans CJK otherwise.
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

// collectionOf returns a font collection (a .ttc file) of fonts: its header, then each font with
// the offsets of its tables moved to where they land.
func collectionOf(fonts ...[]byte) []byte {
	header := 12 + 4*len(fonts)
	data := make([]byte, header)
	copy(data, "ttcf")
	binary.BigEndian.PutUint32(data[4:], 0x00010000)
	binary.BigEndian.PutUint32(data[8:], uint32(len(fonts)))
	for index, font := range fonts {
		for len(data)%4 != 0 {
			data = append(data, 0)
		}
		start := len(data)
		binary.BigEndian.PutUint32(data[12+4*index:], uint32(start))
		data = append(data, font...)
		tables := int(binary.BigEndian.Uint16(font[4:]))
		for table := range tables {
			offset := start + 12 + 16*table + 8
			binary.BigEndian.PutUint32(data[offset:], binary.BigEndian.Uint32(data[offset:])+uint32(start))
		}
	}
	return data
}

// A font of a collection is found by its family, and shaped from the collection too.
func TestParseCollectionFace(t *testing.T) {
	data := collectionOf(goregular.TTF, gomono.TTF)
	file, err := parseCollectionFace("go.ttc", data, "Go Mono")
	if err != nil {
		t.Fatal(err)
	}
	font, err := file.Open(40)
	if err != nil {
		t.Fatal(err)
	}
	defer font.Close()
	if font.Width("i") != font.Width("m") {
		t.Errorf("i %d and m %d wide: not Go Mono", font.Width("i"), font.Width("m"))
	}
	if shaped := font.Shape("im", false, ""); len(shaped) != 2 || shaped[0].Advance != shaped[1].Advance {
		t.Errorf("shaped %+v, want two glyphs of Go Mono's one advance", shaped)
	}
	if _, err := parseCollectionFace("go.ttc", data, "Noto Sans CJK JP"); err == nil {
		t.Error("a family the collection lacks was found")
	}
}

// The firmware's CJK collection, when a font directory has it, is used rather than the
// program's font, which is not unpacked.
func TestCJKFromFirmware(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cjk.ttc"), collectionOf(goregular.TTF, gomono.TTF), 0o644); err != nil {
		t.Fatal(err)
	}
	saved := fontDirs
	fontDirs = []string{dir}
	t.Cleanup(func() { fontDirs = saved })
	font := &cjkFont{firmware: "cjk.ttc", family: "Go Mono", name: "not-compiled-in.otf"}
	file := font.parsed()
	if file == nil || !file.collection || file.face != 1 {
		t.Fatalf("parsed %+v, want face 1 of the firmware's collection", file)
	}
	missing := &cjkFont{firmware: "none.ttc", family: "Go Mono", name: cjkSans.name}
	if file := missing.parsed(); file == nil || file.collection {
		t.Errorf("without the firmware's collection, parsed %+v, want the program's font", file)
	}
}
