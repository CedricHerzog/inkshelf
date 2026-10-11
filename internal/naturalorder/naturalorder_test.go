package naturalorder

import "testing"

func TestNaturalCompare(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{left: "Vol. 2", right: "Vol. 10", want: -1},
		{left: "Vol. 10", right: "Vol. 2", want: 1},
		{left: "Vol. 02", right: "Vol. 2", want: 0}, // leading zeros don't count
		{left: "Vol. 2", right: "Vol. 2", want: 0},
		{left: "Vol. 2", right: "Vol. 2 extra", want: -1},
		{left: "Alpha", right: "Beta", want: -1},
		{left: "10", right: "9", want: 1},
		{left: "a9", right: "b1", want: -1},
	}
	for _, test := range tests {
		t.Run(test.left+" vs "+test.right, func(t *testing.T) {
			if got := Compare(test.left, test.right); got != test.want {
				t.Errorf("Compare = %d, want %d", got, test.want)
			}
		})
	}
}
