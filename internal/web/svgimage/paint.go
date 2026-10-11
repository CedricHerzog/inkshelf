package svgimage

import (
	"math"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// paint is a fill or a stroke as oksvg reads it, and the opacity it takes from the color.
type paint struct {
	value   string // "none", a #rrggbb, or the url(#id) of a gradient
	opacity float64
}

// paintOf returns a fill or stroke as oksvg reads it: a color as a gray, with its opacity; a
// gradient's url; a pattern's as its dominant color; a missing server's as its fallback, else
// none. currentColor is color.
func (work *styler) paintOf(value string, color css.Color) (paint, bool) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "none") {
		return paint{value: "none", opacity: 1}, true
	}
	if address, ok := css.URL(value); ok {
		if id, local := strings.CutPrefix(address, "#"); local {
			if server, found := work.ids[id]; found {
				switch server.local {
				case "lineargradient", "radialgradient":
					return paint{value: "url(#" + id + ")", opacity: 1}, true
				case "pattern":
					return work.patternPaint(server), true
				}
			}
		}
		// The fallback color after the url(), else none.
		if end := strings.LastIndexByte(value, ')'); end >= 0 {
			if fallback, ok := work.paintOf(value[end+1:], color); ok && !strings.HasPrefix(fallback.value, "url(") {
				return fallback, true
			}
		}
		return paint{value: "none", opacity: 1}, true
	}
	parsed, ok := css.ParseColor(value, color)
	if !ok {
		return paint{}, false
	}
	return paint{value: hexOf(parsed.Gray), opacity: float64(parsed.Alpha) / 255}, true
}

// patternPaint returns what a pattern paints with, as one color: the fill of the shape that
// covers the most of its tile, a gradient's first stop. A pattern without shapes takes those
// of the pattern its href names.
func (work *styler) patternPaint(pattern *node) paint {
	id := pattern.ID()
	if found, ok := work.patternColors[id]; ok {
		return found
	}
	if work.resolving[id] {
		return paint{value: "none", opacity: 1}
	}
	work.resolving[id] = true
	defer delete(work.resolving, id)
	best, bestArea := paint{value: "none", opacity: 1}, -1.0
	var visit func(element *node)
	visit = func(element *node) {
		if element.local == "" || strings.EqualFold(element.values["display"], "none") {
			return
		}
		if area, shape := shapeArea(element); shape && area > bestArea {
			if painted, ok := work.paintOf(element.inherited.fill, element.inherited.color); ok && painted.value != "none" {
				if gradient, isGradient := strings.CutPrefix(painted.value, "url(#"); isGradient {
					painted = work.gradientPaint(strings.TrimSuffix(gradient, ")"))
				}
				painted.opacity *= element.inherited.fillOpacity
				best, bestArea = painted, area
			}
		}
		for _, child := range element.children {
			visit(child)
		}
	}
	for _, child := range pattern.children {
		visit(child)
	}
	if bestArea < 0 {
		if href, ok := pattern.Attribute("href"); ok {
			if other, found := work.ids[strings.TrimPrefix(href, "#")]; found && other.local == "pattern" {
				best = work.patternPaint(other)
			}
		}
	}
	work.patternColors[id] = best
	return best
}

// gradientPaint returns the color of the first stop of the gradient id.
func (work *styler) gradientPaint(id string) paint {
	if gradient, ok := work.ids[id]; ok {
		for _, stop := range gradient.children {
			if stop.local != "stop" {
				continue
			}
			color, ok := css.ParseColor(stop.values["stop-color"], stop.inherited.color)
			if !ok {
				color = css.Black
			}
			return paint{value: hexOf(color.Gray), opacity: float64(color.Alpha) / 255}
		}
	}
	return paint{value: "#000000", opacity: 1}
}

// shapeArea returns the area of a shape, roughly, and whether element is a shape: a path or a
// polygon counts as no area, to be chosen only if nothing has one.
func shapeArea(element *node) (float64, bool) {
	number := func(name string) float64 {
		value, _ := element.Attribute(name)
		length, _ := lengthOf(value, defaultFontSize)
		return math.Abs(length)
	}
	switch element.local {
	case "rect":
		return number("width") * number("height"), true
	case "circle":
		return math.Pi * number("r") * number("r"), true
	case "ellipse":
		return math.Pi * number("rx") * number("ry"), true
	case "path", "polygon", "polyline":
		return 0, true
	}
	return 0, false
}
