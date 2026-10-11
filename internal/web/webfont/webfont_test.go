package webfont

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"image"
	"os"
	"slices"
	"testing"

	"github.com/tdewolff/font"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// sampleRunes are the characters whose glyphs the tests compare.
const sampleRunes = "AHagjyz09?&é"

// openFace parses TrueType or OpenType data and opens it 40 pixels high.
func openFace(t *testing.T, data []byte) xfont.Face {
	t.Helper()
	parsed, err := opentype.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: 40, DPI: 72, Hinting: xfont.HintingFull})
	if err != nil {
		t.Fatal(err)
	}
	return face
}

// checkSameGlyphs checks that the fonts of got and want have the same advances and outlines'
// bounds for sampleRunes.
func checkSameGlyphs(t *testing.T, got, want []byte) {
	t.Helper()
	gotFace, wantFace := openFace(t, got), openFace(t, want)
	for _, character := range sampleRunes {
		gotBounds, gotAdvance, gotOK := gotFace.GlyphBounds(character)
		wantBounds, wantAdvance, wantOK := wantFace.GlyphBounds(character)
		if gotBounds != wantBounds || gotAdvance != wantAdvance || gotOK != wantOK {
			t.Errorf("%q: bounds %v, advance %v, %v; want %v, %v, %v", character, gotBounds, gotAdvance, gotOK, wantBounds, wantAdvance, wantOK)
		}
	}
}

// checkDraws checks that the font of data draws character.
func checkDraws(t *testing.T, data []byte, character rune) {
	t.Helper()
	rect, mask, _, _, ok := openFace(t, data).Glyph(fixed.P(0, 40), character)
	if !ok || rect.Empty() {
		t.Fatalf("%q: no glyph", character)
	}
	alpha, isAlpha := mask.(*image.Alpha)
	if !isAlpha || !slices.ContainsFunc(alpha.Pix, func(value byte) bool { return value != 0 }) {
		t.Errorf("%q: nothing drawn", character)
	}
}

// encodeWOFF returns the WOFF of an sfnt, its tables compressed with zlib when that makes
// them smaller and compress is set, in the sfnt's order (sorted by tag), packed back to back
// without padding.
func encodeWOFF(t *testing.T, sfnt []byte, compress bool) []byte {
	t.Helper()
	tableCount := int(binary.BigEndian.Uint16(sfnt[4:]))
	header := make([]byte, woffHeaderSize+tableCount*woffEntrySize)
	copy(header, "wOFF")
	copy(header[4:], sfnt[:4])
	binary.BigEndian.PutUint16(header[12:], uint16(tableCount))
	var tables bytes.Buffer
	for index := range tableCount {
		record := sfnt[sfntHeaderSize+index*sfntEntrySize:][:sfntEntrySize]
		offset, length := binary.BigEndian.Uint32(record[8:]), binary.BigEndian.Uint32(record[12:])
		table := sfnt[offset : offset+length]
		stored := table
		if compress {
			var compressed bytes.Buffer
			writer := zlib.NewWriter(&compressed)
			if _, err := writer.Write(table); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if compressed.Len() < len(table) {
				stored = compressed.Bytes()
			}
		}
		entry := header[woffHeaderSize+index*woffEntrySize:]
		copy(entry, record[:4])
		binary.BigEndian.PutUint32(entry[4:], uint32(len(header)+tables.Len()))
		binary.BigEndian.PutUint32(entry[8:], uint32(len(stored)))
		binary.BigEndian.PutUint32(entry[12:], length)
		binary.BigEndian.PutUint32(entry[16:], binary.BigEndian.Uint32(record[4:]))
		tables.Write(stored)
	}
	binary.BigEndian.PutUint32(header[8:], uint32(len(header)+tables.Len()))
	return append(header, tables.Bytes()...)
}

func TestToSFNTKeepsSFNT(t *testing.T) {
	decoded, err := ToSFNT(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	if &decoded[0] != &goregular.TTF[0] {
		t.Errorf("TrueType data was copied")
	}
}

func TestToSFNTRejects(t *testing.T) {
	collection := append([]byte("ttcf"), make([]byte, 40)...)
	if _, err := ToSFNT(collection); !errors.Is(err, ErrCollection) {
		t.Errorf("collection: %v, want ErrCollection", err)
	}
	for _, data := range [][]byte{nil, []byte("PK\x03\x04 not a font")} {
		if _, err := ToSFNT(data); !errors.Is(err, ErrUnknownFormat) {
			t.Errorf("%q: %v, want ErrUnknownFormat", data, err)
		}
	}
	truncated := encodeWOFF(t, goregular.TTF, true)[:200]
	if _, err := ToSFNT(truncated); err == nil {
		t.Errorf("truncated WOFF: no error")
	}
}

func TestWOFF(t *testing.T) {
	for _, compress := range []bool{true, false} {
		woff := encodeWOFF(t, goregular.TTF, compress)
		original := bytes.Clone(woff)
		decoded, err := ToSFNT(woff)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(woff, original) {
			t.Errorf("ToSFNT modified its input")
		}
		checkSameGlyphs(t, decoded, goregular.TTF)
		gotTables, wantTables := tablesOf(t, decoded), tablesOf(t, goregular.TTF)
		if len(gotTables) != len(wantTables) {
			t.Errorf("%d tables, want %d", len(gotTables), len(wantTables))
		}
		for tag, want := range wantTables {
			if !bytes.Equal(gotTables[tag], want) {
				t.Errorf("compress %v: table %q differs", compress, tag)
			}
		}
	}
}

// tablesOf returns the tables of an sfnt by tag, checking that they are 4-aligned.
func tablesOf(t *testing.T, sfnt []byte) map[string][]byte {
	t.Helper()
	tables := map[string][]byte{}
	for index := range int(binary.BigEndian.Uint16(sfnt[4:])) {
		record := sfnt[sfntHeaderSize+index*sfntEntrySize:][:sfntEntrySize]
		offset, length := binary.BigEndian.Uint32(record[8:]), binary.BigEndian.Uint32(record[12:])
		if offset%4 != 0 {
			t.Errorf("table %q at %d, not 4-aligned", record[:4], offset)
		}
		tables[string(record[:4])] = sfnt[offset : offset+length]
	}
	return tables
}

func TestWOFF2(t *testing.T) {
	// tdewolff/font's encoder transforms glyf, loca and hmtx.
	parsed, err := font.ParseSFNT(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	woff2, err := parsed.WriteWOFF2()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ToSFNT(woff2)
	if err != nil {
		t.Fatal(err)
	}
	checkSameGlyphs(t, decoded, goregular.TTF)
	checkDraws(t, decoded, 'g')
}

// TestWOFF2File decodes a WOFF2 made by Google's encoder, whose glyf and loca are transformed:
// KaTeX's sans serif font, under the SIL Open Font License 1.1 (copyright and license in its
// name table).
func TestWOFF2File(t *testing.T) {
	woff2, err := os.ReadFile("testdata/KaTeX_SansSerif-Regular.woff2")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ToSFNT(woff2)
	if err != nil {
		t.Fatal(err)
	}
	for _, character := range "Aag" {
		checkDraws(t, decoded, character)
	}
}

func BenchmarkWOFF2(b *testing.B) {
	woff2, err := os.ReadFile("testdata/KaTeX_SansSerif-Regular.woff2")
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := ToSFNT(woff2); err != nil {
			b.Fatal(err)
		}
	}
}
