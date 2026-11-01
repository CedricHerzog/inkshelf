package fakecalibreweb

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// The showcase EPUB opens with a chapter per topic: the Arabic one right to left, the
// emphasis one with its marks.
func TestShowcaseEPUB(t *testing.T) {
	data, err := makeShowcaseEPUB(book{ID: 51, Title: "World scripts", Author: "Anne Martin"})
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
	if got := opened.Chapters(); got != len(showcaseChapters) {
		t.Fatalf("%d chapters, want %d", got, len(showcaseChapters))
	}
	arabic, err := opened.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	if block := arabic.Blocks[1]; block.Direction != document.RightToLeft || arabic.Language != "ar" {
		t.Errorf("Arabic paragraph %+v in %q, want right to left", block, arabic.Language)
	}
	emphasisIndex := slices.IndexFunc(showcaseChapters, func(chapter showcaseChapter) bool { return chapter.file == "emphasis.xhtml" })
	emphasis, err := opened.Chapter(emphasisIndex)
	if err != nil {
		t.Fatal(err)
	}
	marks := 0
	for _, block := range emphasis.Blocks {
		for _, run := range block.Runs {
			if run.Emphasis.Mark != 0 {
				marks++
			}
		}
	}
	if marks != 5 { // dots, sesames, circles, double circles, triangles
		t.Errorf("%d runs with emphasis marks, want 5", marks)
	}
}
