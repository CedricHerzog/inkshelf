package fakecalibreweb

// The vertical book is Japanese set as Japanese books are, in columns from the right
// (writing-mode: vertical-rl), its pages turning from right to left. Its spine gives no
// page-progression-direction: the reader finds it from the vertical text. Its text is the
// opening of Natsume Sōseki's I Am a Cat (1905), in the public domain.

// sosekiOpening is the opening of I Am a Cat.
const sosekiOpening = `吾輩は猫である。名前はまだ無い。どこで生れたかとんと見当がつかぬ。何でも薄暗いじめじめした所でニャーニャー泣いていた事だけは記憶している。吾輩はここで始めて人間というものを見た。しかもあとで聞くとそれは書生という人間中で一番獰悪な種族であったそうだ。この書生というのは時々我々を捕えて煮て食うという話である。しかしその当時は何という考もなかったから別段恐しいとも思わなかった。ただ彼の掌に載せられてスーと持ち上げられた時何だかフワフワした感じがあったばかりである。`

// sosekiContinued follows sosekiOpening.
const sosekiContinued = `掌の上で少し落ちついて書生の顔を見たのがいわゆる人間というものの見始であろう。この時妙なものだと思った感じが今でも残っている。第一毛をもって装飾されべきはずの顔がつるつるしてまるで薬缶だ。その後猫にもだいぶ逢ったがこんな片輪には一度も出会わした事がない。のみならず顔の真中があまりに突起している。そうしてその穴の中から時々ぷうぷうと煙を吹く。どうも咽せぽくて実に弱った。これが人間の飲む煙草というものである事はようやくこの頃知った。`

// verticalChapters are the chapters of the vertical book: the vertical text, with a Latin word,
// a year across its column, sesame marks, a horizontal table and a boxed note, then an Arabic
// chapter, which keeps its horizontal lines.
var verticalChapters = []showcaseChapter{
	{file: "tategaki.xhtml", title: "縦書き", language: "ja", bodyClass: "vrtl", body: `<h1>第一章　縦書き</h1>
<p>` + sosekiOpening + `</p>
<p>この本は<span class="latin">Inkshelf</span>で読む。西暦<span class="tcy">2026</span>年、縦書きの<span class="sesame">傍点</span>は文字の右に打ち、<u>傍線</u>も右に引く。「括弧」と長音の「ー」は縦の形になる。</p>
<table class="yoko">
<tr><th>Name</th><th>Year</th></tr>
<tr><td>Inkshelf</td><td>2026</td></tr>
</table>
<p>` + sosekiContinued + `</p>
<div class="note"><p>注：吾輩は名前のない猫である。</p></div>
<p>` + sosekiOpening + `</p>
<p>` + sosekiContinued + `</p>`},
	{file: "arabic.xhtml", title: "العربية", language: "ar", direction: "rtl", body: `<h1>العربية</h1>
<p>مرحبا بكم في هذا الكتاب. تكتب اللغة العربية من اليمين إلى اليسار، وتتصل حروفها ببعضها البعض داخل الكلمة، فيتغير شكل الحرف بحسب موضعه في أولها أو وسطها أو آخرها.</p>
<p>هذه الفقرة الثانية تختبر ترتيب الكلمات على السطر، والأرقام مثل 2026 و١٤٤٧ التي تبقى من اليسار إلى اليمين، وعلامات الترقيم: هل تظهر في مكانها الصحيح؟</p>`},
}

// verticalCSS is the stylesheet of the vertical book, with the prefixed properties that EPUB
// 3.0 books give. Its margins, borders and paddings name the sides on the screen, as CSS does: in
// vertical-rl, the space after the heading is its margin-left, and the note's double rule on its
// right is before it.
const verticalCSS = `body { margin: 0; }
body.vrtl { -epub-writing-mode: vertical-rl; -webkit-writing-mode: vertical-rl; writing-mode: vertical-rl; }
p { margin: 0; text-indent: 1em; text-align: justify; }
h1 { margin: 0 0 1em; }
body.vrtl h1 { margin: 0 0 0 1em; }
.tcy { -epub-text-combine: horizontal; -webkit-text-combine: horizontal; text-combine-upright: all; }
.sesame { -epub-text-emphasis-style: sesame; text-emphasis-style: sesame; }
table.yoko { writing-mode: horizontal-tb; border-collapse: collapse; margin: 1em 0; }
th, td { border: 1px solid black; padding: 0.2em 0.6em; }
.note { border-right: 4px double black; border-left: 1px solid black; padding: 0 0.5em; margin: 2em 1em 0 0; }
.note p { text-indent: 0; }
`

// makeVerticalEPUB returns the EPUB of the vertical book.
func makeVerticalEPUB(item book) ([]byte, error) {
	return chaptersEPUB(item, verticalChapters, verticalCSS, "ja", nil)
}
