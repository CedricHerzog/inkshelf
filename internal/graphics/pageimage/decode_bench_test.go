package pageimage

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"math/bits"
	"math/rand/v2"
	"testing"

	"github.com/gen2brain/jxl"
)

// BenchmarkDecode decodes benchmarkPage's image in each format a page comes in, to compare
// what the decoders cost before toGray and fit, which have benchmarks of their own.
func BenchmarkDecode(b *testing.B) {
	page := benchmarkPageImage()
	var pngData, jxlData bytes.Buffer
	if err := png.Encode(&pngData, page); err != nil {
		b.Fatal(err)
	}
	// Effort 1: the file is made once, and the decoder does the same work at any effort.
	if err := jxl.Encode(&jxlData, page, jxl.EncodeOptions{Quality: 85, Effort: 1}); err != nil {
		b.Fatal(err)
	}
	for _, format := range []struct {
		name string
		data []byte
	}{
		{name: "JPEG", data: benchmarkPage(b)},
		{name: "PNG", data: pngData.Bytes()},
		{name: "WebP lossless", data: encodeLosslessWebP(greenOf(page))},
		{name: "JPEG XL", data: jxlData.Bytes()},
	} {
		b.Run(format.name, func(b *testing.B) {
			b.SetBytes(int64(len(format.data)))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Decode(format.data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// encodeLosslessWebP's files decode to the grays encoded.
func TestEncodeLosslessWebP(t *testing.T) {
	random := rand.New(rand.NewPCG(3, 4))
	gray := image.NewGray(image.Rect(0, 0, 37, 23))
	for index := range gray.Pix {
		gray.Pix[index] = uint8(random.IntN(256))
	}
	decoded, err := DecodeGray(encodeLosslessWebP(gray))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.Pix, gray.Pix) || decoded.Rect != gray.Rect {
		t.Error("the decoded grays differ from those encoded")
	}
}

// greenOf returns the green channel of img, as a gray image: the page without its blue
// gradient, which encodeLosslessWebP cannot encode.
func greenOf(img *image.RGBA) *image.Gray {
	gray := image.NewGray(img.Rect)
	for index := range gray.Pix {
		gray.Pix[index] = img.Pix[index*4+1]
	}
	return gray
}

// bitWriter writes the bits of a WebP lossless stream, which fills each byte from its lowest
// bit.
type bitWriter struct {
	data    []byte
	pending uint64
	count   uint
}

// write writes the low width bits of value, its lowest bit first.
func (writer *bitWriter) write(value uint64, width uint) {
	writer.pending |= value << writer.count
	writer.count += width
	for writer.count >= 8 {
		writer.data = append(writer.data, byte(writer.pending))
		writer.pending >>= 8
		writer.count -= 8
	}
}

// bytes returns the stream, its last byte padded with zeros.
func (writer *bitWriter) bytes() []byte {
	if writer.count > 0 {
		writer.data = append(writer.data, byte(writer.pending))
		writer.pending, writer.count = 0, 0
	}
	return writer.data
}

// encodeLosslessWebP encodes gray as a WebP lossless (VP8L) file, the simplest one that is not
// trivial to decode: the standard library has no WebP encoder, and the decoder must read a
// prefix code per pixel as for a real page. Each gray is the green of a pixel coded on 8 bits;
// red and blue are 0 and the subtract-green transform gives them the green back; alpha and
// backward distances have one symbol each, which costs no bits. There is no LZ77 nor color
// cache, so the file is a byte a pixel, bigger than a real one.
func encodeLosslessWebP(gray *image.Gray) []byte {
	size := gray.Rect.Size()
	var writer bitWriter
	writer.write(0x2f, 8) // the VP8L signature
	writer.write(uint64(size.X-1), 14)
	writer.write(uint64(size.Y-1), 14)
	writer.write(0, 1) // no alpha
	writer.write(0, 3) // version 0
	writer.write(1, 1) // a transform:
	writer.write(2, 2) // subtract green
	writer.write(0, 1) // no other transform
	writer.write(0, 1) // no color cache
	writer.write(0, 1) // one prefix code group for the whole image
	writeEightBitGreenCode(&writer)
	writeSingleSymbolCode(&writer, 0)    // red
	writeSingleSymbolCode(&writer, 0)    // blue
	writeSingleSymbolCode(&writer, 0xff) // alpha
	writeSingleSymbolCode(&writer, 0)    // distance
	for row := range size.Y {
		for _, level := range gray.Pix[row*gray.Stride:][:size.X] {
			// A prefix code is read from its first bit, the stream from each byte's lowest
			// bit: the code is written reversed.
			writer.write(uint64(bits.Reverse8(level)), 8)
		}
	}
	stream := writer.bytes()
	streamLength := len(stream)
	if streamLength%2 == 1 {
		stream = append(stream, 0) // RIFF chunks have an even size
	}
	file := make([]byte, 0, 20+len(stream))
	file = append(file, "RIFF"...)
	file = binary.LittleEndian.AppendUint32(file, uint32(12+len(stream)))
	file = append(file, "WEBPVP8L"...)
	file = binary.LittleEndian.AppendUint32(file, uint32(streamLength))
	return append(file, stream...)
}

// writeSingleSymbolCode writes a simple prefix code of one 8-bit symbol, which takes no bits to
// read.
func writeSingleSymbolCode(writer *bitWriter, symbol uint64) {
	writer.write(1, 1) // a simple code
	writer.write(0, 1) // of one symbol
	writer.write(1, 1) // of 8 bits
	writer.write(symbol, 8)
}

// writeEightBitGreenCode writes the green code: the 256 literals 8 bits long, the 24 length
// prefixes unused. Its code lengths are themselves coded with two 1-bit codes: 0 for length
// 0 and 1 for length 8.
func writeEightBitGreenCode(writer *bitWriter) {
	// The code lengths of the code-length alphabet come in this order; length 8's is the 12th.
	const codeLengthCount = 12
	lengthZeroPosition, lengthEightPosition := 2, 11
	writer.write(0, 1) // a normal code
	writer.write(codeLengthCount-4, 4)
	for position := range codeLengthCount {
		length := uint64(0)
		if position == lengthZeroPosition || position == lengthEightPosition {
			length = 1
		}
		writer.write(length, 3)
	}
	writer.write(0, 1) // a length for every symbol of the alphabet
	const literals, lengthPrefixes = 256, 24
	for range literals {
		writer.write(1, 1)
	}
	for range lengthPrefixes {
		writer.write(0, 1)
	}
}
