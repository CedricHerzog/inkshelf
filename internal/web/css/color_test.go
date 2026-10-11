package css

import (
	"math"
	"testing"
)

func TestParseColor(t *testing.T) {
	current := Color{Gray: 77, Alpha: 255}
	for _, test := range []struct {
		value string
		want  Color
		ok    bool
	}{
		{"black", Color{0, 255}, true},
		{"White", Color{255, 255}, true},
		{"red", Color{76, 255}, true},
		{"#fff", Color{255, 255}, true},
		{"#000c", Color{0, 204}, true},
		{"#ff0000", Color{76, 255}, true},
		{"#00000080", Color{0, 128}, true},
		{"rgb(255, 0, 0)", Color{76, 255}, true},
		{"rgb(100%, 100%, 100%)", Color{255, 255}, true},
		{"rgba(0,0,0,0.5)", Color{0, 128}, true},
		{"rgb(0 0 0 / 25%)", Color{0, 64}, true},
		{"rgb(300, 300, 300)", Color{255, 255}, true},
		{"hsl(0, 0%, 50%)", Color{128, 255}, true},
		{"hsl(120deg 100% 50%)", Color{150, 255}, true},
		{"hsla(0.5turn, 100%, 50%, 0.5)", Color{179, 128}, true},
		{"transparent", Color{255, 0}, true},
		{"currentColor", current, true},
		{"  #123456  ", Color{grayOf(0x12, 0x34, 0x56), 255}, true},
		{"", Color{}, false},
		{"nocolor", Color{}, false},
		{"#12345", Color{}, false},
		{"#ggg", Color{}, false},
		{"rgb(1, 2)", Color{}, false},
		{"rgb(1px, 2, 3)", Color{}, false},
		{"hsl(10%, 2%, 3%)", Color{}, false},
		{"red blue", Color{}, false},
		{"url(#p)", Color{}, false},
	} {
		got, ok := ParseColor(test.value, current)
		if got != test.want || ok != test.ok {
			t.Errorf("ParseColor(%q) = %v, %v, want %v, %v", test.value, got, ok, test.want, test.ok)
		}
	}
}

func TestParseLinearGradient(t *testing.T) {
	white, black := Color{255, 255}, Color{0, 255}
	for _, test := range []struct {
		value     string
		want      Gradient
		ok        bool
		positions []float64
	}{
		{"linear-gradient(white, black)", Gradient{Angle: 180, Stops: []GradientStop{{white, 0}, {black, 1}}}, true, nil},
		{"url(a.png) linear-gradient(to right, white 10%, black)", Gradient{Angle: 90, Stops: []GradientStop{{white, 0.1}, {black, 1}}}, true, nil},
		{"linear-gradient(45deg, white, gray, black)", Gradient{Angle: 45}, true, []float64{0, 0.5, 1}},
		{"linear-gradient(0.25turn, white 50%, black 20%)", Gradient{Angle: 90}, true, []float64{0.5, 0.5}},
		{"linear-gradient(to top left, white, black 0% 50%, white)", Gradient{ToCorner: true, CornerX: -1, CornerY: -1}, true, []float64{0, 0, 0.5, 1}},
		{"linear-gradient(white 10px, black 0, white)", Gradient{Angle: 180}, true, []float64{0, 0, 1}},
		{"linear-gradient(red)", Gradient{}, false, nil},
		{"linear-gradient(to middle, red, blue)", Gradient{}, false, nil},
		{"linear-gradient(red, nocolor)", Gradient{}, false, nil},
		{"radial-gradient(red, blue)", Gradient{}, false, nil},
		{"none", Gradient{}, false, nil},
	} {
		got, ok := ParseLinearGradient(test.value, Black)
		if ok != test.ok {
			t.Errorf("ParseLinearGradient(%q) ok = %v, want %v", test.value, ok, test.ok)
			continue
		}
		if !ok {
			continue
		}
		if got.Angle != test.want.Angle || got.ToCorner != test.want.ToCorner || got.CornerX != test.want.CornerX || got.CornerY != test.want.CornerY {
			t.Errorf("ParseLinearGradient(%q) direction = %+v, want %+v", test.value, got, test.want)
		}
		if test.want.Stops != nil && !equalStops(got.Stops, test.want.Stops) {
			t.Errorf("ParseLinearGradient(%q) stops = %v, want %v", test.value, got.Stops, test.want.Stops)
		}
		if test.positions != nil {
			if len(got.Stops) != len(test.positions) {
				t.Errorf("ParseLinearGradient(%q) has %d stops, want %d", test.value, len(got.Stops), len(test.positions))
				continue
			}
			for index, position := range test.positions {
				if math.Abs(got.Stops[index].Position-position) > 1e-9 {
					t.Errorf("ParseLinearGradient(%q) stop %d at %v, want %v", test.value, index, got.Stops[index].Position, position)
				}
			}
		}
	}
}

func equalStops(got, want []GradientStop) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index].Color != want[index].Color || math.Abs(got[index].Position-want[index].Position) > 1e-9 {
			return false
		}
	}
	return true
}

func TestGradientStopsCapped(t *testing.T) {
	value := "linear-gradient(red"
	for range 100 {
		value += ", blue"
	}
	gradient, ok := ParseLinearGradient(value+")", Black)
	if !ok || len(gradient.Stops) != maxGradientStops {
		t.Errorf("%d stops, %v, want %d", len(gradient.Stops), ok, maxGradientStops)
	}
}

func TestBackgroundColor(t *testing.T) {
	for _, test := range []struct {
		value string
		want  Color
		ok    bool
	}{
		{"#fff url(a.png) no-repeat", Color{255, 255}, true},
		{"url(a.png) center / cover rgba(0, 0, 0, 0.5)", Color{0, 128}, true},
		{"none", Color{}, false},
		{"linear-gradient(red, blue)", Color{}, false},
	} {
		got, ok := BackgroundColor(test.value, Black)
		if got != test.want || ok != test.ok {
			t.Errorf("BackgroundColor(%q) = %v, %v, want %v, %v", test.value, got, ok, test.want, test.ok)
		}
	}
}

func FuzzParseColor(f *testing.F) {
	for _, seed := range []string{"red", "#abc", "#aabbccdd", "rgb(1 2 3 / 50%)", "hsla(1rad, 2%, 3%, .5)",
		"linear-gradient(to top left, red 10% 20%, blue)", "linear-gradient(45deg, #000, #fff 3px)", "url(x) #fff"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		ParseColor(value, Black)
		BackgroundColor(value, Black)
		gradient, ok := ParseLinearGradient(value, Black)
		if !ok {
			return
		}
		if len(gradient.Stops) < 2 || len(gradient.Stops) > maxGradientStops {
			t.Fatalf("%d stops", len(gradient.Stops))
		}
		for index, stop := range gradient.Stops {
			if math.IsNaN(stop.Position) || index > 0 && stop.Position < gradient.Stops[index-1].Position {
				t.Fatalf("stops %v out of order", gradient.Stops)
			}
		}
	})
}
