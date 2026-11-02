package layout

import (
	"fmt"
	"hash/fnv"
	"image"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/deadline"
)

// PageBudget is how long drawing a page waits in all for its slow elements (images, SVGs,
// formulas). A crafted or merely huge element must not keep the reader on a blank page: past it,
// every element not ready yet shows its text alternative.
const PageBudget = 10 * time.Second

// Elements bounds how long drawing a page waits for its elements: each is prepared on a worker
// of Pool, and the page waits for them Budget in all. An element given up is not prepared again
// until the pool is made again, so the next page that shows it does not wait for it either.
type Elements struct {
	// Pool prepares the elements; nil prepares them on the goroutine that draws, with no limit.
	Pool *deadline.Pool
	// Budget is how long the page waits in all, PageBudget if 0.
	Budget time.Duration
	// Book names the book in the keys a pool remembers given-up elements by.
	Book string
	// end is when the page's budget runs out, set by Started.
	end time.Time
}

// Started returns elements whose page budget starts now, unless it started already: the parts
// of a page drawn one after the other (the regions of a fixed-layout page) share one budget.
func (elements Elements) Started() Elements {
	if elements.end.IsZero() {
		budget := elements.Budget
		if budget <= 0 {
			budget = PageBudget
		}
		elements.end = time.Now().Add(budget)
	}
	return elements
}

// PrepareElement runs work, which prepares the element at path (an image, an SVG, a formula)
// for a box of size, through elements, in a span called name. It waits no longer than what is
// left of the page's budget, started now if it was not. ok is false when the element was not
// ready in time, or was given up before, or panicked: the caller then shows its fallback.
func PrepareElement[Result any](elements Elements, name, path string, size image.Point, work func() Result) (result Result, ok bool) {
	if elements.Pool == nil {
		return work(), true
	}
	elements = elements.Started()
	return deadline.Run(elements.Pool, name, elements.key(path, size), time.Until(elements.end), work)
}

// key names an element of the book: its path, which may be a long data: URI, hashed, and the size
// it is prepared at.
func (elements Elements) key(path string, size image.Point) string {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(path))
	return fmt.Sprintf("%s %016x %dx%d", elements.Book, hash.Sum64(), size.X, size.Y)
}
