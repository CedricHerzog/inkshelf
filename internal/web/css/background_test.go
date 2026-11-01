package css

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

func TestBackgroundShorthand(t *testing.T) {
	for _, test := range []struct{ declaration, want string }{
		{"background: #fff", "background-color=#fff background-image=none background-position=0% 0% background-repeat=repeat background-size=auto"},
		{"background: url(a.png) no-repeat center / cover rgba(0, 0, 0, 0.5)",
			"background-color=rgba(0, 0, 0, 0.5) background-image=url(a.png) background-position=center background-repeat=no-repeat background-size=cover"},
		{"background: linear-gradient(red 1px, transparent 1px) 0 0 / 50px 50px, white",
			"background-color=white background-image=linear-gradient(red 1px, transparent 1px), none background-position=0 0, 0% 0% " +
				"background-repeat=repeat, repeat background-size=50px 50px, auto"},
		{"background: repeat-x top left padding-box fixed url(b.png)",
			"background-color=transparent background-image=url(b.png) background-position=top left background-repeat=repeat-x background-size=auto"},
		{"background: none", "background-color=transparent background-image=none background-position=0% 0% background-repeat=repeat background-size=auto"},
		{"background: red, url(a.png)", ""},      // a color only in the last layer
		{"background: url(a.png) / cover", ""},   // a size only after a position
		{"background: url(a.png) 0 0 / big", ""}, // no such size
	} {
		cascade := NewCascade()
		cascade.Add(Parse("p {"+test.declaration+"}", screen))
		values := cascade.Compute(add(nil, "p", "p"), "")
		var got []string
		for _, property := range slices.Sorted(maps.Keys(values)) {
			got = append(got, property+"="+values[property])
		}
		if strings.Join(got, " ") != test.want {
			t.Errorf("%s:\n got %q\nwant %q", test.declaration, strings.Join(got, " "), test.want)
		}
	}
}

func TestBackgrounds(t *testing.T) {
	values := ParseDeclarations("background-image: linear-gradient(currentColor 1px, transparent 1px), linear-gradient(to right, currentColor 1px, transparent 1px), none, url('tile.png');" +
		"background-size: 50px 50px; background-repeat: repeat-x, space round; background-position: right 20%")
	layers := Backgrounds(values, Color{Gray: 9, Alpha: 255})
	if len(layers) != 3 {
		t.Fatalf("%d layers, want 3 (none left out): %+v", len(layers), layers)
	}
	grid, across, tile := layers[0], layers[1], layers[2]
	if grid.Gradient == nil || grid.Gradient.Stops[0].Color.Gray != 9 || across.Gradient == nil || across.Gradient.Angle != 90 {
		t.Errorf("gradients = %+v, %+v", grid.Gradient, across.Gradient)
	}
	if tile.Image != "tile.png" || tile.Gradient != nil {
		t.Errorf("image layer = %+v", tile)
	}
	for _, layer := range layers {
		if layer.Size != (BackgroundSize{Width: Length{50, "px"}, Height: Length{50, "px"}}) || layer.X != (Length{100, "%"}) || layer.Y != (Length{20, "%"}) {
			t.Errorf("size and position = %+v, %v, %v; want the lists repeated", layer.Size, layer.X, layer.Y)
		}
	}
	// The repeat list (2 values) repeats over the 4 images: repeat-x, space round, repeat-x, space round.
	if grid.RepeatX != Repeated || grid.RepeatY != NoRepeat || across.RepeatX != Spaced || across.RepeatY != Rounded || tile.RepeatY != Rounded {
		t.Errorf("repeats = %v %v, %v %v, %v %v", grid.RepeatX, grid.RepeatY, across.RepeatX, across.RepeatY, tile.RepeatX, tile.RepeatY)
	}
}

func TestBackgroundSizes(t *testing.T) {
	for _, test := range []struct {
		value string
		want  BackgroundSize
	}{
		{"cover", BackgroundSize{Cover: true}},
		{"contain", BackgroundSize{Contain: true}},
		{"auto", BackgroundSize{Width: Auto, Height: Auto}},
		{"100%", BackgroundSize{Width: Length{100, "%"}, Height: Auto}},
		{"auto 2em", BackgroundSize{Width: Auto, Height: Length{2, "em"}}},
		{"-1px", BackgroundSize{Width: Auto, Height: Auto}},
		{"1px 2px 3px", BackgroundSize{Width: Auto, Height: Auto}},
	} {
		if got := parseBackgroundSize(components(tokenize(test.value))); got != test.want {
			t.Errorf("parseBackgroundSize(%q) = %+v, want %+v", test.value, got, test.want)
		}
	}
}

func TestBackgroundLayersCapped(t *testing.T) {
	value := "background-image: url(a.png)" + strings.Repeat(", linear-gradient(red, blue)", 20)
	if layers := Backgrounds(ParseDeclarations(value), Black); len(layers) != maxBackgroundLayers || layers[0].Image != "a.png" {
		t.Errorf("%d layers, the first %+v; want %d, the top one kept", len(layers), layers[0], maxBackgroundLayers)
	}
}

func FuzzBackgrounds(f *testing.F) {
	for _, seed := range []string{"background: url(a) 0 0 / 1px 2px, red", "background: repeating-radial-gradient(circle at 1px 2px, red, blue) round space",
		"background-image: url(a), none; background-size: cover, 1px; background-position: right", "background: / /, ,"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, style string) {
		layers := Backgrounds(ParseDeclarations(style), Black)
		if len(layers) > maxBackgroundLayers {
			t.Fatalf("%d layers", len(layers))
		}
		for _, layer := range layers {
			if (layer.Image == "") == (layer.Gradient == nil) {
				t.Fatalf("layer %+v has not one image", layer)
			}
		}
	})
}
