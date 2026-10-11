package servers

import "testing"

func TestSeriesStatus(t *testing.T) {
	tests := []struct {
		name                    string
		books, read, inProgress int
		want                    ReadStatus
	}{
		{name: "empty series", books: 0, read: 0, inProgress: 0, want: Unread},
		{name: "nothing started", books: 3, read: 0, inProgress: 0, want: Unread},
		{name: "one book started", books: 3, read: 0, inProgress: 1, want: Reading},
		{name: "one book read", books: 3, read: 1, inProgress: 0, want: Reading},
		{name: "all books started or read", books: 3, read: 2, inProgress: 1, want: Reading},
		{name: "all books read", books: 3, read: 3, inProgress: 0, want: Read},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SeriesStatus(test.books, test.read, test.inProgress); got != test.want {
				t.Errorf("SeriesStatus(%d, %d, %d) = %d, want %d", test.books, test.read, test.inProgress, got, test.want)
			}
		})
	}
}
