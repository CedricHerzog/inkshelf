package fonts

import "testing"

// benchmarkLines are a line of a book's text in three scripts, with the direction and language
// the layout shapes them in. DejaVu Sans, the fallback font, has no CJK: the Japanese line is
// shaped to .notdef glyphs, which is what the reader draws for it without an embedded font.
var benchmarkLines = []struct {
	name        string
	text        string
	rightToLeft bool
	language    string
}{
	{name: "Latin", text: "Il se leva, prit son chapeau « déjà » usé, et sortit sans un mot — ﬁni.", language: "fr"},
	{name: "Arabic", text: "السلام عليكم ورحمة الله وبركاته، كيف حالك اليوم يا صديقي العزيز؟", rightToLeft: true, language: "ar"},
	{name: "CJK", text: "吾輩は猫である。名前はまだ無い。どこで生れたかとんと見当がつかぬ。", language: "ja"},
}

// BenchmarkShape shapes a line without the cache, as a page laid out for the first time does,
// and from the cache, as laying the page out again does.
func BenchmarkShape(b *testing.B) {
	font := openDejaVu(b)
	for _, line := range benchmarkLines {
		// The first call parses the font for the shaper, once per file: not what is measured.
		font.shape(line.text, line.rightToLeft, line.language)
		b.Run(line.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				font.shape(line.text, line.rightToLeft, line.language)
			}
		})
	}
	latin := benchmarkLines[0]
	b.Run("Latin cached", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			font.Shape(latin.text, latin.rightToLeft, latin.language)
		}
	})
}

// BenchmarkWidth measures a line from the cached advances and kerns, as the app's widgets do
// for every label they place.
func BenchmarkWidth(b *testing.B) {
	font := openDejaVu(b)
	text := benchmarkLines[0].text
	font.Width(text)
	b.ReportAllocs()
	for b.Loop() {
		font.Width(text)
	}
}

// BenchmarkParse parses a font file, opens it at a size and parses it again for the shaper:
// what a book's embedded fonts cost when the book opens.
func BenchmarkParse(b *testing.B) {
	data := readDejaVu(b)
	b.Run("Parse", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := Parse("DejaVuSans.ttf", data); err != nil {
				b.Fatal(err)
			}
		}
	})
	parsed, err := Parse("DejaVuSans.ttf", data)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("Open", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			font, err := parsed.Open(40)
			if err != nil {
				b.Fatal(err)
			}
			if err := font.Close(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("for the shaper", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			// A new File each time: the shaper's parse is done once per File.
			file := &File{data: data}
			if _, err := file.textFont(); err != nil {
				b.Fatal(err)
			}
		}
	})
}
