package pageimage

import (
	"iter"
	"math"
)

const (
	// clippedShare is the share of the darkest pixels, and of the lightest, that the tone table
	// turns fully black or fully white. Without it, a single black speck on a yellowed scan
	// would keep the paper from turning white.
	clippedShare = 0.005
	// gamma darkens the mid grays, which e-ink renders lighter than LCD screens do: a gray
	// at half intensity comes out at 0.5^1.8, about 29%.
	gamma = 1.8
)

// toneTable returns the tone table of an image whose histogram is given: for each gray of the
// image, the gray to show. It stretches the image's grays so that its darkest pixels become
// black and its lightest white (scans are often gray on gray), then applies gamma. A table
// of 256 entries makes the per-pixel work of the next stage a single lookup.
func toneTable(histogram *[256]int) *[256]uint8 {
	pixelCount := 0
	for _, count := range histogram {
		pixelCount += count
	}
	clip := int(float64(pixelCount) * clippedShare)
	darkest := clipLevel(histogram, clip, darkToLight)
	lightest := clipLevel(histogram, clip, lightToDark)
	if lightest <= darkest { // a blank page, or one of a single gray: nothing to stretch
		darkest, lightest = 0, 255
	}
	var tones [256]uint8
	for level := range tones {
		normalized := min(max(float64(level-darkest)/float64(lightest-darkest), 0), 1)
		tones[level] = uint8(math.Round(255 * math.Pow(normalized, gamma)))
	}
	return &tones
}

// clipLevel walks the grays in order, adding up how many pixels have each, and returns the
// first gray at which more than clip pixels have been counted. The pixels of the grays before
// it are the ones clipped. If the image has no more than clip pixels, it returns the last gray.
func clipLevel(histogram *[256]int, clip int, order iter.Seq[int]) int {
	counted, last := 0, 0
	for level := range order {
		counted += histogram[level]
		if counted > clip {
			return level
		}
		last = level
	}
	return last
}

// darkToLight yields the grays from black (0) to white (255).
func darkToLight(yield func(int) bool) {
	for level := range 256 {
		if !yield(level) {
			return
		}
	}
}

// lightToDark yields the grays from white (255) to black (0).
func lightToDark(yield func(int) bool) {
	for index := range 256 {
		if !yield(255 - index) {
			return
		}
	}
}
