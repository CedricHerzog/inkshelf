package pageimage

// A .pg4 file holds a page prepared for the panel, in the panel's 16 grays. Four bits a pixel
// make it half the size of an 8-bit gray image, and reading it back needs no decoder, only a
// table:
//
//	offset 0  "PG4\x01" (pg4Magic)
//	offset 4  the width, a little-endian uint16
//	offset 6  the height, a little-endian uint16
//	offset 8  the rows, top to bottom, each (width+1)/2 bytes
//
// Each byte holds two pixels, the left one in its high 4 bits. A pixel is an index from 0
// (black) to 15 (white), and its gray is index × grayStep. A row of odd width ends with a
// byte whose low 4 bits are unused.

import (
	"encoding/binary"
	"errors"
	"image"
)

const (
	pg4Magic        = "PG4\x01"
	pg4WidthOffset  = len(pg4Magic)
	pg4HeightOffset = pg4WidthOffset + 2
	pg4Header       = pg4HeightOffset + 2 // the size of the magic, width and height
)

const (
	// panelGrays is the number of grays the e-ink panel shows. The emulator's panel
	// (pocketbook's emulated firmware) rounds to as many.
	panelGrays = 16
	// grayStep is the difference between two neighbouring panel grays, from 0 to 255.
	grayStep = 255 / (panelGrays - 1)
)

// newPG4 returns a .pg4 page of width×height, its pixels all black, and the size of its rows.
func newPG4(width, height int) ([]byte, int) {
	rowBytes := (width + 1) / 2 // two pixels per byte, rounded up
	out := make([]byte, pg4Header+rowBytes*height)
	copy(out, pg4Magic)
	binary.LittleEndian.PutUint16(out[pg4WidthOffset:], uint16(width))
	binary.LittleEndian.PutUint16(out[pg4HeightOffset:], uint16(height))
	return out, rowBytes
}

// gray4 maps a byte of a .pg4 page to the grays of its two pixels.
var gray4 = func() (table [256][2]uint8) {
	for packed := range table {
		table[packed] = [2]uint8{uint8(packed>>4) * grayStep, uint8(packed&15) * grayStep}
	}
	return table
}()

// Unpack decodes a .pg4 page.
//
// HOT PATH, run at each page turn while the reader waits for the next page. It
// was tuned against the ARM assembly; run BenchmarkUnpack before and after changing it.
func Unpack(data []byte) (*image.Gray, error) {
	if len(data) < pg4Header || string(data[:len(pg4Magic)]) != pg4Magic {
		return nil, errors.New("not a page file")
	}
	width := int(binary.LittleEndian.Uint16(data[pg4WidthOffset:]))
	height := int(binary.LittleEndian.Uint16(data[pg4HeightOffset:]))
	rowBytes := (width + 1) / 2
	if len(data) != pg4Header+rowBytes*height {
		return nil, errors.New("truncated page file")
	}
	gray := image.NewGray(image.Rect(0, 0, width, height))
	for row := range height {
		packed := data[pg4Header+row*rowBytes : pg4Header+(row+1)*rowBytes]
		unpackRow(gray.Pix[row*width:(row+1)*width], packed)
	}
	return gray, nil
}

// unpackRow decodes a row of a .pg4 page, packed, into the grays of its pixels, grays.
func unpackRow(grays, packed []byte) {
	for index, pair := range packed[:len(grays)/2] {
		grays[2*index], grays[2*index+1] = gray4[pair][0], gray4[pair][1]
	}
	if len(grays)%2 == 1 { // the last pixel, alone in the high bits of the last byte
		grays[len(grays)-1] = gray4[packed[len(packed)-1]][0]
	}
}
