package webfont

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // G505: the IDPF font obfuscation is defined with SHA-1
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// The obfuscation algorithms of an EPUB's META-INF/encryption.xml, as its EncryptionMethod
// Algorithm attribute names them.
const (
	// IDPFAlgorithm is EPUB's (https://www.w3.org/TR/epub-33/#sec-font-obfuscation).
	IDPFAlgorithm = "http://www.idpf.org/2008/embedding"
	// AdobeAlgorithm is Adobe's, older.
	AdobeAlgorithm = "http://ns.adobe.com/pdf/enc#RC"
)

// ErrUnknownAlgorithm is the error of an obfuscation algorithm Deobfuscate doesn't know: a
// font encrypted with DRM, say.
var ErrUnknownAlgorithm = errors.New("webfont: unknown obfuscation algorithm")

// The lengths of the obfuscated start of a font, in bytes.
const (
	idpfObfuscatedLength  = 1040
	adobeObfuscatedLength = 1024
)

// Deobfuscate returns a copy of an obfuscated font's data in clear, given the algorithm and
// the identifier of the publication: its unique identifier (dc:identifier) for IDPFAlgorithm, a
// UUID ("urn:uuid:…") for AdobeAlgorithm. Both XOR the start of the font with a key, so
// Deobfuscate also obfuscates.
func Deobfuscate(data []byte, algorithm, identifier string) ([]byte, error) {
	var key []byte
	var obfuscatedLength int
	switch algorithm {
	case IDPFAlgorithm:
		key, obfuscatedLength = idpfKey(identifier), idpfObfuscatedLength
	case AdobeAlgorithm:
		var err error
		if key, err = adobeKey(identifier); err != nil {
			return nil, err
		}
		obfuscatedLength = adobeObfuscatedLength
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownAlgorithm, algorithm)
	}
	return xorStart(data, key, obfuscatedLength), nil
}

// xorStart returns a copy of data whose first length bytes are XORed with key, repeated.
func xorStart(data, key []byte, length int) []byte {
	plain := bytes.Clone(data)
	for index := range min(length, len(plain)) {
		plain[index] ^= key[index%len(key)]
	}
	return plain
}

// sfntVersions are the first 4 bytes of a TrueType or OpenType font.
var sfntVersions = [][]byte{{0, 1, 0, 0}, []byte("OTTO"), []byte("true")}

// tableTags are the tags of the tables a font may have: those of the first table in a font's
// table directory, which is sorted by tag, are tried as the clear text of an Adobe key.
var tableTags = []string{
	"BASE", "CBDT", "CBLC", "CFF ", "CFF2", "COLR", "CPAL", "DSIG", "EBDT", "EBLC", "EBSC", "FFTM",
	"GDEF", "GPOS", "GSUB", "HVAR", "JSTF", "LTSH", "MATH", "MERG", "MVAR", "OS/2", "PCLT", "STAT",
	"SVG ", "VDMX", "VORG", "VVAR", "avar", "cmap", "cvar", "cvt ", "fpgm", "fvar", "gasp", "glyf",
	"gvar", "hdmx", "head", "hhea", "hmtx", "kern", "loca", "maxp", "meta", "name", "post", "prep",
	"sbix", "vhea", "vmtx",
}

// RecoverAdobe returns a copy of a font obfuscated with AdobeAlgorithm in clear, its key found
// from the font itself: for books that lost the UUID it was obfuscated with. The key repeats
// every 16 bytes over the font's table directory, whose start follows from the sfnt version and
// the number of tables (bytes 0 to 11), and whose tables' tags (bytes 12 to 15 of every 16) are
// known names in order.
func RecoverAdobe(data []byte) ([]byte, error) {
	if len(data) < 32 {
		return nil, errors.New("webfont: Adobe obfuscation: the font is too short")
	}
	key := make([]byte, 16)
	for _, version := range sfntVersions {
		for index := range 64 {
			tables := index + 1
			header := offsetTable(version, tables)
			for index := range header {
				key[index] = data[index] ^ header[index]
			}
			for _, tag := range tableTags {
				for index := range 4 {
					key[12+index] = data[12+index] ^ tag[index]
				}
				if plain := xorStart(data, key, adobeObfuscatedLength); validDirectory(plain, tables) {
					return plain, nil
				}
			}
		}
	}
	return nil, errors.New("webfont: Adobe obfuscation: no key fits the font's header")
}

// offsetTable returns the first 12 bytes of an sfnt font of version with tables tables: its
// version, numTables, searchRange, entrySelector and rangeShift.
func offsetTable(version []byte, tables int) []byte {
	entrySelector := 0
	for 2<<entrySelector <= tables {
		entrySelector++
	}
	searchRange := 16 << entrySelector
	header := append(bytes.Clone(version), byte(tables>>8), byte(tables))
	header = append(header, byte(searchRange>>8), byte(searchRange), 0, byte(entrySelector))
	rangeShift := tables*16 - searchRange
	return append(header, byte(rangeShift>>8), byte(rangeShift))
}

// validDirectory reports whether the table directory of font, of tables tables, is a real one:
// known tags in increasing order, each table within the font.
func validDirectory(font []byte, tables int) bool {
	if len(font) < 12+16*tables {
		return false
	}
	previous := ""
	for table := range tables {
		record := font[12+16*table:]
		tag := string(record[:4])
		offset, length := binary.BigEndian.Uint32(record[8:]), binary.BigEndian.Uint32(record[12:])
		if tag <= previous || !slices.Contains(tableTags, tag) || uint64(offset)+uint64(length) > uint64(len(font)) {
			return false
		}
		previous = tag
	}
	return true
}

// idpfKey returns the IDPF key of identifier: the SHA-1 of the identifier without its spaces,
// tabs and line breaks.
func idpfKey(identifier string) []byte {
	stripped := strings.Map(func(character rune) rune {
		if strings.ContainsRune(" \t\r\n", character) {
			return -1
		}
		return character
	}, identifier)
	sum := sha1.Sum([]byte(stripped)) //nolint:gosec // G401: the IDPF font obfuscation is defined with SHA-1
	return sum[:]
}

// adobeKey returns the Adobe key of identifier: the 16 bytes of its UUID.
func adobeKey(identifier string) ([]byte, error) {
	digits := strings.TrimSpace(identifier)
	digits = strings.TrimPrefix(strings.ToLower(digits), "urn:uuid:")
	digits = strings.NewReplacer("-", "", ":", "").Replace(digits)
	key, err := hex.DecodeString(digits)
	if err != nil || len(key) != 16 {
		return nil, fmt.Errorf("webfont: Adobe obfuscation: %q is not a UUID", identifier)
	}
	return key, nil
}
