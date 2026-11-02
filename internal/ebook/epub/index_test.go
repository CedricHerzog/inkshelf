package epub

import (
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/epubtest"
)

// A book whose text is vertical-rl turns its pages from right to left, unless its spine says
// otherwise; its cover, an image, does not count.
func TestVerticalPageProgression(t *testing.T) {
	for _, test := range []struct {
		css, spine, want string
	}{
		{"", "", ""},
		{"body { writing-mode: vertical-rl }", "", "rtl"},
		{"body { -epub-writing-mode: vertical-rl }", "", "rtl"},
		{"body { writing-mode: vertical-lr }", "", ""},
		{"body { writing-mode: vertical-rl }", `page-progression-direction="ltr"`, "ltr"},
	} {
		files := epub3(t)
		files["OEBPS/css/style.css"] += test.css
		files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"], `<spine toc="ncx">`, `<spine toc="ncx" `+test.spine+`>`, 1)
		book, err := Open(epubtest.Write(t, files))
		if err != nil {
			t.Fatal(err)
		}
		index, err := BuildIndex(book)
		book.Close()
		if err != nil {
			t.Fatal(err)
		}
		if index.PageProgression != test.want {
			t.Errorf("%q, %q: page progression %q, want %q", test.css, test.spine, index.PageProgression, test.want)
		}
	}
}
