package pageimage

import (
	"image"
	"math/bits"
)

// fit scales src down to fit within width×height, keeping its aspect ratio. Each output pixel
// is the average of the box of source pixels it covers, which keeps thin lines visible. It
// also returns the histogram of the result: how many of its pixels have each gray.
//
// HOT PATH, run on every page of every download. It was tuned against the ARM assembly: its
// helpers are inlined or called once per source row, and its slicing lets the compiler drop
// most bounds checks. fitRef in the tests is the plain version it must match. Run
// BenchmarkFitDither and check the inlining with GOARCH=arm before and after changing it.
func fit(src *image.Gray, width, height int) (*image.Gray, *[256]int) {
	var histogram [256]int
	srcWidth, srcHeight := src.Rect.Dx(), src.Rect.Dy()
	if srcWidth == 0 || srcHeight == 0 {
		return src, &histogram
	}
	dstWidth, dstHeight := fitSize(srcWidth, srcHeight, width, height)
	dst := image.NewGray(image.Rect(0, 0, dstWidth, dstHeight))

	// The source columns of each output column, and the widest of them.
	columnBoxes := make([]span, dstWidth)
	maxBoxWidth := 0
	for column := range columnBoxes {
		columnBoxes[column] = boxSpan(column, srcWidth, dstWidth)
		maxBoxWidth = max(maxBoxWidth, columnBoxes[column].length())
	}
	reciprocals := reciprocalTable(maxBoxWidth * (srcHeight/dstHeight + 1))

	// sums[column] adds up the source pixels of the box of output pixel (column, row).
	sums := make([]uint32, dstWidth)
	for row := range dstHeight {
		rowBox := boxSpan(row, srcHeight, dstHeight)
		clear(sums)
		for boxRow := range rowBox.length() {
			start := (rowBox.start + boxRow) * src.Stride
			addBoxSums(sums, src.Pix[start:start+srcWidth], columnBoxes)
		}
		out := dst.Pix[row*dst.Stride : row*dst.Stride+dstWidth]
		for column, sum := range sums {
			area := rowBox.length() * columnBoxes[column].length()
			average := divide(sum, area, reciprocals)
			out[column] = uint8(average)
			// Indexing with uint8(average) would drop this bounds check, but measured 5%
			// slower on the host for no gain on ARM.
			histogram[average]++
		}
	}
	return dst, &histogram
}

// fitSize returns the size of a srcWidth×srcHeight image scaled to fit within width×height,
// keeping its aspect ratio, at least 1×1.
func fitSize(srcWidth, srcHeight, width, height int) (int, int) {
	dstWidth, dstHeight := width, srcHeight*width/srcWidth
	if dstHeight > height {
		dstWidth, dstHeight = srcWidth*height/srcHeight, height
	}
	return max(dstWidth, 1), max(dstHeight, 1)
}

// span is a range of source rows or columns, from start included to end excluded.
type span struct{ start, end int }

func (box span) length() int { return box.end - box.start }

// boxSpan returns the source rows (or columns) that output row (or column) index covers, when
// srcLength of them are scaled to dstLength: at least one.
func boxSpan(index, srcLength, dstLength int) span {
	start := index * srcLength / dstLength
	return span{start: start, end: max((index+1)*srcLength/dstLength, start+1)}
}

// addBoxSums adds the pixels of a source row to sums, those of columnBoxes[column] to
// sums[column].
func addBoxSums(sums []uint32, pixels []uint8, columnBoxes []span) {
	sums = sums[:len(columnBoxes)] // lets the compiler drop the bounds check of sums[column]
	for column, box := range columnBoxes {
		var sum uint32
		for _, value := range pixels[box.start:box.end] {
			sum += uint32(value)
		}
		sums[column] += sum
	}
}

// maxReciprocalArea is the largest box area that fit divides by through a table of
// reciprocals. Multiplying by a 32-bit reciprocal gives the exact quotient as long as
// 255·area² < 2³², that is for areas up to 4104.
const maxReciprocalArea = 4096

// reciprocalTable returns, for each area up to maxArea, 2³²/area rounded up, so that dividing
// by an area becomes a multiplication, much faster on ARM. It returns nil if maxArea is too
// large for that to be exact.
func reciprocalTable(maxArea int) []uint32 {
	if maxArea >= maxReciprocalArea {
		return nil
	}
	reciprocals := make([]uint32, maxArea+1)
	for area := range reciprocals {
		if area >= 2 { // areas 0 and 1 are never looked up
			reciprocals[area] = uint32((1<<32 + uint64(area) - 1) / uint64(area))
		}
	}
	return reciprocals
}

// divide returns sum/area, through reciprocals (see reciprocalTable) when it can.
func divide(sum uint32, area int, reciprocals []uint32) uint32 {
	switch {
	case area == 1:
		return sum
	case area < len(reciprocals):
		quotient, _ := bits.Mul32(sum, reciprocals[area]) // the high 32 bits: sum/area
		return quotient
	default:
		return sum / uint32(area)
	}
}
