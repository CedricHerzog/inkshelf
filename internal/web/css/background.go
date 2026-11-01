package css

import (
	"strings"
)

// maxBackgroundLayers caps the layers of a background that are kept: the rest are left out.
const maxBackgroundLayers = 8

// BackgroundLayer is a layer of a box's background: an image or a gradient, sized, placed
// and repeated. Layers are listed top first, as CSS writes them.
type BackgroundLayer struct {
	// Image is the address of a url() image, "" when the layer is Gradient.
	Image    string
	Gradient *Gradient
	Size     BackgroundSize
	// X and Y place the image (one tile of it): a percentage puts that point of the image on
	// the same point of the box, a length is the offset of its top left from the box's.
	X, Y             Length
	RepeatX, RepeatY Repeat
}

// BackgroundSize is the size of a background image: Cover or Contain the box, else Width and
// Height, either of which may be auto (its Unit "auto"): from the image's own size, or its
// aspect ratio.
type BackgroundSize struct {
	Cover, Contain bool
	Width, Height  Length
}

// Auto is the length auto, as BackgroundSize gives it.
var Auto = Length{Unit: "auto"}

// IsAuto reports whether length is auto.
func (length Length) IsAuto() bool { return length.Unit == "auto" }

// Repeat is how a background image repeats along an axis.
type Repeat uint8

// The ways a background repeats: tiled from where it is placed, once, tiled with the space
// left spread between whole tiles, or tiled with the tiles stretched to fit a whole number.
const (
	Repeated Repeat = iota
	NoRepeat
	Spaced
	Rounded
)

// repeats are the keywords of background-repeat for one axis.
var repeats = map[string]Repeat{"repeat": Repeated, "no-repeat": NoRepeat, "space": Spaced, "round": Rounded}

// backgroundBoxes are the keywords of background-origin, -clip and -attachment, which the
// shorthand may hold and which are not followed: a background fills the border box.
var backgroundBoxes = map[string]bool{
	"border-box": true, "padding-box": true, "content-box": true, "text": true, "scroll": true, "fixed": true, "local": true,
}

// backgroundLonghands expands background into background-color and, for each of its layers
// (separated by commas), background-image, -repeat, -position and -size. What a layer does
// not give is reset. Only the last layer may have a color.
func backgroundLonghands(value []token) map[string]string {
	layers := splitAt(value, commaToken)
	images, repeatValues, positions, sizes := make([]string, len(layers)), make([]string, len(layers)), make([]string, len(layers)), make([]string, len(layers))
	color := "transparent"
	for index, layer := range layers {
		images[index], repeatValues[index], positions[index], sizes[index] = "none", "repeat", "0% 0%", "auto"
		var repeatWords, positionWords []string
		inSize, sizeDone, sizeWords := false, false, []string(nil)
		for _, part := range components(layer) {
			text := serialize(part)
			keyword := strings.ToLower(text)
			_, isPositionKeyword := positionKeywords[keyword]
			_, isLength := ParseLength(text)
			_, isRepeat := repeats[keyword]
			isFunction := part[0].kind == functionToken
			_, isGradient := gradientFunctions[strings.ToLower(part[0].value)]
			isSize := isLength || keyword == "auto" || keyword == "cover" || keyword == "contain"
			// The size is the one or two values right after the "/".
			sizeDone = sizeDone || len(sizeWords) == 2 || len(sizeWords) > 0 && !isSize
			switch {
			case isDelim(part[0], "/"):
				if len(positionWords) == 0 || inSize {
					return nil
				}
				inSize = true
			case inSize && !sizeDone && isSize:
				sizeWords = append(sizeWords, text)
			case inSize && len(sizeWords) == 0:
				return nil
			case keyword == "none" || part[0].kind == urlToken || isFunction && (strings.EqualFold(part[0].value, "url") || isGradient):
				images[index] = text
			case keyword == "repeat-x" || keyword == "repeat-y" || isRepeat:
				repeatWords = append(repeatWords, text)
			case (isPositionKeyword || isLength) && !inSize:
				positionWords = append(positionWords, text)
			case backgroundBoxes[keyword]:
			default:
				if _, isColor := colorOf(part, Black); !isColor || index != len(layers)-1 {
					return nil
				}
				color = text
			}
		}
		if len(repeatWords) > 0 {
			repeatValues[index] = strings.Join(repeatWords, " ")
		}
		if len(positionWords) > 0 {
			positions[index] = strings.Join(positionWords, " ")
		}
		if inSize {
			if len(sizeWords) == 0 {
				return nil
			}
			sizes[index] = strings.Join(sizeWords, " ")
		}
	}
	return map[string]string{
		"background-color": color, "background-image": strings.Join(images, ", "),
		"background-repeat": strings.Join(repeatValues, ", "), "background-position": strings.Join(positions, ", "),
		"background-size": strings.Join(sizes, ", "),
	}
}

// Backgrounds returns the layers of a background from the values declared for an element:
// background-image, and background-repeat, -position and -size, whose lists repeat to give each
// image one. currentColor in a gradient is current. Layers whose image is none, or doesn't
// parse, are left out; past maxBackgroundLayers, the bottom ones are.
func Backgrounds(declared map[string]string, current Color) []BackgroundLayer {
	images := splitAt(tokenize(declared["background-image"]), commaToken)
	repeatValues := splitAt(tokenize(declared["background-repeat"]), commaToken)
	positions := splitAt(tokenize(declared["background-position"]), commaToken)
	sizes := splitAt(tokenize(declared["background-size"]), commaToken)
	var layers []BackgroundLayer
	for index, image := range images {
		if len(layers) == maxBackgroundLayers {
			break
		}
		layer := BackgroundLayer{X: Length{0, "%"}, Y: Length{0, "%"}, Size: BackgroundSize{Width: Auto, Height: Auto}}
		parts := components(image)
		if len(parts) != 1 {
			continue
		}
		if gradient, ok := gradientOf(parts[0], current); ok {
			layer.Gradient = &gradient
		} else if address, ok := urlOf(parts[0]); ok && address != "" {
			layer.Image = address
		} else {
			continue
		}
		layer.RepeatX, layer.RepeatY = parseRepeat(components(repeatValues[index%len(repeatValues)]))
		if x, y, ok := parsePosition(components(positions[index%len(positions)])); ok {
			layer.X, layer.Y = x, y
		}
		layer.Size = parseBackgroundSize(components(sizes[index%len(sizes)]))
		layers = append(layers, layer)
	}
	return layers
}

// parseRepeat parses a layer's background-repeat: repeat-x, repeat-y, or a keyword for both
// axes or one for each. What doesn't parse repeats.
func parseRepeat(values [][]token) (x, y Repeat) {
	keywords := make([]string, len(values))
	for index, value := range values {
		keywords[index] = strings.ToLower(serialize(value))
	}
	switch {
	case len(keywords) == 1 && keywords[0] == "repeat-x":
		return Repeated, NoRepeat
	case len(keywords) == 1 && keywords[0] == "repeat-y":
		return NoRepeat, Repeated
	case len(keywords) == 1:
		return repeats[keywords[0]], repeats[keywords[0]]
	case len(keywords) == 2:
		return repeats[keywords[0]], repeats[keywords[1]]
	}
	return Repeated, Repeated
}

// parseBackgroundSize parses a layer's background-size: cover, contain, or one or two widths
// (lengths, percentages or auto), the second auto when not given. What doesn't parse is auto.
func parseBackgroundSize(values [][]token) BackgroundSize {
	size := BackgroundSize{Width: Auto, Height: Auto}
	if len(values) == 1 {
		switch strings.ToLower(serialize(values[0])) {
		case "cover":
			return BackgroundSize{Cover: true}
		case "contain":
			return BackgroundSize{Contain: true}
		}
	}
	if len(values) == 0 || len(values) > 2 {
		return size
	}
	lengths := [2]Length{Auto, Auto}
	for index, value := range values {
		text := serialize(value)
		if strings.EqualFold(text, "auto") {
			continue
		}
		length, ok := ParseLength(text)
		if !ok || length.Value < 0 {
			return size
		}
		lengths[index] = length
	}
	return BackgroundSize{Width: lengths[0], Height: lengths[1]}
}
