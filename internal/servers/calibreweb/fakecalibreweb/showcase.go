package fakecalibreweb

import (
	"fmt"
	"html"
	"strings"
)

// showcaseChapter is a chapter of the showcase EPUB: a topic of its own.
type showcaseChapter struct {
	file, title string
	// language and direction are the html element's lang and dir ("" for none).
	language, direction string
	body                string
}

// showcaseChapters are the chapters of the showcase EPUB: Arabic, Hebrew, text in both
// directions, Chinese and Japanese lines and their punctuation, and the emphasis marks and
// underlines of CSS Text.
var showcaseChapters = []showcaseChapter{
	{file: "arabic.xhtml", title: "العربية", language: "ar", direction: "rtl", body: `<h1>العربية</h1>
<p>مرحبا بكم في هذا الكتاب. تكتب اللغة العربية من اليمين إلى اليسار، وتتصل حروفها ببعضها البعض داخل الكلمة، فيتغير شكل الحرف بحسب موضعه في أولها أو وسطها أو آخرها.</p>
<p>هذه الفقرة الثانية تختبر ترتيب الكلمات على السطر، والأرقام مثل 2026 و١٤٤٧ التي تبقى من اليسار إلى اليمين، وعلامات الترقيم: هل تظهر في مكانها الصحيح؟</p>`},
	{file: "hebrew.xhtml", title: "עברית", language: "he", direction: "rtl", body: `<h1>עברית</h1>
<p>שָׁלוֹם! זוהי פסקה בעברית. העברית נכתבת מימין לשמאל, והמילה הראשונה של הפסקה מנוקדת. מספרים כמו 1948 ו־3.14 נשארים משמאל לימין בתוך השורה.</p>
<p class="start">פסקה מיושרת להתחלה, שהיא הצד הימני בעברית.</p>`},
	{file: "mixed.xhtml", title: "Both directions", language: "en", body: `<h1>Both directions</h1>
<p>In English, the Hebrew word <span lang="he">שלום</span> and the Arabic word <span lang="ar">سلام</span> both mean peace; a phrase such as <span lang="he">ספר טוב מאוד</span> keeps its own order inside the line.</p>
<p dir="rtl" lang="he">הספר Inkshelf קורא ספרי EPUB 3 על PocketBook, גם בשפות אחרות.</p>
<p>The user <bdi>إيان</bdi> wrote 3 posts; <bdo dir="rtl">reversed</bdo> by an override.</p>`},
	{file: "cjk.xhtml", title: "日本語と中文", language: "ja", body: `<h1>日本語と中文</h1>
<p>「吾輩は猫である。」名前はまだ無い。どこで生れたかとんと見当がつかぬ。何でも薄暗いじめじめした所でニャーニャー泣いていた事だけは記憶している。吾輩はここで始めて人間というものを見た。しかもあとで聞くとそれは書生という人間中で一番獰悪な種族であったそうだ。</p>
<p lang="zh">学而时习之，不亦说乎？有朋自远方来，不亦乐乎？人不知而不愠，不亦君子乎？「曾子曰：吾日三省吾身。」为人谋而不忠乎？与朋友交而不信乎？传不习乎？</p>
<p class="strict">line-break: strict はちょっと待ってのような小さい仮名の前で改行しない。</p>
<p lang="ko" class="keep">한국어 문장은 띄어쓰기로 단어를 나누며, keep-all은 단어 안에서 줄을 바꾸지 않습니다.</p>`},
	{file: "emphasis.xhtml", title: "Emphasis", language: "en", body: `<h1>Emphasis</h1>
<p lang="ja">日本語の<span class="dots">強調</span>は文字の上に<span class="sesame">傍点</span>を打つ。</p>
<p>Marks above: <span class="circles">circled words</span>; under: <span class="under-marks">triangles</span>.</p>
<p><u>An ordinary underline, gypsy</u>, and <u class="under">one under the descenders, gypsy</u>.</p>
<p class="last">A justified paragraph whose last line is centred by text-align-last, written with a vendor prefix: the words of its lines stretch to both margins, but the last.</p>`},
}

// showcaseCSS is the stylesheet of the showcase: prefixed properties among them.
const showcaseCSS = `body { margin: 0; }
p { margin: 0 0 0.6em; text-indent: 1.5em; text-align: justify; }
h1 { text-align: center; margin: 1em 0; }
.start { text-align: start; }
.strict { line-break: strict; }
.keep { word-break: keep-all; }
.dots { text-emphasis: filled dot; }
.sesame { -epub-text-emphasis-style: sesame; }
.circles { -webkit-text-emphasis-style: open circle; }
.under-marks { text-emphasis-style: filled triangle; text-emphasis-position: under right; }
.under { text-underline-position: under; }
.last { -epub-text-align-last: center; }
`

// makeShowcaseEPUB returns the EPUB of the showcase book: a chapter per topic of
// showcaseChapters.
func makeShowcaseEPUB(item book) ([]byte, error) {
	files := map[string][]byte{
		"META-INF/container.xml": []byte(containerXML),
		"OEBPS/style.css":        []byte(showcaseCSS),
		"OEBPS/content.opf":      []byte(showcaseOPF(item)),
		"OEBPS/nav.xhtml":        []byte(showcaseNav()),
	}
	for _, chapter := range showcaseChapters {
		attributes := ""
		if chapter.direction != "" {
			attributes = fmt.Sprintf(` dir="%s"`, chapter.direction)
		}
		files["OEBPS/"+chapter.file] = []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="%s" lang="%s"%s>
<head><title>%s</title><link rel="stylesheet" type="text/css" href="style.css"/></head>
<body>
%s
</body>
</html>
`, chapter.language, chapter.language, attributes, chapter.title, chapter.body))
	}
	return zipFiles([]storedFile{{"mimetype", []byte("application/epub+zip")}}, files)
}

// showcaseOPF returns the package document of the showcase book.
func showcaseOPF(item book) string {
	var manifest, spine strings.Builder
	for index, chapter := range showcaseChapters {
		fmt.Fprintf(&manifest, "    <item id=\"chapter%d\" href=\"%s\" media-type=\"application/xhtml+xml\"/>\n", index+1, chapter.file)
		fmt.Fprintf(&spine, "    <itemref idref=\"chapter%d\"/>\n", index+1)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:calibre:%d</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:creator>%s</dc:creator>
    <dc:language>en</dc:language>
    <meta property="dcterms:modified">2026-10-01T12:00:00Z</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="css" href="style.css" media-type="text/css"/>
%s  </manifest>
  <spine>
%s  </spine>
</package>
`, item.ID, html.EscapeString(item.Title), html.EscapeString(item.Author), manifest.String(), spine.String())
}

// showcaseNav returns the navigation document of the showcase book: a chapter per topic.
func showcaseNav() string {
	var entries strings.Builder
	for _, chapter := range showcaseChapters {
		fmt.Fprintf(&entries, "    <li><a href=\"%s\">%s</a></li>\n", chapter.file, chapter.title)
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>Contents</title></head>
<body>
  <nav epub:type="toc"><ol>
` + entries.String() + `  </ol></nav>
</body>
</html>
`
}
