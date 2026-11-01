package svgimage

import (
	"image"
	"testing"
)

// darkRun returns the length of the run of dark pixels in the row y of gray that starts at or
// after x.
func darkRun(gray *image.Gray, x, y int) (start, length int) {
	dark := func(column int) bool { return gray.GrayAt(column, y).Y < 128 }
	start = x
	for start < gray.Bounds().Max.X && !dark(start) {
		start++
	}
	for start+length < gray.Bounds().Max.X && dark(start+length) {
		length++
	}
	return start, length
}

// A stroke is as thick as the drawing says, whatever its viewBox: 1 unit of a 10-unit-wide
// drawing and 100 of a 1000-unit-wide one both make 20 pixels of 200. A transform scales it too,
// and so are dashes and their offset.
func TestStrokeWidthsScaled(t *testing.T) {
	small := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><path d="M 5 0 V 10" stroke="black" stroke-width="1"/></svg>`
	large := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 1000"><path d="M 500 0 V 1000" stroke="black" stroke-width="100"/></svg>`
	scaled := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 1000"><g transform="scale(10)"><path d="M 50 0 V 100" stroke="black" stroke-width="10"/></g></svg>`
	initial := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><path d="M 5 0 V 10" stroke="black"/></svg>`
	for name, svg := range map[string]string{"10x10": small, "1000x1000": large, "a transform": scaled, "no width": initial} {
		gray, err := Rasterize([]byte(svg), image.Pt(200, 200), Options{})
		if err != nil {
			t.Fatal(err)
		}
		if start, width := darkRun(gray, 0, 100); width < 19 || width > 21 || start < 89 || start > 91 {
			t.Errorf("%s: stroke %d pixels wide from %d, want 20 from 90", name, width, start)
		}
	}

	dashed := func(viewBox, width, dashes, offset string) string {
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ` + viewBox + ` ` + viewBox + `"><path d="M 0 ` + width + ` H ` + viewBox +
			`" stroke="black" stroke-width="` + width + `" stroke-dasharray="` + dashes + `" stroke-dashoffset="` + offset + `"/></svg>`
	}
	var runs [][]int
	for _, svg := range []string{dashed("10", "1", "2 1", "0.5"), dashed("1000", "100", "200 100", "50")} {
		gray, err := Rasterize([]byte(svg), image.Pt(200, 200), Options{})
		if err != nil {
			t.Fatal(err)
		}
		var lengths []int
		for x := 0; x < 200; {
			start, length := darkRun(gray, x, 20)
			if length == 0 {
				break
			}
			lengths = append(lengths, start, length)
			x = start + length
		}
		runs = append(runs, lengths)
	}
	if len(runs[0]) < 6 || len(runs[0]) != len(runs[1]) {
		t.Fatalf("dashes %v and %v, want the same several", runs[0], runs[1])
	}
	for index := range runs[0] {
		if difference := runs[0][index] - runs[1][index]; difference < -1 || difference > 1 {
			t.Errorf("dashes %v and %v differ", runs[0], runs[1])
			break
		}
	}
	if first := runs[0]; first[1] < 29 || first[1] > 31 || first[3] < 39 || first[3] > 41 {
		t.Errorf("dashes %v, want 30 then 40 pixels: 2 units less the offset's 0.5, then 2", first)
	}
}
