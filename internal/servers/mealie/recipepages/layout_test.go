package recipepages

import (
	"slices"
	"strings"
	"testing"

	"golang.org/x/image/font"
)

// Lines fit the width, and a word too long for a line is split.
func TestWrap(t *testing.T) {
	face := newFaces()[bodyStyle]
	const width = 300
	lines := wrap(face, "Cuire à 180°C pendant 40-45 minutes.\nAnticonstitutionnellementissimement", width)
	if len(lines) < 4 || !strings.HasPrefix(lines[0], "Cuire") {
		t.Fatalf("lines %q", lines)
	}
	for _, text := range lines {
		if measured := font.MeasureString(face, text).Ceil(); measured > width {
			t.Errorf("line %q is %d px wide, over %d", text, measured, width)
		}
	}
}

// Empty ingredients, steps and notes are left out, and steps are numbered without them.
func TestLayoutSkipsEmpty(t *testing.T) {
	recipe := Recipe{
		Name:        "Test",
		Ingredients: []Entry{{Text: "sel"}, {}},
		Steps:       []Entry{{Text: "un"}, {}, {Text: "deux"}},
		Notes:       []Entry{{}},
	}
	var texts []string
	for _, laidOut := range Layout(recipe, false) {
		for _, placed := range laidOut.lines {
			texts = append(texts, placed.text)
		}
	}
	want := []string{"Test", "Ingredients", "•", "sel", "Instructions", "1.", "un", "2.", "deux"}
	if !slices.Equal(texts, want) {
		t.Fatalf("lines %q, want %q", texts, want)
	}
}
