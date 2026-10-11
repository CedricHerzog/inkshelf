package fixedlayout

import (
	"cmp"
	"image"
	"math"
	"strconv"
	"strings"
)

// ParseViewport reads the size of a page from the content of its viewport meta, such as
// "width=1200, height=1800", leniently as EPUB asks: properties apart by commas or
// semicolons, spaces around "=", and each value the number it starts with ("1200px" is
// 1200). The first width and height win. It returns false unless both are positive.
func ParseViewport(content string) (image.Point, bool) {
	var size image.Point
	separators := func(character rune) bool { return character == ',' || character == ';' }
	for _, property := range strings.FieldsFunc(content, separators) {
		name, value, found := strings.Cut(property, "=")
		if !found {
			continue
		}
		number := leadingNumber(strings.TrimSpace(value))
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "width":
			size.X = cmp.Or(size.X, number)
		case "height":
			size.Y = cmp.Or(size.Y, number)
		}
	}
	return size, size.X > 0 && size.Y > 0
}

// leadingNumber returns the number value starts with, rounded; 0 if it starts with none.
func leadingNumber(value string) int {
	end := 0
	for end < len(value) && (value[end] >= '0' && value[end] <= '9' || value[end] == '.') {
		end++
	}
	number, err := strconv.ParseFloat(value[:end], 64)
	if err != nil {
		return 0
	}
	return int(math.Round(number))
}
