package css

import (
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/colornames"
)

// Color is a color as the panel shows it: a gray (0 black, 255 white) and its opacity (0
// transparent, 255 opaque).
type Color struct {
	Gray, Alpha uint8
}

// Black is opaque black, the initial value of color.
var Black = Color{Gray: 0, Alpha: 255}

// ParseColor parses a color: a name, #rgb, #rgba, #rrggbb, #rrggbbaa, rgb(), rgba(), hsl(),
// hsla() (with commas or spaces and "/ alpha"), transparent, or currentColor, which is
// current. It returns false for what is not a color.
func ParseColor(value string, current Color) (Color, bool) {
	tokens := trimWhitespace(tokenize(value))
	if end := componentEnd(tokens, 0); len(tokens) == 0 || end != len(tokens) {
		return Color{}, false
	}
	return colorOf(tokens, current)
}

// colorOf returns the color of a component value: a name, a hash or a color function.
func colorOf(component []token, current Color) (Color, bool) {
	first := component[0]
	switch first.kind {
	case identToken:
		name := strings.ToLower(first.value)
		switch name {
		case "transparent":
			return Color{Gray: 255}, true
		case "currentcolor":
			return current, true
		}
		named, ok := colornames.Map[name]
		if !ok {
			return Color{}, false
		}
		return Color{Gray: grayOf(float64(named.R), float64(named.G), float64(named.B)), Alpha: 255}, true
	case hashToken:
		return hexColor(first.value)
	case functionToken:
		arguments := component[1:]
		if len(arguments) > 0 && arguments[len(arguments)-1].kind == closeParenToken {
			arguments = arguments[:len(arguments)-1]
		}
		switch strings.ToLower(first.value) {
		case "rgb", "rgba":
			return rgbColor(arguments)
		case "hsl", "hsla":
			return hslColor(arguments)
		}
	}
	return Color{}, false
}

// grayOf returns the gray of red, green and blue from 0 to 255, weighted as
// image/color's GrayModel weighs them.
func grayOf(red, green, blue float64) uint8 {
	return clampByte((19595*red + 38470*green + 7471*blue) / 65536)
}

// clampByte rounds value into 0 to 255.
func clampByte(value float64) uint8 {
	if math.IsNaN(value) {
		return 0
	}
	return uint8(math.Round(min(max(value, 0), 255)))
}

// hexColor returns the color of a hash's digits: 3, 4, 6 or 8 of them.
func hexColor(digits string) (Color, bool) {
	if len(digits) == 3 || len(digits) == 4 {
		var doubled strings.Builder
		for index := range len(digits) {
			doubled.WriteByte(digits[index])
			doubled.WriteByte(digits[index])
		}
		digits = doubled.String()
	}
	if len(digits) != 6 && len(digits) != 8 {
		return Color{}, false
	}
	number, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return Color{}, false
	}
	alpha := uint64(255)
	if len(digits) == 8 {
		alpha, number = number&0xff, number>>8
	}
	return Color{Gray: grayOf(float64(number>>16), float64(number>>8&0xff), float64(number&0xff)), Alpha: uint8(alpha)}, true
}

// colorArguments returns the 3 channels of a color function and its alpha (1 if not given),
// each a number or percentage token: "a, b, c[, alpha]" or "a b c[ / alpha]".
func colorArguments(arguments []token) (channels [3]token, alpha float64, ok bool) {
	var values []token
	slash := -1
	for _, part := range components(arguments) {
		switch {
		case len(part) != 1:
			return channels, 0, false
		case part[0].kind == commaToken:
		case isDelim(part[0], "/"):
			slash = len(values)
		case part[0].kind == numberToken, part[0].kind == percentageToken, part[0].kind == dimensionToken:
			values = append(values, part[0])
		default:
			return channels, 0, false
		}
	}
	if len(values) != 3 && len(values) != 4 || slash >= 0 && slash != 3 {
		return channels, 0, false
	}
	copy(channels[:], values)
	alpha = 1
	if len(values) == 4 {
		number, err := strconv.ParseFloat(values[3].value, 64)
		if err != nil {
			return channels, 0, false
		}
		if values[3].kind == percentageToken {
			number /= 100
		}
		alpha = number
	}
	return channels, alpha, true
}

// alphaByte returns an opacity from 0 to 1 as a byte.
func alphaByte(alpha float64) uint8 { return clampByte(alpha * 255) }

// rgbColor returns the color of rgb()'s arguments: numbers to 255 or percentages.
func rgbColor(arguments []token) (Color, bool) {
	channels, alpha, ok := colorArguments(arguments)
	if !ok {
		return Color{}, false
	}
	var values [3]float64
	for index, channel := range channels {
		number, err := strconv.ParseFloat(channel.value, 64)
		if err != nil || channel.kind == dimensionToken {
			return Color{}, false
		}
		if channel.kind == percentageToken {
			number *= 255.0 / 100
		}
		values[index] = min(max(number, 0), 255)
	}
	return Color{Gray: grayOf(values[0], values[1], values[2]), Alpha: alphaByte(alpha)}, true
}

// hueUnits are the units of a hue, in degrees.
var hueUnits = map[string]float64{"": 1, "deg": 1, "grad": 0.9, "rad": 180 / math.Pi, "turn": 360}

// hslColor returns the color of hsl()'s arguments: a hue, then saturation and lightness in
// percent.
func hslColor(arguments []token) (Color, bool) {
	channels, alpha, ok := colorArguments(arguments)
	if !ok || channels[0].kind == percentageToken {
		return Color{}, false
	}
	degrees, known := hueUnits[strings.ToLower(channels[0].unit)]
	hue, err := strconv.ParseFloat(channels[0].value, 64)
	if !known || err != nil {
		return Color{}, false
	}
	var fractions [2]float64 // saturation and lightness
	for index, channel := range channels[1:] {
		number, err := strconv.ParseFloat(channel.value, 64)
		if err != nil || channel.kind == dimensionToken {
			return Color{}, false
		}
		fractions[index] = min(max(number/100, 0), 1)
	}
	hue = math.Mod(hue*degrees, 360)
	if hue < 0 {
		hue += 360
	}
	if math.IsNaN(hue) || math.IsInf(hue, 0) {
		hue = 0
	}
	saturation, lightness := fractions[0], fractions[1]
	chroma := (1 - math.Abs(2*lightness-1)) * saturation
	second := chroma * (1 - math.Abs(math.Mod(hue/60, 2)-1))
	var red, green, blue float64
	switch sector := int(hue / 60); sector {
	case 0:
		red, green = chroma, second
	case 1:
		red, green = second, chroma
	case 2:
		green, blue = chroma, second
	case 3:
		green, blue = second, chroma
	case 4:
		red, blue = second, chroma
	default:
		red, blue = chroma, second
	}
	lowest := lightness - chroma/2
	return Color{Gray: grayOf((red+lowest)*255, (green+lowest)*255, (blue+lowest)*255), Alpha: alphaByte(alpha)}, true
}
