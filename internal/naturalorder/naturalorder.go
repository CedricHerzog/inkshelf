// Package naturalorder sorts titles as people do: "tome 2" before "tome 10".
package naturalorder

import (
	"cmp"
	"strings"
)

// Compare compares two strings as people sort titles: their runs of digits compare as
// numbers, so that "Vol. 2" comes before "Vol. 10". The rest compares byte by byte. It returns
// a negative number if left comes first, a positive one if right does, and 0 if they are equal.
func Compare(left, right string) int {
	for left != "" && right != "" {
		leftDigits, rightDigits := digitPrefix(left), digitPrefix(right)
		if leftDigits == "" || rightDigits == "" {
			// At least one of them is not at a number: compare a single byte.
			if left[0] != right[0] {
				return cmp.Compare(left[0], right[0])
			}
			left, right = left[1:], right[1:]
			continue
		}
		// Both are at a number. Without its leading zeros, the longer number is the larger,
		// and numbers of the same length compare as text.
		leftNumber, rightNumber := strings.TrimLeft(leftDigits, "0"), strings.TrimLeft(rightDigits, "0")
		if len(leftNumber) != len(rightNumber) {
			return cmp.Compare(len(leftNumber), len(rightNumber))
		}
		if leftNumber != rightNumber {
			return strings.Compare(leftNumber, rightNumber)
		}
		left, right = left[len(leftDigits):], right[len(rightDigits):]
	}
	// One is a prefix of the other: the shorter comes first.
	return cmp.Compare(len(left), len(right))
}

// digitPrefix returns the digits that text starts with.
func digitPrefix(text string) string {
	end := strings.IndexFunc(text, func(char rune) bool { return char < '0' || char > '9' })
	if end < 0 { // all digits
		return text
	}
	return text[:end]
}
