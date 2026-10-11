package pageimage

import "testing"

func TestClipLevel(t *testing.T) {
	// 1000 pixels: 2 black, 3 at gray 10, the others at 100, but 5 at 200 and 1 white.
	var histogram [256]int
	histogram[0], histogram[10], histogram[100], histogram[200], histogram[255] = 2, 3, 989, 5, 1
	tests := []struct {
		name  string
		clip  int
		order func(yield func(int) bool)
		want  int
	}{
		{name: "nothing clipped, from black", clip: 0, order: darkToLight, want: 0},
		{name: "nothing clipped, from white", clip: 0, order: lightToDark, want: 255},
		{name: "black clipped", clip: 2, order: darkToLight, want: 10},
		{name: "black and gray 10 clipped", clip: 5, order: darkToLight, want: 100},
		{name: "white clipped", clip: 1, order: lightToDark, want: 200},
		{name: "white and gray 200 clipped", clip: 6, order: lightToDark, want: 100},
		{name: "everything clipped, from black", clip: 1000, order: darkToLight, want: 255},
		{name: "everything clipped, from white", clip: 1000, order: lightToDark, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := clipLevel(&histogram, test.clip, test.order); got != test.want {
				t.Errorf("clipLevel = %d, want %d", got, test.want)
			}
		})
	}
}

func TestLevelsBlankPage(t *testing.T) {
	var histogram [256]int
	histogram[128] = 1000
	tones := toneTable(&histogram)
	if tones[0] != 0 || tones[255] != 255 {
		t.Fatalf("black → %d, white → %d; want them unchanged", tones[0], tones[255])
	}
}
