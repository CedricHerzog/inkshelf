package layout

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/deadline"
	"github.com/CedricHerzog/inkshelf/internal/web/mathml"
	"github.com/CedricHerzog/inkshelf/internal/web/webimage"
)

// startPool returns a pool of workers workers, started as the app starts its own, stopped
// after the test once slow loaders let go.
func startPool(t *testing.T, workers int) *deadline.Pool {
	t.Helper()
	pool := deadline.NewPool(workers)
	ctx, cancel := context.WithCancel(context.Background())
	var group sync.WaitGroup
	for range workers {
		group.Go(func() { pool.Loop(ctx) })
	}
	t.Cleanup(func() {
		cancel()
		group.Wait()
	})
	return pool
}

// slowLoader returns a loader that takes delay for each image, then gives a black one of the
// size asked, unless the test ended first.
func slowLoader(t *testing.T, delay time.Duration) webimage.Loader {
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	return func(path string, size image.Point) (*image.Gray, error) {
		select {
		case <-time.After(delay):
		case <-done:
		}
		return image.NewGray(image.Rectangle{Max: size}), nil
	}
}

// altImage returns an image placed at rect whose text alternative is alt, in the test font.
func altImage(path string, rect image.Rectangle, alt string) PlacedImage {
	return PlacedImage{Path: path, Rect: rect, Alt: alt, altFormat: textFormat{size: testSettings.FontSize}}
}

// inkedPixels returns how many pixels of rect in canvas are darker than mid-gray.
func inkedPixels(canvas *image.Gray, rect image.Rectangle) int {
	inked := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if canvas.GrayAt(x, y).Y < 128 {
				inked++
			}
		}
	}
	return inked
}

// timedDraw draws page on a white canvas and returns the canvas and how long Draw took.
func timedDraw(t *testing.T, page Page, faces *Faces, images webimage.Loader, elements Elements) (*image.Gray, time.Duration) {
	t.Helper()
	canvas := whiteCanvas(testSettings)
	start := time.Now()
	if err := page.Draw(canvas, faces, images, elements); err != nil {
		t.Fatal(err)
	}
	return canvas, time.Since(start)
}

func TestDrawShowsAltTextOfSlowImage(t *testing.T) {
	faces := newTestFaces(t)
	elements := Elements{Pool: startPool(t, 2), Budget: 50 * time.Millisecond, Book: "slow"}
	box := image.Rect(50, 50, 450, 250)
	page := Page{Images: []PlacedImage{altImage("slow.png", box, "A lighthouse on a rock")}}
	images := slowLoader(t, 200*time.Millisecond)

	canvas, took := timedDraw(t, page, faces, images, elements)
	if took < 40*time.Millisecond || took > 150*time.Millisecond {
		t.Errorf("Draw took %v, want about the 50 ms budget", took)
	}
	inked := inkedPixels(canvas, box)
	if inked == 0 || inked > box.Dx()*box.Dy()/4 {
		t.Errorf("%d inked pixels in the image's box, want its text alternative", inked)
	}
	if outside := inkedPixels(canvas, canvas.Bounds()) - inked; outside != 0 {
		t.Errorf("%d inked pixels outside the image's box", outside)
	}

	// The image is given up: the next page that shows it does not wait for it.
	canvas, took = timedDraw(t, page, faces, images, elements)
	if took > 20*time.Millisecond {
		t.Errorf("drawing again took %v, want no wait", took)
	}
	if inkedPixels(canvas, box) != inked {
		t.Errorf("drawing again: %d inked pixels, want %d", inkedPixels(canvas, box), inked)
	}
}

// A budget started before the page was laid out, as the reader starts it, leaves the draw only
// what is left: a slow layout and a slow element together still show the page within it.
func TestDrawBudgetIncludesLayout(t *testing.T) {
	faces := newTestFaces(t)
	elements := Elements{Pool: startPool(t, 2), Budget: 100 * time.Millisecond, Book: "late"}.Started()
	time.Sleep(80 * time.Millisecond) // the page's layout
	box := image.Rect(50, 50, 450, 250)
	page := Page{Images: []PlacedImage{altImage("late.png", box, "A lighthouse on a rock")}}
	_, took := timedDraw(t, page, faces, slowLoader(t, 200*time.Millisecond), elements)
	if took > 60*time.Millisecond {
		t.Errorf("Draw took %v after 80 ms of layout, want what is left of the 100 ms budget", took)
	}
}

func TestDrawShowsAltTextOfFailedFormula(t *testing.T) {
	faces := newTestFaces(t)
	box := image.Rect(50, 50, 450, 100)
	broken := altImage(mathml.Prefix+"<math><mi>x</mi", box, "x squared")
	broken.MathSize = testSettings.FontSize
	page := Page{Images: []PlacedImage{broken}}

	canvas, _ := timedDraw(t, page, faces, nil, Elements{Pool: startPool(t, 1), Book: "formula"})
	if inkedPixels(canvas, box) == 0 {
		t.Error("no text alternative drawn for a formula that cannot be laid out")
	}
}

func TestDrawBudgetIsPerPage(t *testing.T) {
	faces := newTestFaces(t)
	elements := Elements{Pool: startPool(t, 2), Budget: 250 * time.Millisecond, Book: "three"}
	boxes := []image.Rectangle{image.Rect(20, 20, 180, 180), image.Rect(220, 20, 380, 180), image.Rect(420, 20, 580, 180)}
	var page Page
	for index, box := range boxes {
		page.Images = append(page.Images, altImage(string(rune('a'+index))+".png", box, "A figure"))
	}

	canvas, took := timedDraw(t, page, faces, slowLoader(t, 200*time.Millisecond), elements)
	if took < 240*time.Millisecond || took > 400*time.Millisecond {
		t.Errorf("Draw took %v, want about the page's 250 ms", took)
	}
	if inked := inkedPixels(canvas, boxes[0]); inked != boxes[0].Dx()*boxes[0].Dy() {
		t.Errorf("first image: %d inked pixels, want it drawn whole", inked)
	}
	for index, box := range boxes[1:] {
		if inked := inkedPixels(canvas, box); inked == 0 || inked > box.Dx()*box.Dy()/4 {
			t.Errorf("image %d: %d inked pixels, want its text alternative", index+2, inked)
		}
	}
}

func TestDrawWithoutPoolWaits(t *testing.T) {
	faces := newTestFaces(t)
	box := image.Rect(50, 50, 150, 150)
	page := Page{Images: []PlacedImage{altImage("slow.png", box, "never shown")}}

	canvas, _ := timedDraw(t, page, faces, slowLoader(t, 30*time.Millisecond), Elements{})
	if inked := inkedPixels(canvas, box); inked != box.Dx()*box.Dy() {
		t.Errorf("%d inked pixels, want the image drawn whole", inked)
	}
}

func TestWrapWords(t *testing.T) {
	widthOf := func(text string) int { return len(text) }
	got := wrapWords("a lighthouse on a rock", 10, widthOf)
	want := []string{"a", "lighthouse", "on a rock"}
	if len(got) != len(want) {
		t.Fatalf("wrapWords = %q, want %q", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("wrapWords = %q, want %q", got, want)
		}
	}
}
