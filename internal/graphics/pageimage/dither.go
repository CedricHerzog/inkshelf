package pageimage

import "image"

// Dithering lets 16 grays look like many more: where a pixel must be rounded to a panel gray,
// the difference (its error) is added to its neighbours not yet drawn, so that on average the
// area keeps its true gray. Floyd–Steinberg dithering spreads each error in sixteenths, to the
// right and to the three pixels below.
const (
	errorToRight      = 7 // the next pixel of the row
	errorToBelowLeft  = 3 // the pixels of the next row, below left,
	errorToBelow      = 5 // below,
	errorToBelowRight = 1 // and below right
	errorDivisor      = 16
)

// ditherAndPack maps each pixel of gray through tones, then to the nearest of the panel's 16
// grays with Floyd–Steinberg dithering, and returns the result as a .pg4 page. It does what
// draw.FloydSteinberg does, much faster, as it only handles gray.
//
// HOT PATH, run on every page of every download. It was tuned against the ARM assembly: its
// helpers are inlined, and the three-pixel windows of errors let the compiler drop most bounds
// checks. Run BenchmarkFitDither and check the inlining with GOARCH=arm before and after
// changing it.
func ditherAndPack(gray *image.Gray, tones *[256]uint8) []byte {
	width, height := gray.Rect.Dx(), gray.Rect.Dy()
	out, rowBytes := newPG4(width, height)
	// The errors carried to the row being drawn and to the next one, times errorDivisor. They
	// have a pixel of margin on each side, so pixel column is at index column+1.
	errorsThisRow, errorsNextRow := make([]int, width+2), make([]int, width+2)
	for row := range height {
		src := gray.Pix[row*gray.Stride : row*gray.Stride+width]
		dst := out[pg4Header+row*rowBytes : pg4Header+(row+1)*rowBytes]
		for column, level := range src {
			// The errors of the pixel and of its neighbours, columns column-1 to column+1: the
			// pixel's own at here[1].
			here := errorsThisRow[column : column+3 : column+3]
			below := errorsNextRow[column : column+3 : column+3]
			value := int(tones[level]) + here[1]/errorDivisor
			index := nearestPanelGray(value)
			if column%2 == 0 { // the left pixel of a byte: its high nibble
				dst[column/2] = uint8(index) << 4
			} else {
				dst[column/2] |= uint8(index)
			}
			spreadError(here, below, value-index*grayStep)
		}
		errorsThisRow, errorsNextRow = errorsNextRow, errorsThisRow
		clear(errorsNextRow)
	}
	return out
}

// nearestPanelGray returns the index of the panel gray nearest to value, a gray that the
// errors carried may have pushed below 0 or above 255.
func nearestPanelGray(value int) int {
	return min(max((value+grayStep/2)/grayStep, 0), panelGrays-1)
}

// spreadError spreads the error of a pixel over the pixels not yet drawn around it: here and
// below are the errors of the pixel's row and of the next one, columns column-1 to column+1.
func spreadError(here, below []int, pixelError int) {
	here[2] += pixelError * errorToRight
	below[0] += pixelError * errorToBelowLeft
	below[1] += pixelError * errorToBelow
	below[2] += pixelError * errorToBelowRight
}
