package fakecalibreweb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// The vertical book's first chapter is vertical-rl, its year combined, its sesame marks and its
// table horizontal; its Arabic chapter is horizontal; and its pages turn from right to left,
// though its spine does not say so.
func TestVerticalEPUB(t *testing.T) {
	data, err := makeVerticalEPUB(book{ID: 52, Title: "Written down", Author: "Anne Martin"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "book.epub")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	opened, err := epub.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	vertical, err := opened.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	if vertical.WritingMode != document.VerticalRL {
		t.Errorf("chapter's writing mode %d, want vertical-rl", vertical.WritingMode)
	}
	var combined, sesame, horizontalCells int
	for _, block := range vertical.Blocks {
		if block.Cell != nil {
			if block.WritingMode == document.HorizontalTB {
				horizontalCells++
			}
			continue
		}
		if block.WritingMode != document.VerticalRL {
			t.Errorf("block %q in writing mode %d, want vertical-rl", block.Runs[0].Text, block.WritingMode)
		}
		for _, run := range block.Runs {
			if run.Combine == document.CombineAll && run.Text == "2026" {
				combined++
			}
			if run.Emphasis.Mark == '﹅' {
				sesame++
			}
		}
	}
	if combined != 1 || sesame != 1 || horizontalCells != 4 {
		t.Errorf("%d years combined, %d runs of sesames, %d horizontal cells; want 1, 1, 4", combined, sesame, horizontalCells)
	}
	arabic, err := opened.Chapter(1)
	if err != nil {
		t.Fatal(err)
	}
	if arabic.WritingMode != document.HorizontalTB {
		t.Errorf("Arabic chapter's writing mode %d, want horizontal", arabic.WritingMode)
	}
	index, err := epub.BuildIndex(opened)
	if err != nil {
		t.Fatal(err)
	}
	if !index.RightToLeft() {
		t.Errorf("page progression %q: want right to left", index.PageProgression)
	}
}
