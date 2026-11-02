package fakecalibreweb

import (
	"fmt"
	"html"
	"maps"
	"slices"
	"strings"
)

// showcaseChapter is a chapter of the showcase EPUB (or of the vertical one): a topic of its own.
type showcaseChapter struct {
	file, title string
	// language and direction are the html element's lang and dir ("" for none), bodyClass the
	// body's class.
	language, direction, bodyClass string
	body                           string
}

// showcaseChapters are the chapters of the showcase EPUB: Arabic, Hebrew, text in both
// directions, Chinese and Japanese lines and their punctuation, the emphasis marks and
// underlines of CSS Text, backgrounds and SVG strokes, what a book cannot do (run a script,
// fetch from the web, keep a page waiting for a slow image), and audio and video, which never
// play.
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
<p>Marks above: <span class="circles">circled words</span>, <span class="double">double circles</span>; under: <span class="under-marks">triangles</span>.</p>
<p><u>An ordinary underline, gypsy</u>, and <u class="under">one under the descenders, gypsy</u>.</p>
<p class="last">A justified paragraph whose last line is centred by text-align-last, written with a vendor prefix: the words of its lines stretch to both margins, but the last.</p>`},
	{file: "backgrounds.xhtml", title: "Backgrounds", language: "en", body: `<h1>Backgrounds</h1>
<div class="grid"><p>Squared paper: two repeating gradients, a line every 40 pixels each way, tiled by background-size.</p></div>
<div class="glow"><p>A radial gradient, light around a point left of the middle, darker to the corners.</p></div>
<p>The same square drawn in a viewBox 10 wide, its stroke 0.5, and in one 1000 wide, its stroke 50: both lines are as thick.</p>
<p class="strokes"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" width="80" height="80"><rect x="1" y="1" width="8" height="8" fill="none" stroke="black" stroke-width="0.5"/></svg>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 1000" width="80" height="80"><rect x="100" y="100" width="800" height="800" fill="none" stroke="black" stroke-width="50"/></svg></p>`},
	{file: "safety.xhtml", title: "Safety", language: "en", body: `<h1>Safety</h1>
<p>Nothing in a book runs.<script>document.write(" This sentence was written by a script.")</script> The script after the first sentence wrote nothing, and <span onclick="this.textContent = 'clicked'">this text has a click handler</span> that never runs.</p>
<p>A picture on the web is never fetched: the one after this paragraph is left out.</p>
<p><img src="https://images.example.com/harbour.jpg" alt="A harbour at dusk, on the web"/></p>
<p>A photograph of 30 megapixels follows, slow to decode. A page waits 10 seconds in all for its pictures; with a shorter wait, its description shows in its place.</p>
<p><img src="large.jpg" alt="A large photograph of gray squares, too slow for this page" style="width: 40%"/></p>`},
	{file: "media.xhtml", title: "Media", language: "en", body: `<h1>Media</h1>
<p>Nothing plays. A video shows its poster, a still the book holds, and never its controls:</p>
<div class="still"><video src="clip.mp4" poster="poster.jpg" controls="controls" style="width: 50%"><p>This fallback hides behind the poster.</p></video></div>
<video src="clip.mp4" controls="controls"><source src="clip.webm" type="video/webm"/><p>A video without a poster shows its fallback content: this paragraph.</p></video>
<audio src="song.mp3" controls="controls"><p>A song shows its fallback content too: this paragraph.</p></audio>
<p>A book read aloud, with a media overlay, reads as its text alone.</p>`},
}

// The showcase's images, in the book: the large photograph and the video's poster.
const (
	largeImagePath  = "large.jpg"
	posterImagePath = "poster.jpg"
)

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
.double { text-emphasis: double-circle; }
.under-marks { text-emphasis-style: filled triangle; text-emphasis-position: under right; }
.under { text-underline-position: under; }
.last { -epub-text-align-last: center; }
.grid { border: 1px solid black; padding: 2em 1em; margin: 1em 0; background: repeating-linear-gradient(transparent 0 38px, #999 38px 40px),
  repeating-linear-gradient(90deg, transparent 0 38px, #999 38px 40px) 0 0 / 80px 80px, white; }
.glow { padding: 2em 1em; margin: 1em 0; background: radial-gradient(circle at 30% 50%, white, #777); }
.strokes { text-indent: 0; text-align: center; }
.still { text-align: center; margin: 1em 0; }
`

// makeShowcaseEPUB returns the EPUB of the showcase book: a chapter per topic of
// showcaseChapters.
func makeShowcaseEPUB(item book) ([]byte, error) {
	large, err := largeImage()
	if err != nil {
		return nil, err
	}
	poster, err := drawPoster()
	if err != nil {
		return nil, err
	}
	return chaptersEPUB(item, showcaseChapters, showcaseCSS, "en", map[string][]byte{largeImagePath: large, posterImagePath: poster})
}

// chaptersEPUB returns the EPUB of a book of chapters styled by css, in language, with the JPEG
// images of images by path.
func chaptersEPUB(item book, chapters []showcaseChapter, css, language string, images map[string][]byte) ([]byte, error) {
	files := map[string][]byte{
		"META-INF/container.xml": []byte(containerXML),
		"OEBPS/style.css":        []byte(css),
		"OEBPS/content.opf":      []byte(chaptersOPF(item, chapters, language, slices.Sorted(maps.Keys(images)))),
		"OEBPS/nav.xhtml":        []byte(chaptersNav(chapters)),
	}
	for imagePath, data := range images {
		files["OEBPS/"+imagePath] = data
	}
	for _, chapter := range chapters {
		attributes, bodyAttributes := "", ""
		if chapter.direction != "" {
			attributes = fmt.Sprintf(` dir="%s"`, chapter.direction)
		}
		if chapter.bodyClass != "" {
			bodyAttributes = fmt.Sprintf(` class="%s"`, chapter.bodyClass)
		}
		files["OEBPS/"+chapter.file] = []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="%s" lang="%s"%s>
<head><title>%s</title><link rel="stylesheet" type="text/css" href="style.css"/></head>
<body%s>
%s
</body>
</html>
`, chapter.language, chapter.language, attributes, chapter.title, bodyAttributes, chapter.body))
	}
	return zipFiles([]storedFile{{"mimetype", []byte("application/epub+zip")}}, files)
}

// chaptersOPF returns the package document of a book of chapters, in language, with JPEG
// images at imagePaths.
func chaptersOPF(item book, chapters []showcaseChapter, language string, imagePaths []string) string {
	var manifest, spine strings.Builder
	for index, chapter := range chapters {
		fmt.Fprintf(&manifest, "    <item id=\"chapter%d\" href=\"%s\" media-type=\"application/xhtml+xml\"/>\n", index+1, chapter.file)
		fmt.Fprintf(&spine, "    <itemref idref=\"chapter%d\"/>\n", index+1)
	}
	for index, imagePath := range imagePaths {
		fmt.Fprintf(&manifest, "    <item id=\"image%d\" href=\"%s\" media-type=\"image/jpeg\"/>\n", index+1, imagePath)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:calibre:%d</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:creator>%s</dc:creator>
    <dc:language>%s</dc:language>
    <meta property="dcterms:modified">2026-10-01T12:00:00Z</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="css" href="style.css" media-type="text/css"/>
%s  </manifest>
  <spine>
%s  </spine>
</package>
`, item.ID, html.EscapeString(item.Title), html.EscapeString(item.Author), language, manifest.String(), spine.String())
}

// chaptersNav returns the navigation document of a book of chapters: an entry per chapter.
func chaptersNav(chapters []showcaseChapter) string {
	var entries strings.Builder
	for _, chapter := range chapters {
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
