package pocketbook

import (
	"image"
	"iter"
	"unsafe"
)

// The framebuffer is the portrait screen turned a quarter (1872x1404 on an InkPad 3, which the
// driver calls rotate 2): screen pixel (x, y) is framebuffer column y, row
// framebufferHeight-1-x. So a row of the canvas becomes a column of the framebuffer, and
// copying means reading down one and writing along the other.
//
// HOT PATH. The copy below runs on every page turn, and its speed shows: about 22 ms a page
// turn on an InkPad 3, against 48 ms byte by byte. Its shape was tuned against the ARM
// assembly and BenchmarkCopyRotated: the helpers it calls are inlined (interleaveBytes,
// interleaveHalves, the tiles iterator) or called once per tile, never once per block.
// Before changing it, run the benchmark, and check the inlining (go build -gcflags=-m) and
// the assembly (-gcflags=-S) with GOOS=linux GOARCH=arm; then compare after.

// framebufferRect returns the framebuffer rectangle that holds screen rectangle rect.
func framebufferRect(rect image.Rectangle, framebufferHeight int) image.Rectangle {
	return image.Rect(rect.Min.Y, framebufferHeight-rect.Max.X, rect.Max.Y, framebufferHeight-rect.Min.X)
}

// rotateTile is the side of the square tiles that copyRotated works in (see tiles).
const rotateTile = 32

// copyRotated copies rect of canvas into the framebuffer memory framebuffer, which has
// framebufferHeight rows of framebufferStride bytes, turning it into the framebuffer's
// orientation. Most of rect goes in 4x4 blocks moved as whole aligned words: fb0 is slow memory
// (about 19 ns a byte when written byte by byte on an InkPad 3), and Go on 32-bit ARM never
// merges byte loads or stores into words.
func copyRotated(framebuffer []byte, framebufferStride, framebufferHeight int, canvas *image.Gray, rect image.Rectangle) {
	rect = rect.Intersect(canvas.Rect)
	blocks := wordBlocks(framebuffer, framebufferStride, framebufferHeight, canvas, rect)
	if blocks.Empty() {
		copyRotatedBytes(framebuffer, framebufferStride, framebufferHeight, canvas, rect)
		return
	}
	copyRotatedWords(framebuffer, framebufferStride, framebufferHeight, canvas, blocks)
	// The edges of rect around the blocks: above, below, left and right.
	for _, edge := range []image.Rectangle{
		image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, blocks.Min.Y),
		image.Rect(rect.Min.X, blocks.Max.Y, rect.Max.X, rect.Max.Y),
		image.Rect(rect.Min.X, blocks.Min.Y, blocks.Min.X, blocks.Max.Y),
		image.Rect(blocks.Max.X, blocks.Min.Y, rect.Max.X, blocks.Max.Y),
	} {
		copyRotatedBytes(framebuffer, framebufferStride, framebufferHeight, canvas, edge)
	}
}

// wordBlocks returns the part of rect that copyRotatedWords can copy: whole 4x4 blocks whose
// canvas rows and framebuffer rows start on word boundaries. It is empty if the memory is not
// laid out for it, or if framebuffer is too small to hold rect: copyRotatedBytes then reports
// the error, by panicking on an index out of range.
func wordBlocks(framebuffer []byte, framebufferStride, framebufferHeight int, canvas *image.Gray, rect image.Rectangle) image.Rectangle {
	if rect.Empty() || framebufferStride%4 != 0 || canvas.Stride%4 != 0 || len(canvas.Pix) == 0 ||
		uintptr(unsafe.Pointer(unsafe.SliceData(framebuffer)))%4 != 0 ||
		rect.Min.X < 0 || rect.Max.X > framebufferHeight || rect.Min.Y < 0 || rect.Max.Y > framebufferStride ||
		framebufferStride*framebufferHeight > len(framebuffer) ||
		canvas.PixOffset(rect.Max.X-1, rect.Max.Y-1) >= len(canvas.Pix) {
		return image.Rectangle{}
	}
	// Framebuffer rows start on words when column y is a multiple of 4. Canvas pixel x starts
	// a word when its address, counted from Pix's, is a multiple of 4.
	canvasPhase := int(uintptr(unsafe.Pointer(unsafe.SliceData(canvas.Pix)))%4) - canvas.Rect.Min.X
	firstX := rect.Min.X + modulo4(-(rect.Min.X + canvasPhase))
	lastX := rect.Max.X - modulo4(rect.Max.X+canvasPhase)
	firstY, lastY := rect.Min.Y+modulo4(-rect.Min.Y), rect.Max.Y-modulo4(rect.Max.Y)
	if firstX >= lastX || firstY >= lastY {
		return image.Rectangle{}
	}
	return image.Rect(firstX, firstY, lastX, lastY)
}

// modulo4 returns value modulo 4, between 0 and 3 even for a negative value.
func modulo4(value int) int { return (value%4 + 4) % 4 }

// copyRotatedWords copies blocks (see wordBlocks) of canvas into framebuffer, a tile at a time.
func copyRotatedWords(framebuffer []byte, framebufferStride, framebufferHeight int, canvas *image.Gray, blocks image.Rectangle) {
	for tile := range tiles(blocks) {
		copyTileWords(framebuffer, framebufferStride, framebufferHeight, canvas, tile)
	}
}

// copyTileWords copies tile, made of whole 4x4 blocks (see wordBlocks), a block at a time: it
// reads the block's four rows as four words down the canvas, transposes them into its four
// columns, then writes each column as a word into its framebuffer row. It assumes a
// little-endian CPU, as ARM and the hosts the tests run on are.
func copyTileWords(framebuffer []byte, framebufferStride, framebufferHeight int, canvas *image.Gray, tile image.Rectangle) {
	canvasPixels := unsafe.Pointer(unsafe.SliceData(canvas.Pix))
	framebufferPixels := unsafe.Pointer(unsafe.SliceData(framebuffer))
	canvasStride := canvas.Stride
	for blockColumn := range tile.Dx() / 4 {
		x := tile.Min.X + 4*blockColumn
		// source walks down canvas columns x to x+3, and destination along their framebuffer
		// rows (canvas column x+k is framebuffer row framebufferHeight-1-x-k), a block at a time.
		source := unsafe.Add(canvasPixels, canvas.PixOffset(x, tile.Min.Y))
		destination := unsafe.Add(framebufferPixels, (framebufferHeight-1-x)*framebufferStride+tile.Min.Y)
		for range tile.Dy() / 4 {
			// The block's four rows, the first pixel in the low byte.
			row0 := *(*uint32)(source)
			row1 := *(*uint32)(unsafe.Add(source, canvasStride))
			row2 := *(*uint32)(unsafe.Add(source, 2*canvasStride))
			row3 := *(*uint32)(unsafe.Add(source, 3*canvasStride))
			// The transpose, in two steps (see interleaveBytes and interleaveHalves).
			even01, odd01 := interleaveBytes(row0, row1)
			even23, odd23 := interleaveBytes(row2, row3)
			column0, column2 := interleaveHalves(even01, even23)
			column1, column3 := interleaveHalves(odd01, odd23)
			// The block's four columns, each into its framebuffer row.
			*(*uint32)(destination) = column0
			*(*uint32)(unsafe.Add(destination, -framebufferStride)) = column1
			*(*uint32)(unsafe.Add(destination, -2*framebufferStride)) = column2
			*(*uint32)(unsafe.Add(destination, -3*framebufferStride)) = column3
			source = unsafe.Add(source, 4*canvasStride)
			destination = unsafe.Add(destination, 4)
		}
	}
}

// The masks that interleaveBytes and interleaveHalves pick bytes with, numbering the bytes of
// a word from its low one, 0, to its high one, 3.
const (
	bytes0and2 = 0x00ff00ff
	bytes1and3 = 0xff00ff00
	bytes0and1 = 0x0000ffff
	bytes2and3 = 0xffff0000
)

// A 4x4 block of pixels held in four words, one per row, is transposed into four words, one
// per column, in two steps. Written as letters, the first pixel of each row in the low byte:
//
//	rows     a b c d   e f g h   i j k l   m n o p
//	step 1   a e c g   b f d h   i m k o   j n l p   (interleaveBytes of rows 0 and 1, 2 and 3)
//	step 2   a e i m   b f j n   c g k o   d h l p   (interleaveHalves of a e c g and i m k o, ...)
//
// Each step is a few shifts, masks and ORs, and ARM folds the shifts into the other instructions.

// interleaveBytes interleaves the bytes of two rows: from a b c d and e f g h it returns
// a e c g (the even bytes) and b f d h (the odd ones).
func interleaveBytes(first, second uint32) (even, odd uint32) {
	return first&bytes0and2 | second<<8&bytes1and3, first>>8&bytes0and2 | second&bytes1and3
}

// interleaveHalves interleaves the halves of two words: from a e c g and i m k o it returns
// a e i m (the low halves) and c g k o (the high ones).
func interleaveHalves(first, second uint32) (low, high uint32) {
	return first&bytes0and1 | second<<16, first>>16 | second&bytes2and3
}

// copyRotatedBytes does what copyRotated does a byte at a time, for any rectangle of the
// canvas.
func copyRotatedBytes(framebuffer []byte, framebufferStride, framebufferHeight int, canvas *image.Gray, rect image.Rectangle) {
	for tile := range tiles(rect) {
		copyTileBytes(framebuffer, framebufferStride, framebufferHeight, canvas, tile)
	}
}

// copyTileBytes copies tile a byte at a time: each canvas column, read down, becomes part of a
// framebuffer row.
func copyTileBytes(framebuffer []byte, framebufferStride, framebufferHeight int, canvas *image.Gray, tile image.Rectangle) {
	for column := range tile.Dx() {
		x := tile.Min.X + column
		rowStart := (framebufferHeight - 1 - x) * framebufferStride
		destination := framebuffer[rowStart+tile.Min.Y : rowStart+tile.Max.Y]
		// offset walks down canvas column x, one row at a time.
		offset := canvas.PixOffset(x, tile.Min.Y)
		for index := range destination {
			destination[index] = canvas.Pix[offset]
			offset += canvas.Stride
		}
	}
}

// tiles yields the rotateTile×rotateTile squares that cover rect, row after row, those along
// its right and bottom edges cut to fit. Working a tile at a time keeps both the canvas rows
// read (down a column) and the framebuffer rows written in the cache.
func tiles(rect image.Rectangle) iter.Seq[image.Rectangle] {
	return func(yield func(image.Rectangle) bool) {
		for tileRow := range tileCount(rect.Dy()) {
			for tileColumn := range tileCount(rect.Dx()) {
				corner := rect.Min.Add(image.Pt(tileColumn, tileRow).Mul(rotateTile))
				tile := image.Rectangle{Min: corner, Max: corner.Add(image.Pt(rotateTile, rotateTile))}
				if !yield(tile.Intersect(rect)) {
					return
				}
			}
		}
	}
}

// tileCount returns how many tiles cover a length of pixels, the last one maybe partly.
func tileCount(length int) int {
	return (length + rotateTile - 1) / rotateTile
}
