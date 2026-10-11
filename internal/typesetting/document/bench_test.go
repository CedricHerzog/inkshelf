package document

import (
	"strings"
	"testing"
)

// ScanScripts runs on every block of a chapter before it is laid out: Latin text takes its
// fast path, a paragraph mixing Latin, Arabic and Chinese decodes every rune past it.
func BenchmarkScanScripts(b *testing.B) {
	for _, test := range []struct {
		name string
		runs []Run
	}{
		{"latin", []Run{
			{Text: strings.Repeat("Il fallait partir avant l'aube, sans bruit ; personne ne devait le savoir. ", 12)},
			{Text: strings.Repeat("Déjà, la route montait vers les collines — chaque virage montrait une vallée plus verte. ", 6), Style: Italic},
		}},
		{"mixed", []Run{
			{Text: strings.Repeat("The word for book is ", 6)},
			{Text: strings.Repeat("كتاب في المكتبة القديمة، ", 12)},
			{Text: strings.Repeat(" and in Chinese ", 6)},
			{Text: strings.Repeat("书在旧图书馆里。", 20), Style: Italic},
		}},
	} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				ScanScripts(test.runs)
			}
		})
	}
}
