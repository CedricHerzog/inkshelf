package openbook

import (
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// A book with spreads at pages 2 and 4 of 6 turns past both pages of a spread, and shows a
// position on a spread's second page as the spread. A book without spreads, as every book
// downloaded before them, turns one page at a time.
func TestSpreadPositions(t *testing.T) {
	spreads := shelf.Book{Pages: 6, Spreads: []int{2, 4}}
	for _, test := range []struct {
		book              shelf.Book
		page, shown, next int
		previous          int
	}{
		{book: spreads, page: 1, shown: 1, next: 2, previous: 0},
		{book: spreads, page: 2, shown: 2, next: 4, previous: 1},
		{book: spreads, page: 3, shown: 2, next: 4, previous: 1},
		{book: spreads, page: 4, shown: 4, next: 6, previous: 2},
		{book: spreads, page: 6, shown: 6, next: 7, previous: 4},
		{book: shelf.Book{Pages: 6}, page: 3, shown: 3, next: 4, previous: 2},
	} {
		shown := shownPage(test.book, test.page)
		if shown != test.shown || nextPosition(test.book, shown) != test.next || previousPosition(test.book, shown) != test.previous {
			t.Errorf("%v, page %d: shown %d, next %d, previous %d; want %d, %d, %d", test.book.Spreads, test.page,
				shown, nextPosition(test.book, shown), previousPosition(test.book, shown), test.shown, test.next, test.previous)
		}
	}
	if lastShown(spreads, 4) != 5 || lastShown(spreads, 6) != 6 {
		t.Error("the last page shown of a spread is its second")
	}
}
