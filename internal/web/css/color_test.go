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

func FuzzParseColor(f *testing.F) {
	for _, seed := range []string{"red", "#abc", "#aabbccdd", "rgb(1 2 3 / 50%)", "hsla(1rad, 2%, 3%, .5)",
		"linear-gradient(to top left, red 10% 20%, blue)", "linear-gradient(45deg, #000, #fff 3px)", "url(x) #fff",
		"repeating-linear-gradient(red, red 1px, blue 2px)", "radial-gradient(circle 10px at 1px top, red, blue)",
		"repeating-radial-gradient(ellipse farthest-side at 20% 30%, red 0 2%, blue 4%)", "radial-gradient(at, red, blue)"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		ParseColor(value, Black)
		gradient, ok := ParseGradient(value, Black)
		if !ok {
			return
		}
		if len(gradient.Stops) < 2 || len(gradient.Stops) > maxGradientStops {
			t.Fatalf("%d stops", len(gradient.Stops))
		}
		for _, length := range []float64{0, 1, 1000} {
			stops := gradient.StopsAlong(length)
			for index, stop := range stops {
				if math.IsNaN(stop.Position) || index > 0 && stop.Position < stops[index-1].Position {
					t.Fatalf("stops %v along %v out of order", stops, length)
				}
			}
		}
	})
}
