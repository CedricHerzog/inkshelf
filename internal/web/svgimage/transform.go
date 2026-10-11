package svgimage

import (
	"math"
	"strconv"
	"strings"
)

// matrix is an affine transform: a point (x, y) goes to (a×x + c×y + e, b×x + d×y + f).
type matrix struct{ a, b, c, d, e, f float64 }

var identity = matrix{a: 1, d: 1}

// multiply returns the transform that applies next, then mapping: mapping × next.
func (mapping matrix) multiply(next matrix) matrix {
	return matrix{
		a: mapping.a*next.a + mapping.c*next.b, b: mapping.b*next.a + mapping.d*next.b,
		c: mapping.a*next.c + mapping.c*next.d, d: mapping.b*next.c + mapping.d*next.d,
		e: mapping.a*next.e + mapping.c*next.f + mapping.e, f: mapping.b*next.e + mapping.d*next.f + mapping.f,
	}
}

// apply returns the point x, y transformed.
func (mapping matrix) apply(x, y float64) (float64, float64) {
	return mapping.a*x + mapping.c*y + mapping.e, mapping.b*x + mapping.d*y + mapping.f
}

// inverse returns the transform that undoes mapping, false if it flattens the plane.
func (mapping matrix) inverse() (matrix, bool) {
	determinant := mapping.a*mapping.d - mapping.b*mapping.c
	if determinant == 0 || math.IsNaN(determinant) || math.IsInf(determinant, 0) {
		return identity, false
	}
	inverted := matrix{
		a: mapping.d / determinant, b: -mapping.b / determinant,
		c: -mapping.c / determinant, d: mapping.a / determinant,
	}
	inverted.e = -(inverted.a*mapping.e + inverted.c*mapping.f)
	inverted.f = -(inverted.b*mapping.e + inverted.d*mapping.f)
	return inverted, true
}

// scale returns how much the transform enlarges lengths, on average over its two axes.
func (mapping matrix) scale() float64 {
	return math.Sqrt(math.Abs(mapping.a*mapping.d - mapping.b*mapping.c))
}

// String returns the transform as a transform attribute that oksvg reads.
func (mapping matrix) String() string {
	numbers := []float64{mapping.a, mapping.b, mapping.c, mapping.d, mapping.e, mapping.f}
	written := make([]string, len(numbers))
	for index, number := range numbers {
		written[index] = formatNumber(number)
	}
	return "matrix(" + strings.Join(written, " ") + ")"
}

// matrix returns the transform as a matrix.
func (mapping transform) matrix() matrix {
	return matrix{a: mapping.scaleX, d: mapping.scaleY, e: mapping.offsetX, f: mapping.offsetY}
}

// parseTransform parses a transform attribute: matrix(), translate(), scale(), rotate(),
// skewX() and skewY(), in any number. It returns false if it cannot be parsed: then no
// transform applies.
func parseTransform(value string) (matrix, bool) {
	result := identity
	rest := value
	for {
		rest = strings.TrimLeft(rest, " \t\r\n,")
		if rest == "" {
			return result, true
		}
		open := strings.IndexByte(rest, '(')
		closing := strings.IndexByte(rest, ')')
		if open < 0 || closing < open {
			return identity, false
		}
		name := strings.TrimSpace(rest[:open])
		numbers, ok := parseNumbers(rest[open+1 : closing])
		if !ok {
			return identity, false
		}
		next, ok := transformFunction(name, numbers)
		if !ok {
			return identity, false
		}
		result = result.multiply(next)
		rest = rest[closing+1:]
	}
}

// transformFunction returns the matrix of one transform function.
func transformFunction(name string, numbers []float64) (matrix, bool) {
	count := len(numbers)
	radians := func(degrees float64) float64 { return degrees * math.Pi / 180 }
	switch name {
	case "matrix":
		if count == 6 {
			return matrix{a: numbers[0], b: numbers[1], c: numbers[2], d: numbers[3], e: numbers[4], f: numbers[5]}, true
		}
	case "translate":
		if count == 1 || count == 2 {
			moved := matrix{a: 1, d: 1, e: numbers[0]}
			if count == 2 {
				moved.f = numbers[1]
			}
			return moved, true
		}
	case "scale":
		if count == 1 || count == 2 {
			scaled := matrix{a: numbers[0], d: numbers[0]}
			if count == 2 {
				scaled.d = numbers[1]
			}
			return scaled, true
		}
	case "rotate":
		if count == 1 || count == 3 {
			sine, cosine := math.Sincos(radians(numbers[0]))
			rotation := matrix{a: cosine, b: sine, c: -sine, d: cosine}
			if count == 3 {
				centreX, centreY := numbers[1], numbers[2]
				rotation = matrix{a: 1, d: 1, e: centreX, f: centreY}.multiply(rotation).multiply(matrix{a: 1, d: 1, e: -centreX, f: -centreY})
			}
			return rotation, true
		}
	case "skewX":
		if count == 1 {
			return matrix{a: 1, c: math.Tan(radians(numbers[0])), d: 1}, true
		}
	case "skewY":
		if count == 1 {
			return matrix{a: 1, b: math.Tan(radians(numbers[0])), d: 1}, true
		}
	}
	return identity, false
}

// parseNumbers parses a list of numbers, separated by whitespace, commas, or nothing when the
// next starts with a sign or a second point ("1-2.5.5" is 1, -2.5, .5).
func parseNumbers(text string) ([]float64, bool) {
	var numbers []float64
	for index := 0; index < len(text); {
		character := text[index]
		if character == ' ' || character == ',' || character == '\t' || character == '\n' || character == '\r' {
			index++
			continue
		}
		end := numberEnd(text, index)
		if end == index {
			return nil, false
		}
		number, err := strconv.ParseFloat(text[index:end], 64)
		if err != nil || math.IsInf(number, 0) {
			return nil, false
		}
		numbers = append(numbers, number)
		index = end
	}
	return numbers, true
}

// numberEnd returns the end of the number that starts at start in text, start if there is
// none.
func numberEnd(text string, start int) int {
	index := start
	if index < len(text) && (text[index] == '+' || text[index] == '-') {
		index++
	}
	digits := 0
	for ; index < len(text) && text[index] >= '0' && text[index] <= '9'; index++ {
		digits++
	}
	if index < len(text) && text[index] == '.' {
		index++
		for ; index < len(text) && text[index] >= '0' && text[index] <= '9'; index++ {
			digits++
		}
	}
	if digits == 0 {
		return start
	}
	if index < len(text) && (text[index] == 'e' || text[index] == 'E') {
		exponent := index + 1
		if exponent < len(text) && (text[exponent] == '+' || text[exponent] == '-') {
			exponent++
		}
		if exponent < len(text) && text[exponent] >= '0' && text[exponent] <= '9' {
			index = exponent
			for index < len(text) && text[index] >= '0' && text[index] <= '9' {
				index++
			}
		}
	}
	return index
}
