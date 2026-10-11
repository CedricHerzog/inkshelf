package comicarchive

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// benchmarkPageCount is the pages of a volume of manga, about.
const benchmarkPageCount = 50

// benchmarkPageSize is the size of a page's JPEG, about, at the resolution Komga serves.
const benchmarkPageSize = 300 << 10

// writeBenchmarkArchive writes a CBZ of benchmarkPageCount pages of random bytes, which compress
// no more than JPEGs do, last page first so that Open has them to sort, and its ComicInfo.xml.
func writeBenchmarkArchive(b *testing.B) string {
	b.Helper()
	random := rand.New(rand.NewPCG(1, 2))
	names := []string{"ComicInfo.xml"}
	contents := map[string]string{"ComicInfo.xml": "<ComicInfo><Title>Tome 1</Title></ComicInfo>"}
	for index := range benchmarkPageCount {
		name := fmt.Sprintf("Tome 1/page %d.jpg", benchmarkPageCount-index)
		page := make([]byte, benchmarkPageSize)
		for byteIndex := range page {
			page[byteIndex] = byte(random.Uint32())
		}
		names = append(names, name)
		contents[name] = string(page)
	}
	return writeZip(b, names, contents)
}

// BenchmarkOpen opens the archive and lists its pages in order, what opening a downloaded CBZ
// costs before its first page.
func BenchmarkOpen(b *testing.B) {
	filePath := writeBenchmarkArchive(b)
	b.ReportAllocs()
	for b.Loop() {
		opened, err := Open(filePath)
		if err != nil {
			b.Fatal(err)
		}
		if err := opened.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPage reads the pages one after the other, as the download prepares them.
func BenchmarkPage(b *testing.B) {
	opened, err := Open(writeBenchmarkArchive(b))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := opened.Close(); err != nil {
			b.Error(err)
		}
	})
	b.SetBytes(benchmarkPageSize)
	b.ReportAllocs()
	number := 0
	for b.Loop() {
		if _, err := opened.Page(number%benchmarkPageCount + 1); err != nil {
			b.Fatal(err)
		}
		number++
	}
}
