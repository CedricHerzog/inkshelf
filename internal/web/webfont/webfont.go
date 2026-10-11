package webfont

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"slices"

	"github.com/tdewolff/font"
)

// ErrCollection is the error of a TrueType or OpenType collection (.ttc), which holds several
// fonts.
var ErrCollection = errors.New("webfont: font collections are not supported")

// ErrUnknownFormat is the error of data that is not a font ToSFNT knows.
var ErrUnknownFormat = errors.New("webfont: unknown font format")

// ToSFNT returns font data as TrueType or OpenType data: TrueType and OpenType data as is, WOFF
// and WOFF2 decoded. It never modifies data.
func ToSFNT(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, ErrUnknownFormat
	}
	switch string(data[:4]) {
	case "\x00\x01\x00\x00", "true", "OTTO":
		return data, nil
	case "ttcf":
		return nil, ErrCollection
	case "wOFF":
		return decodeWOFF(data)
	case "wOF2":
		// WOFF2's Brotli and table transforms (glyf, loca, hmtx) are tdewolff/font's.
		if len(data) >= 8 && string(data[4:8]) == "ttcf" {
			return nil, ErrCollection
		}
		decoded, err := font.ParseWOFF2(data)
		if err != nil {
			return nil, fmt.Errorf("webfont: WOFF2: %w", err)
		}
		return decoded, nil
	}
	return nil, ErrUnknownFormat
}

// The sizes of WOFF's header and table directory entries (https://www.w3.org/TR/WOFF/), and of
// an sfnt's.
const (
	woffHeaderSize = 44
	woffEntrySize  = 20
	sfntHeaderSize = 12
	sfntEntrySize  = 16
)

// sfntTable is a table of an sfnt.
type sfntTable struct {
	tag      [4]byte
	checksum uint32
	data     []byte
}

// decodeWOFF returns the sfnt of WOFF data: its tables, each stored as is or compressed with
// zlib, laid out after a new table directory. The checksums are WOFF's, those of the original
// tables; the header's totalSfntSize and the table order are not trusted.
func decodeWOFF(data []byte) ([]byte, error) {
	if len(data) < woffHeaderSize {
		return nil, errors.New("webfont: WOFF: truncated header")
	}
	tableCount := int(binary.BigEndian.Uint16(data[12:]))
	if len(data) < woffHeaderSize+tableCount*woffEntrySize {
		return nil, errors.New("webfont: WOFF: truncated table directory")
	}
	tables := make([]sfntTable, tableCount)
	for index := range tableCount {
		entry := data[woffHeaderSize+index*woffEntrySize:][:woffEntrySize]
		table := &tables[index]
		copy(table.tag[:], entry)
		table.checksum = binary.BigEndian.Uint32(entry[16:])
		// In uint64: an int is 32 bits on the reader.
		offset, storedLength := uint64(binary.BigEndian.Uint32(entry[4:])), uint64(binary.BigEndian.Uint32(entry[8:]))
		length := uint64(binary.BigEndian.Uint32(entry[12:]))
		if offset+storedLength > uint64(len(data)) {
			return nil, fmt.Errorf("webfont: WOFF: table %q past the end", table.tag)
		}
		stored := data[offset : offset+storedLength]
		if storedLength == length {
			table.data = stored
			continue
		}
		reader, err := zlib.NewReader(bytes.NewReader(stored))
		if err != nil {
			return nil, fmt.Errorf("webfont: WOFF: table %q: %w", table.tag, err)
		}
		// Read at most one byte more than announced, so that a lying length costs nothing.
		table.data, err = io.ReadAll(io.LimitReader(reader, int64(length)+1))
		if err != nil {
			return nil, fmt.Errorf("webfont: WOFF: table %q: %w", table.tag, err)
		}
		if uint64(len(table.data)) != length {
			return nil, fmt.Errorf("webfont: WOFF: table %q is %d bytes, not %d", table.tag, len(table.data), length)
		}
	}
	return buildSFNT(data[4:8], tables), nil
}

// buildSFNT lays out an sfnt of flavor (its version tag) with tables: the header, the table
// directory sorted by tag, then each table 4-aligned, padded with zeros.
func buildSFNT(flavor []byte, tables []sfntTable) []byte {
	slices.SortFunc(tables, func(first, second sfntTable) int { return bytes.Compare(first.tag[:], second.tag[:]) })
	size := sfntHeaderSize + len(tables)*sfntEntrySize
	for _, table := range tables {
		size += padded(len(table.data))
	}
	sfnt := make([]byte, size)
	copy(sfnt, flavor)
	// The binary search hints: the largest power of 2 not above the table count, and its log.
	tableCount := len(tables)
	entrySelector := max(bits.Len(uint(tableCount))-1, 0)
	searchRange := (1 << entrySelector) * sfntEntrySize
	binary.BigEndian.PutUint16(sfnt[4:], uint16(tableCount))
	binary.BigEndian.PutUint16(sfnt[6:], uint16(searchRange))
	binary.BigEndian.PutUint16(sfnt[8:], uint16(entrySelector))
	binary.BigEndian.PutUint16(sfnt[10:], uint16(tableCount*sfntEntrySize-searchRange))
	offset := sfntHeaderSize + tableCount*sfntEntrySize
	for index, table := range tables {
		entry := sfnt[sfntHeaderSize+index*sfntEntrySize:]
		copy(entry, table.tag[:])
		binary.BigEndian.PutUint32(entry[4:], table.checksum)
		binary.BigEndian.PutUint32(entry[8:], uint32(offset))
		binary.BigEndian.PutUint32(entry[12:], uint32(len(table.data)))
		copy(sfnt[offset:], table.data)
		offset += padded(len(table.data))
	}
	return sfnt
}

// padded returns length rounded up to a multiple of 4.
func padded(length int) int {
	return (length + 3) &^ 3
}
