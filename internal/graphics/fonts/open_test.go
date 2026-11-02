package fonts

import "testing"

// BenchmarkOpenSize opens the reader's serif font at a size it was not opened at yet, as a
// page's heading or note does: the file is parsed already, so only the size is made.
func BenchmarkOpenSize(b *testing.B) {
	if _, err := Open(Serif, Regular, 44); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	size := 0
	for b.Loop() {
		font, err := Open(Serif, Regular, 20+size%40)
		if err != nil {
			b.Fatal(err)
		}
		font.Close()
		size++
	}
}
