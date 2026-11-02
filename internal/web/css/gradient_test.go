package css

import (
	"math"
	"reflect"
	"testing"
)

func TestParseGradient(t *testing.T) {
	white, black := Color{255, 255}, Color{0, 255}
	for _, test := range []struct {
		value     string
		want      Gradient
		ok        bool
		positions []float64 // along a line 100 CSS pixels long
	}{
		{"linear-gradient(white, black)", Gradient{Angle: 180}, true, []float64{0, 1}},
		{"url(a.png) linear-gradient(to right, white 10%, black)", Gradient{Angle: 90}, true, []float64{0.1, 1}},
		{"linear-gradient(45deg, white, gray, black)", Gradient{Angle: 45}, true, []float64{0, 0.5, 1}},
		{"linear-gradient(0.25turn, white 50%, black 20%)", Gradient{Angle: 90}, true, []float64{0.5, 0.5}},
		{"linear-gradient(to top left, white, black 0% 50%, white)", Gradient{ToCorner: true, CornerX: -1, CornerY: -1}, true, []float64{0, 0, 0.5, 1}},
		{"linear-gradient(white 1em, black 0, white)", Gradient{Angle: 180}, true, []float64{0, 0, 1}},
		{"linear-gradient(currentColor 1px, transparent 1px)", Gradient{Angle: 180}, true, []float64{0.01, 0.01}},
		{"repeating-linear-gradient(90deg, white, white 10px, black 10px, black 20px)", Gradient{Angle: 90, Repeating: true}, true, []float64{0, 0.1, 0.1, 0.2}},
		{"radial-gradient(white, black)", Gradient{Radial: true, Extent: "farthest-corner", CenterX: Length{50, "%"}, CenterY: Length{50, "%"}}, true, []float64{0, 1}},
		{"radial-gradient(circle closest-side at left 30%, white, black 50%)",
			Gradient{Radial: true, Circle: true, Extent: "closest-side", CenterX: Length{0, "%"}, CenterY: Length{30, "%"}}, true, []float64{0, 0.5}},
		{"radial-gradient(20px at top, white, black)",
			Gradient{Radial: true, Circle: true, RadiusX: Length{20, "px"}, RadiusY: Length{20, "px"}, CenterX: Length{50, "%"}, CenterY: Length{0, "%"}}, true, nil},
		{"repeating-radial-gradient(ellipse 10% 20px, white, black 5px)",
			Gradient{Radial: true, Repeating: true, RadiusX: Length{10, "%"}, RadiusY: Length{20, "px"}, CenterX: Length{50, "%"}, CenterY: Length{50, "%"}}, true, []float64{0, 0.05}},
		{"radial-gradient(circle 10% 20%, red, blue)", Gradient{}, false, nil},
		{"radial-gradient(circle 10%, red, blue)", Gradient{}, false, nil},
		{"radial-gradient(at, red, blue)", Gradient{}, false, nil},
		{"linear-gradient(red)", Gradient{}, false, nil},
		{"linear-gradient(to middle, red, blue)", Gradient{}, false, nil},
		{"linear-gradient(red, nocolor)", Gradient{}, false, nil},
		{"conic-gradient(red, blue)", Gradient{}, false, nil},
		{"none", Gradient{}, false, nil},
	} {
		got, ok := ParseGradient(test.value, Black)
		if ok != test.ok {
			t.Errorf("ParseGradient(%q) ok = %v, want %v", test.value, ok, test.ok)
			continue
		}
		if !ok {
			continue
		}
		got.Stops, test.want.Stops = nil, nil
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("ParseGradient(%q) = %+v, want %+v", test.value, got, test.want)
		}
		if test.positions == nil {
			continue
		}
		parsed, _ := ParseGradient(test.value, Black)
		stops := parsed.StopsAlong(100)
		if len(stops) != len(test.positions) {
			t.Errorf("ParseGradient(%q) has %d stops, want %d", test.value, len(stops), len(test.positions))
			continue
		}
		for index, position := range test.positions {
			if math.Abs(stops[index].Position-position) > 1e-9 {
				t.Errorf("ParseGradient(%q) stop %d at %v, want %v", test.value, index, stops[index].Position, position)
			}
		}
	}
	if gradient, _ := ParseGradient("linear-gradient(white, black)", Black); gradient.Stops[0].Color != white || gradient.Stops[1].Color != black {
		t.Errorf("stops' colors = %v", gradient.Stops)
	}
}

func TestGradientStopsCapped(t *testing.T) {
	value := "repeating-radial-gradient(red"
	for range 100 {
		value += ", blue"
	}
	gradient, ok := ParseGradient(value+")", Black)
	if !ok || len(gradient.Stops) != maxGradientStops {
		t.Errorf("%d stops, %v, want %d", len(gradient.Stops), ok, maxGradientStops)
	}
}

func TestParsePosition(t *testing.T) {
	percent := func(value float64) Length { return Length{value, "%"} }
	for _, test := range []struct {
		value string
		x, y  Length
		ok    bool
	}{
		{"center", percent(50), percent(50), true},
		{"top", percent(50), percent(0), true},
		{"right bottom", percent(100), percent(100), true},
		{"bottom left", percent(0), percent(100), true},
		{"10px 20%", Length{10, "px"}, percent(20), true},
		{"10px top", Length{10, "px"}, percent(0), true},
		{"top 10px", Length{}, Length{}, false},
		{"left right", Length{}, Length{}, false},
		{"right 10px bottom 20px", Length{}, Length{}, false},
	} {
		x, y, ok := parsePosition(components(tokenize(test.value)))
		if x != test.x || y != test.y || ok != test.ok {
			t.Errorf("parsePosition(%q) = %v, %v, %v, want %v, %v, %v", test.value, x, y, ok, test.x, test.y, test.ok)
		}
	}
}
