package widgets

import (
	"fmt"
	"strings"
	"unicode"
)

// Ellipsize shortens label to at most maxWidth pixels in font, ending it with "…".
func Ellipsize(font Font, label string, maxWidth int) string {
	if font.Width(label) <= maxWidth {
		return label
	}
	runes := []rune(label)
	// A binary search for the longest prefix that fits with "…": runes[:fits] is known to fit,
	// and no prefix longer than runes[:mostThatMayFit] does.
	fits, mostThatMayFit := 0, len(runes)
	for fits < mostThatMayFit {
		middle := (fits + mostThatMayFit + 1) / 2
		if font.Width(string(runes[:middle])+"…") <= maxWidth {
			fits = middle
		} else {
			mostThatMayFit = middle - 1
		}
	}
	return string(runes[:fits]) + "…"
}

// WrapWords splits message into lines of at most maxWidth pixels at its spaces. A word too long
// for a line is broken between two characters.
func WrapWords(font Font, message string, maxWidth int) []string {
	var lines []string
	line := ""
	for word := range strings.FieldsSeq(message) {
		next := word
		if line != "" {
			next = line + " " + word
		}
		if font.Width(next) <= maxWidth {
			line = next
			continue
		}
		if line != "" {
			lines = append(lines, line)
		}
		parts := wrapChars(font, word, maxWidth)
		lines = append(lines, parts[:len(parts)-1]...)
		line = parts[len(parts)-1]
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// wrapChars splits word into lines of at most maxWidth pixels, between any two characters.
func wrapChars(font Font, word string, maxWidth int) []string {
	var lines []string
	var line []rune
	lineWidth := 0
	for _, char := range word {
		charWidth := font.Width(string(char))
		if lineWidth+charWidth > maxWidth && len(line) > 0 {
			lines = append(lines, string(line))
			line, lineWidth = nil, 0
		}
		line = append(line, char)
		lineWidth += charWidth
	}
	if len(line) > 0 {
		lines = append(lines, string(line))
	}
	return lines
}

// Clean replaces the control characters of label, which the fonts cannot draw, with U+FFFD.
// Titles and error messages come from servers and files, so they may hold any character.
func Clean(label string) string {
	return strings.Map(func(char rune) rune {
		if unicode.IsControl(char) {
			return '�'
		}
		return char
	}, label)
}

// FormatBytes writes a size in MB, or in GB from 1 GB, with a decimal under 10 GB.
func FormatBytes(size int64) string {
	const megabyte, gigabyte = 1 << 20, 1 << 30
	if size >= 10*gigabyte {
		return fmt.Sprintf("%d GB", (size+gigabyte/2)/gigabyte)
	}
	if size >= gigabyte {
		return fmt.Sprintf("%.1f GB", float64(size)/gigabyte)
	}
	return fmt.Sprintf("%d MB", (size+megabyte/2)/megabyte)
}

// Counted writes count and the word for it: "1 recipe", "2 recipes".
func Counted(count int, one, many string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, one)
	}
	return fmt.Sprintf("%d %s", count, many)
}
