package pageimage

import (
	"context"
	"fmt"
	"image"
	"sync/atomic"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// Prepare turns a page image (JPEG, PNG, WebP or JPEG XL) into a .pg4 page that fits a
// width×height screen: it decodes the image to gray, scales it down, adjusts its tones, then
// dithers it to the panel's grays. If times is not nil, Prepare adds the time spent in each
// stage to it.
func Prepare(data []byte, width, height int, times *Times) ([]byte, error) {
	return prepare(data, width, height, DecodeGray, times)
}

// prepare is Prepare with the decoder as a parameter, for the tests.
func prepare(data []byte, width, height int, decode func(data []byte) (*image.Gray, error), times *Times) ([]byte, error) {
	if times == nil {
		times = &Times{}
	}
	// The stages are spans too, for the trace: around the hot paths, never inside them.
	start := time.Now()
	span := timing.Start(context.Background(), "decode")
	src, err := decode(data)
	span.End()
	if err != nil {
		return nil, err
	}
	start = addTimeSince(&times.Decode, start)
	span = timing.Start(context.Background(), "fit")
	gray, histogram := fit(src, width, height)
	tones := toneTable(histogram)
	span.End()
	start = addTimeSince(&times.Scale, start)
	span = timing.Start(context.Background(), "dither")
	packed := ditherAndPack(gray, tones)
	span.End()
	addTimeSince(&times.Dither, start)
	return packed, nil
}

// PrepareDecoded is Prepare for an image already decoded, such as a drawn SVG: it scales it
// to fit width×height, adjusts its tones, then dithers it to the panel's grays.
func PrepareDecoded(img image.Image, width, height int) []byte {
	gray, histogram := fit(toGray(img), width, height)
	return ditherAndPack(gray, toneTable(histogram))
}

// Fit scales a gray image to fit within width×height, keeping its aspect ratio, as Prepare
// does. It is for a page drawn from several images: each is fitted, and the page is prepared
// once composed.
func Fit(src *image.Gray, width, height int) *image.Gray {
	fitted, _ := fit(src, width, height)
	return fitted
}

// Times adds up, across goroutines, the time Prepare spends in each stage, in nanoseconds.
type Times struct {
	Decode atomic.Int64 // decoding the image to gray
	Scale  atomic.Int64 // fitting it to the screen and building its tone table
	Dither atomic.Int64 // dithering it and packing it as a .pg4 page
}

// addTimeSince adds the time elapsed since start to total and returns the current time, the
// start of the next stage.
func addTimeSince(total *atomic.Int64, start time.Time) time.Time {
	now := time.Now()
	total.Add(int64(now.Sub(start)))
	return now
}

// PerPage formats the average time of each stage over pageCount pages, for the log.
func (times *Times) PerPage(pageCount int) string {
	millisecondsPerPage := func(total *atomic.Int64) int64 {
		return total.Load() / int64(time.Millisecond) / int64(pageCount)
	}
	return fmt.Sprintf("decode %d ms, scale %d ms, dither %d ms",
		millisecondsPerPage(&times.Decode), millisecondsPerPage(&times.Scale), millisecondsPerPage(&times.Dither))
}
