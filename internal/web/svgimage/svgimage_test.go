package svgimage

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestIsSVG(t *testing.T) {
	cases := []struct {
		data string
		want bool
	}{
		{`<svg xmlns="http://www.w3.org/2000/svg"/>`, true},
		{"\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!-- drawn by hand -->\n" +
			`<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">` +
			`<svg viewBox="0 0 10 10"></svg>`, true},
		{`<?xml version="1.0" encoding="ISO-8859-1"?><svg width="10" height="10"/>`, true},
		{`<html><body><svg/></body></html>`, false},
		{"\xff\xd8\xff\xe0 a JPEG", false},
		{"", false},
	}
	for _, test := range cases {
		if got := IsSVG([]byte(test.data)); got != test.want {
			t.Errorf("IsSVG(%q) = %v, want %v", test.data, got, test.want)
		}
	}
}

func TestSize(t *testing.T) {
	cases := []struct {
		attributes string
		want       image.Point
	}{
		{`width="200" height="100"`, image.Pt(200, 100)},
		{`width="200px" height="1in"`, image.Pt(200, 96)},
		{`width="72pt" height="25.4mm"`, image.Pt(96, 96)},
		{`width="2.54cm" height="2em"`, image.Pt(96, 32)},
		{`width="100%" height="100%" viewBox="0 0 600 800"`, image.Pt(600, 800)},
		{`viewBox="10,10 600,800"`, image.Pt(600, 800)},
		{`width="300" viewBox="0 0 600 800"`, image.Pt(300, 400)},
		{`height="400" viewBox="0 0 600 800"`, image.Pt(300, 400)},
		{``, image.Pt(300, 150)},
		{`width="50%"`, image.Pt(300, 150)},
		{`width="120"`, image.Pt(120, 150)},
	}
	for _, test := range cases {
		got, err := Size([]byte(`<svg xmlns="http://www.w3.org/2000/svg" ` + test.attributes + `/>`))
		if err != nil || got != test.want {
			t.Errorf("Size of <svg %s> = %v, %v, want %v", test.attributes, got, err, test.want)
		}
	}
	if _, err := Size([]byte(`<html/>`)); err == nil {
		t.Error("Size of <html>: no error")
	}
}

// grayAt returns the gray of canvas at x, y.
func grayAt(canvas image.Image, x, y int) uint8 {
	return color.GrayModel.Convert(canvas.At(x, y)).(color.Gray).Y
}

// A black square fills the middle of its viewBox, which is scaled into a wider image and
// centred: white bars on its sides.
func TestRasterizeRectangle(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="100 100 40 40">` +
		`<rect x="110" y="110" width="20" height="20" fill="#000"/></svg>`
	canvas, err := Rasterize([]byte(svg), image.Pt(200, 100), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if canvas.Bounds() != image.Rect(0, 0, 200, 100) {
		t.Fatalf("bounds %v, want 200x100", canvas.Bounds())
	}
	// The viewBox becomes x 50 to 150, the square x 75 to 125, y 25 to 75.
	for _, point := range []image.Point{{100, 50}, {76, 26}, {124, 74}} {
		if gray := grayAt(canvas, point.X, point.Y); gray != 0 {
			t.Errorf("gray at %v = %d, want black", point, gray)
		}
	}
	for _, point := range []image.Point{{10, 50}, {190, 50}, {73, 50}, {127, 50}, {100, 23}, {100, 77}} {
		if gray := grayAt(canvas, point.X, point.Y); gray != 255 {
			t.Errorf("gray at %v = %d, want white", point, gray)
		}
	}
}

// A gray circle, with preserveAspectRatio="none" stretching it into an ellipse.
func TestRasterizeCircle(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" preserveAspectRatio="none">` +
		`<circle cx="5" cy="5" r="4" fill="rgb(128,128,128)"/></svg>`
	canvas, err := Rasterize([]byte(svg), image.Pt(100, 50), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if gray := grayAt(canvas, 50, 25); gray != 128 {
		t.Errorf("centre gray %d, want 128", gray)
	}
	if gray := grayAt(canvas, 88, 25); gray != 128 { // inside the ellipse, 40 px wide
		t.Errorf("gray at (88, 25) = %d, want 128", gray)
	}
	for _, point := range []image.Point{{2, 2}, {50, 2}, {95, 25}} {
		if gray := grayAt(canvas, point.X, point.Y); gray != 255 {
			t.Errorf("gray at %v = %d, want white", point, gray)
		}
	}
}

// encodedPNG returns a PNG of the given size and gray.
func encodedPNG(t *testing.T, size image.Point, gray uint8) []byte {
	t.Helper()
	picture := image.NewGray(image.Rectangle{Max: size})
	for index := range picture.Pix {
		picture.Pix[index] = gray
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, picture); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// The cover wrapper of EPUBs: an <image> filling the viewBox, read through the resolver,
// fitted and centred.
func TestRasterizeCoverWrapper(t *testing.T) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1"
  width="100%" height="100%" viewBox="0 0 600 800" preserveAspectRatio="xMidYMid meet">
  <image width="600" height="800" xlink:href="../Images/cover.png"/>
</svg>`
	size, err := Size([]byte(svg))
	if err != nil || size != image.Pt(600, 800) {
		t.Fatalf("Size = %v, %v, want 600x800", size, err)
	}
	cover := encodedPNG(t, image.Pt(60, 80), 0)
	var asked []string
	resolve := func(href string) ([]byte, error) {
		asked = append(asked, href)
		if href == "../Images/cover.png" {
			return cover, nil
		}
		return nil, errors.New("no such image")
	}
	canvas, err := Rasterize([]byte(svg), image.Pt(400, 400), Options{Resolve: resolve})
	if err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 {
		t.Errorf("resolver asked for %q, want the cover once", asked)
	}
	// 600×800 fitted in 400×400 is 300×400, from x 50 to 350.
	for _, point := range []image.Point{{52, 2}, {200, 200}, {347, 397}} {
		if gray := grayAt(canvas, point.X, point.Y); gray != 0 {
			t.Errorf("gray at %v = %d, want the cover's black", point, gray)
		}
	}
	for _, point := range []image.Point{{47, 200}, {353, 200}} {
		if gray := grayAt(canvas, point.X, point.Y); gray != 255 {
			t.Errorf("gray at %v = %d, want white", point, gray)
		}
	}
}

// An image embedded as a data: URI, sliced into its box: it covers the box and no more.
// Images inside <defs> are not drawn.
func TestRasterizeDataImage(t *testing.T) {
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(encodedPNG(t, image.Pt(20, 10), 0))
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
		`<defs><image href="` + uri + `" width="100" height="100"/></defs>` +
		`<image href="` + uri + `" x="25" y="25" width="50" height="50" preserveAspectRatio="xMidYMid slice"/></svg>`
	canvas, err := Rasterize([]byte(svg), image.Pt(100, 100), Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range []image.Point{{26, 26}, {50, 50}, {73, 73}} {
		if gray := grayAt(canvas, point.X, point.Y); gray != 0 {
			t.Errorf("gray at %v = %d, want black", point, gray)
		}
	}
	for _, point := range []image.Point{{10, 50}, {90, 50}, {50, 10}, {50, 90}} {
		if gray := grayAt(canvas, point.X, point.Y); gray != 255 {
			t.Errorf("gray at %v = %d, want white", point, gray)
		}
	}
}
