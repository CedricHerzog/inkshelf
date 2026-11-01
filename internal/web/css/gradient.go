package css

import (
	"math"
	"strconv"
	"strings"
)

// maxGradientStops caps the color stops a gradient keeps: the rest are left out.
const maxGradientStops = 32

// Gradient is a linear-gradient() or a radial-gradient(), repeating or not. A linear one has
// its colors along a line through the middle of a box; a radial one along rays out of a centre,
// to an ending shape (a circle or an ellipse). A repeating one repeats its stops, from the first
// to the last, along the whole line or ray.
type Gradient struct {
	Radial, Repeating bool
	// Angle is the direction of a linear gradient's line in degrees, CSS's: 0 towards the top,
	// 90 towards the right. It is meaningful when ToCorner is false.
	Angle float64
	// ToCorner is set for "to top right" and the like, whose angle depends on the box's shape:
	// CornerX is -1 (left) or 1 (right), CornerY -1 (top) or 1 (bottom).
	ToCorner         bool
	CornerX, CornerY int
	// Circle is set for a radial gradient whose ending shape is a circle, else an ellipse. Its
	// size is Extent, a keyword (closest-side, closest-corner, farthest-side, farthest-corner),
	// or when Extent is "" the radii RadiusX and RadiusY (equal for a circle). Its centre is at
	// CenterX, CenterY from the box's top left, percentages being of the box's size.
	Circle           bool
	Extent           string
	RadiusX, RadiusY Length
	CenterX, CenterY Length
	// Stops are at least 2, in their written order.
	Stops []GradientStop
}

// GradientStop is a color stop of a gradient.
type GradientStop struct {
	Color Color
	// Position is where the stop is along the gradient line (or ray): a fraction, 0 at its
	// start and 1 at its end, or a length in CSS pixels when InPixels. It is NaN for a stop
	// written without one: StopsAlong places it.
	Position float64
	InPixels bool
}

// radialExtents are the size keywords of a radial gradient.
var radialExtents = map[string]bool{"closest-side": true, "closest-corner": true, "farthest-side": true, "farthest-corner": true}

// gradientFunctions are the gradient functions, by their lowercase name: whether each is
// radial, and whether it repeats.
var gradientFunctions = map[string][2]bool{
	"linear-gradient": {false, false}, "repeating-linear-gradient": {false, true},
	"radial-gradient": {true, false}, "repeating-radial-gradient": {true, true},
}

// ParseGradient returns the first gradient of a value, such as a background image's;
// currentColor in it is current. It returns false if value has none, or none that parses.
// Stop positions in lengths relative to the font (em…) are spaced out as if not given.
func ParseGradient(value string, current Color) (Gradient, bool) {
	tokens := tokenize(value)
	for index := 0; index < len(tokens); {
		end := componentEnd(tokens, index)
		if gradient, ok := gradientOf(tokens[index:end], current); ok {
			return gradient, true
		}
		index = end
	}
	return Gradient{}, false
}

// gradientOf parses a component value that is a gradient function.
func gradientOf(component []token, current Color) (Gradient, bool) {
	function := component[0]
	kind, isGradient := gradientFunctions[strings.ToLower(function.value)]
	if function.kind != functionToken || !isGradient {
		return Gradient{}, false
	}
	arguments := component[1:]
	if len(arguments) > 0 && arguments[len(arguments)-1].kind == closeParenToken {
		arguments = arguments[:len(arguments)-1]
	}
	gradient := Gradient{Radial: kind[0], Repeating: kind[1]}
	parts := splitAt(arguments, commaToken)
	if gradient.Radial {
		gradient.Extent, gradient.CenterX, gradient.CenterY = "farthest-corner", Length{50, "%"}, Length{50, "%"}
		if len(parts) > 0 && radialShape(&gradient, components(parts[0])) {
			parts = parts[1:]
		}
	} else {
		gradient.Angle = 180 // to bottom
		if len(parts) > 0 {
			if direction, ok := gradientDirection(components(parts[0])); ok {
				gradient.Angle, gradient.ToCorner, gradient.CornerX, gradient.CornerY = direction.Angle, direction.ToCorner, direction.CornerX, direction.CornerY
				parts = parts[1:]
			}
		}
	}
	for _, part := range parts {
		if len(gradient.Stops) >= maxGradientStops {
			break
		}
		values := components(part)
		if len(values) == 0 {
			return Gradient{}, false
		}
		color, ok := colorOf(values[0], current)
		if !ok {
			return Gradient{}, false
		}
		// A stop with two positions is two stops of the same color.
		if len(values) == 1 {
			gradient.Stops = append(gradient.Stops, GradientStop{Color: color, Position: math.NaN()})
		}
		for _, position := range values[1:min(len(values), 3)] {
			stop := stopPosition(position)
			stop.Color = color
			gradient.Stops = append(gradient.Stops, stop)
		}
	}
	if len(gradient.Stops) < 2 {
		return Gradient{}, false
	}
	gradient.Stops = gradient.Stops[:min(len(gradient.Stops), maxGradientStops)]
	return gradient, true
}

// stopPosition returns the position of a color stop: a percentage as a fraction, an absolute
// length in CSS pixels, NaN for anything else.
func stopPosition(position []token) GradientStop {
	length, ok := ParseLength(serialize(position))
	if !ok || math.IsInf(length.Value, 0) || math.IsNaN(length.Value) {
		return GradientStop{Position: math.NaN()}
	}
	if length.Unit == "%" {
		return GradientStop{Position: length.Value / 100}
	}
	if pixels, absolute := length.Pixels(); absolute {
		return GradientStop{Position: pixels, InPixels: true}
	}
	return GradientStop{Position: math.NaN()}
}

// StopsAlong returns the gradient's stops on a line (or ray) length CSS pixels long, each at
// a fraction of it, as CSS places them: the first at 0 and the last at 1 if not given, none
// before an earlier one, and those without a position evenly between their neighbours.
func (gradient *Gradient) StopsAlong(length float64) []GradientStop {
	stops := make([]GradientStop, len(gradient.Stops))
	positions := make([]float64, len(stops))
	for index, stop := range gradient.Stops {
		stops[index].Color, positions[index] = stop.Color, stop.Position
		if stop.InPixels {
			positions[index] = math.NaN()
			if length > 0 {
				positions[index] = stop.Position / length
			}
		}
	}
	last := len(positions) - 1
	if math.IsNaN(positions[0]) {
		positions[0] = 0
	}
	if math.IsNaN(positions[last]) {
		positions[last] = 1
	}
	highest := positions[0]
	for index, position := range positions {
		if !math.IsNaN(position) {
			highest = max(highest, position)
			positions[index] = highest
		}
	}
	for index := 0; index <= last; {
		if !math.IsNaN(positions[index]) {
			index++
			continue
		}
		end := index
		for math.IsNaN(positions[end]) {
			end++
		}
		start, step := positions[index-1], (positions[end]-positions[index-1])/float64(end-index+1)
		for between := index; between < end; between++ {
			positions[between] = start + step*float64(between-index+1)
		}
		index = end
	}
	for index := range stops {
		stops[index].Position = positions[index]
	}
	return stops
}

// sideAngles are the angles of "to <side>".
var sideAngles = map[string]float64{"top": 0, "right": 90, "bottom": 180, "left": 270}

// gradientDirection parses a linear gradient's first argument when it is a direction: an
// angle, or "to" and one or two sides.
func gradientDirection(values [][]token) (Gradient, bool) {
	if len(values) == 1 && len(values[0]) == 1 && values[0][0].kind == dimensionToken {
		degrees, known := hueUnits[strings.ToLower(values[0][0].unit)]
		number, err := strconv.ParseFloat(values[0][0].value, 64)
		if !known || err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
			return Gradient{}, false
		}
		return Gradient{Angle: math.Mod(number*degrees, 360)}, true
	}
	if len(values) < 2 || len(values) > 3 || !isIdent(values[0][0], "to") {
		return Gradient{}, false
	}
	var direction Gradient
	for _, side := range values[1:] {
		if len(side) != 1 || side[0].kind != identToken {
			return Gradient{}, false
		}
		switch strings.ToLower(side[0].value) {
		case "left":
			direction.CornerX = -1
		case "right":
			direction.CornerX = 1
		case "top":
			direction.CornerY = -1
		case "bottom":
			direction.CornerY = 1
		default:
			return Gradient{}, false
		}
	}
	if direction.CornerX != 0 && direction.CornerY != 0 {
		direction.ToCorner = len(values) == 3
		return direction, direction.ToCorner
	}
	if len(values) != 2 {
		return Gradient{}, false
	}
	return Gradient{Angle: sideAngles[strings.ToLower(values[1][0].value)]}, true
}

// radialShape parses a radial gradient's first argument into gradient when it gives its shape,
// size or centre: [circle | ellipse] [an extent keyword | one or two lengths] [at <position>],
// in either order before "at". It returns false, leaving gradient as it was, when it is not one.
func radialShape(gradient *Gradient, values [][]token) bool {
	shaped := *gradient
	var radii []Length
	shapeGiven := false
	for index, value := range values {
		keyword := ""
		if len(value) == 1 && value[0].kind == identToken {
			keyword = strings.ToLower(value[0].value)
		}
		switch {
		case keyword == "at":
			x, y, ok := parsePosition(values[index+1:])
			if !ok {
				return false
			}
			shaped.CenterX, shaped.CenterY = x, y
			return finishRadialShape(gradient, shaped, radii, shapeGiven)
		case keyword == "circle" || keyword == "ellipse":
			if shapeGiven {
				return false
			}
			shaped.Circle, shapeGiven = keyword == "circle", true
		case radialExtents[keyword]:
			shaped.Extent = keyword
		default:
			length, ok := ParseLength(serialize(value))
			if !ok || length.Value < 0 || len(radii) == 2 {
				return false
			}
			radii = append(radii, length)
		}
	}
	return finishRadialShape(gradient, shaped, radii, shapeGiven)
}

// finishRadialShape checks the radii given to a radial gradient's shape and, if they agree with
// it, sets them and the shape into gradient.
func finishRadialShape(gradient *Gradient, shaped Gradient, radii []Length, shapeGiven bool) bool {
	switch len(radii) {
	case 1: // a circle's radius: a length, not a percentage
		if shapeGiven && !shaped.Circle || radii[0].Unit == "%" {
			return false
		}
		shaped.Circle, shaped.Extent, shaped.RadiusX, shaped.RadiusY = true, "", radii[0], radii[0]
	case 2: // an ellipse's
		if shaped.Circle {
			return false
		}
		shaped.Extent, shaped.RadiusX, shaped.RadiusY = "", radii[0], radii[1]
	}
	*gradient = shaped
	return true
}

// positionKeywords are the keywords of a position, with the percentage each stands for and
// the axis it is on: 'x', 'y', or 0 for center, which is on either.
var positionKeywords = map[string]struct {
	percent float64
	axis    byte
}{"left": {0, 'x'}, "right": {100, 'x'}, "top": {0, 'y'}, "bottom": {100, 'y'}, "center": {50, 0}}

// parsePosition parses a position of one or two values (keywords, lengths, percentages), such
// as a background's or a radial gradient's centre, as the offsets of its x and y from the top
// left. A value missing is center. The forms of three and four values are not read.
func parsePosition(values [][]token) (x, y Length, ok bool) {
	if len(values) == 0 || len(values) > 2 {
		return Length{}, Length{}, false
	}
	type part struct {
		length Length
		axis   byte
	}
	parts := make([]part, len(values))
	for index, value := range values {
		text := serialize(value)
		if keyword, isKeyword := positionKeywords[strings.ToLower(text)]; isKeyword {
			parts[index] = part{Length{keyword.percent, "%"}, keyword.axis}
			continue
		}
		length, isLength := ParseLength(text)
		if !isLength {
			return Length{}, Length{}, false
		}
		parts[index] = part{length, 'l'} // a length: x first, then y
	}
	if len(parts) == 1 {
		parts = append(parts, part{Length{50, "%"}, 0})
	}
	first, second := parts[0], parts[1]
	if first.axis == 'y' || second.axis == 'x' {
		if first.axis == 'l' || second.axis == 'l' || first.axis == second.axis {
			return Length{}, Length{}, false // "top 10px" or "left right"
		}
		first, second = second, first
	}
	if first.axis == 'y' || second.axis == 'x' {
		return Length{}, Length{}, false
	}
	return first.length, second.length, true
}
