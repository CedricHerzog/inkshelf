package fakecalibreweb

import (
	"fmt"
	"strings"
)

// The chapters of the fake EPUB. Each one exercises a part of the reader's layout: the
// first has links of every kind, the second floats and SVG, the last MathML, a bordered box
// and a table.

// formulasFile is the last chapter of a book's EPUB, formulasXHTML, titled formulasTitle.
const (
	formulasFile  = "OEBPS/text/formulas.xhtml"
	formulasTitle = "Formules"
)

// formulasXHTML is the last chapter of a book: MathML in lines of text (a fraction, scripts,
// a root) and displayed (the quadratic formula, a sum with its limits, a matrix, an integral,
// a binomial coefficient and a cube root); then a box, bordered, on a gray background, and a table.
const formulasXHTML = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:m="http://www.w3.org/1998/Math/MathML" xml:lang="fr">
<head><title>Formules</title><link rel="stylesheet" type="text/css" href="../style.css"/></head>
<body>
  <h1>Formules</h1>
  <p class="first">La moitié d'un tiers, <math xmlns="http://www.w3.org/1998/Math/MathML"><mfrac><mn>1</mn><mn>6</mn></mfrac></math>,
  tient dans la ligne, comme le carré <math xmlns="http://www.w3.org/1998/Math/MathML"><msup><mi>x</mi><mn>2</mn></msup></math>,
  l'indice <math xmlns="http://www.w3.org/1998/Math/MathML"><msub><mi>a</mi><mrow><mi>n</mi><mo>+</mo><mn>1</mn></mrow></msub></math>
  et la racine <math xmlns="http://www.w3.org/1998/Math/MathML"><msqrt><mn>2</mn></msqrt><mo>&#8776;</mo><mn>1,414</mn></math>.</p>
  <p>Les racines de <m:math><m:mi>a</m:mi><m:msup><m:mi>x</m:mi><m:mn>2</m:mn></m:msup><m:mo>+</m:mo><m:mi>b</m:mi><m:mi>x</m:mi><m:mo>+</m:mo><m:mi>c</m:mi><m:mo>=</m:mo><m:mn>0</m:mn></m:math> sont</p>
  <math xmlns="http://www.w3.org/1998/Math/MathML" display="block"><mi>x</mi><mo>=</mo><mfrac><mrow><mo>&#8722;</mo><mi>b</mi><mo>&#177;</mo><msqrt><msup><mi>b</mi><mn>2</mn></msup><mo>&#8722;</mo><mn>4</mn><mi>a</mi><mi>c</mi></msqrt></mrow><mrow><mn>2</mn><mi>a</mi></mrow></mfrac></math>
  <p>La somme des premiers entiers&#160;:</p>
  <math xmlns="http://www.w3.org/1998/Math/MathML" display="block"><munderover><mo>&#8721;</mo><mrow><mi>k</mi><mo>=</mo><mn>1</mn></mrow><mi>n</mi></munderover><mi>k</mi><mo>=</mo><mfrac><mrow><mi>n</mi><mo>(</mo><mi>n</mi><mo>+</mo><mn>1</mn><mo>)</mo></mrow><mn>2</mn></mfrac></math>
  <p>Une matrice et son déterminant&#160;:</p>
  <math xmlns="http://www.w3.org/1998/Math/MathML" display="block"><mi>A</mi><mo>=</mo><mrow><mo>(</mo><mtable><mtr><mtd><mi>a</mi></mtd><mtd><mi>b</mi></mtd></mtr><mtr><mtd><mi>c</mi></mtd><mtd><mi>d</mi></mtd></mtr></mtable><mo>)</mo></mrow><mo>,</mo><mspace width="1em"/><mo>det</mo><mi>A</mi><mo>=</mo><mi>a</mi><mi>d</mi><mo>&#8722;</mo><mi>b</mi><mi>c</mi></math>
  <p>Une intégrale, un coefficient binomial et une racine cubique&#160;:</p>
  <math xmlns="http://www.w3.org/1998/Math/MathML" display="block"><msubsup><mo>&#8747;</mo><mn>0</mn><mn>1</mn></msubsup><msup><mi>x</mi><mn>2</mn></msup><mspace width="0.2em"/><mi>d</mi><mi>x</mi><mo>=</mo><mfrac><mn>1</mn><mn>3</mn></mfrac><mo>,</mo><mspace width="1em"/><mrow><mo>(</mo><mfrac linethickness="0"><mi>n</mi><mi>k</mi></mfrac><mo>)</mo></mrow><mo>,</mo><mspace width="1em"/><mroot><mn>27</mn><mn>3</mn></mroot><mo>=</mo><mn>3</mn></math>
  <p>Après les formules, le texte reprend.</p>
  <div class="encadre"><p class="first">Un encadré&#160;: sa bordure, son fond gris, <span class="gris">et un texte gris</span>.</p></div>
  <table border="1">
    <caption>Trois constantes</caption>
    <thead><tr><th>Nom</th><th>Symbole</th><th>Valeur</th></tr></thead>
    <tbody>
      <tr><td>Pi</td><td><i>π</i></td><td>3,14159</td></tr>
      <tr><td>Nombre d'Euler</td><td><i>e</i></td><td>2,71828</td></tr>
      <tr><td>Nombre d'or</td><td><i>φ</i></td><td>1,61803</td></tr>
    </tbody>
  </table>
</body>
</html>
`

// figureSVG is the illustration of the second chapter, an SVG: the plate, linked by a relative
// href and sliced into a frame, over a vector ornament styled by its <style>, filled with a
// pattern, and a caption in <text>.
const figureSVG = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="100%" height="100%" viewBox="0 0 400 340">
  <style>
    .frame { fill: none; stroke: black; stroke-width: 6 }
    .rule { stroke: rgba(0, 0, 0, 0.6); stroke-width: 3 }
    text { font-family: serif; font-size: 24px; font-style: italic; text-anchor: middle; fill: #222 }
  </style>
  <defs>
    <pattern id="hatch" width="8" height="8" patternUnits="userSpaceOnUse">
      <rect width="8" height="8" fill="#999"/><path d="M0 8 L8 0" stroke="black"/>
    </pattern>
  </defs>
  <image xlink:href="plate.png" x="20" y="20" width="360" height="200" preserveAspectRatio="xMidYMid slice"/>
  <rect class="frame" x="20" y="20" width="360" height="200"/>
  <line class="rule" x1="40" y1="260" x2="160" y2="260"/>
  <path d="M200 240 L220 260 L200 280 L180 260 Z" fill="url(#hatch)" stroke="black" stroke-width="2"/>
  <line class="rule" x1="240" y1="260" x2="360" y2="260"/>
  <text x="200" y="325">Planche I</text>
</svg>
`

// emblemSVG is an <svg> in the second chapter's XHTML, after its illustration: its discs and
// text styled by the book's stylesheet (svg.emblem) and its own <style>.
const emblemSVG = `  <p class="centre"><svg xmlns="http://www.w3.org/2000/svg" class="emblem" width="480" height="80" viewBox="0 0 480 80">
    <style>.disc { stroke: black; stroke-width: 3 }</style>
    <circle class="disc" cx="40" cy="40" r="28"/><circle class="disc" cx="440" cy="40" r="28"/>
    <text x="240" y="48">Fin de la planche</text>
  </svg></p>
`

// notesXHTML is the notes of a book, a non-linear item of the spine that only links lead to:
// EPUB 2 notes, paragraphs with an id, the second long.
const notesXHTML = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="fr">
<head><title>Notes</title><link rel="stylesheet" type="text/css" href="style.css"/></head>
<body>
  <h1>Notes</h1>
  <p id="note1">1. La lande est une étendue de terre où ne poussent que des plantes sauvages.</p>
  <p id="note2">2. Le village de la lande n'a pas de nom sur les cartes : les bergers l'appellent
  « le Bout du monde », car aucune route ne le traverse. On y arrive par un chemin de terre qui
  longe la rivière, puis par un pont de bois que les crues emportent tous les dix ans. Les
  habitants le reconstruisent chaque fois, au même endroit, avec les mêmes planches de chêne
  quand elles ont été retrouvées en aval.</p>
  <p>Les anciens disent que le premier pont fut bâti par un voyageur qui ne voulut jamais dire
  son nom, et qui repartit le lendemain. Personne ne sait d'où il venait ni où il allait ; on
  raconte seulement qu'il portait une lanterne, et qu'elle brûlait encore quand il disparut
  derrière la colline.</p>
  <p id="note3">3. Les étoiles de la lande sont plus nombreuses qu'en ville, faute de lumière.</p>
</body>
</html>
`

// linksXHTML returns the paragraph of links of the first chapter of a book: a footnote
// (an aside that the noteref's chapter holds), its last chapter, a note of the notes, the web;
// then the call of a publisher's note, without epub:type (publisherNoteXHTML).
func linksXHTML(item book) string {
	return fmt.Sprintf(`  <p>Voir la note<sup><a epub:type="noteref" href="#fn1">1</a></sup>, le <a href="chapter%02d.xhtml">%s</a>,
  les <a href="../notes.xhtml#note2">notes de fin</a> et <a href="https://example.org/etoiles">example.org</a>.</p>
  <aside epub:type="footnote" id="fn1"><p>1. Une note de bas de page : elle s'affiche par-dessus la page
  quand on touche son appel, et ne se lit pas dans le texte.</p></aside>
  <p>Une note d'éditeur<a class="apnote" id="ap_ntb-001-1" href="#ntb-001"><sup>2</sup></a>, sans epub:type.</p>
`, item.Chapters, strings.ToLower(chapterTitle(item.Chapters)))
}

// publisherNoteXHTML is the note at the end of the first chapter that linksXHTML calls, as
// French publishers write them: no epub:type, but a link back to the call's id.
const publisherNoteXHTML = `  <div class="ntb" id="ntb-001"><p><a href="#ap_ntb-001-1">2</a>. Une note d'éditeur : elle
  commence par un lien vers son appel, et s'affiche par-dessus la page comme une note de bas de page.</p></div>
`

// A chapter has chapterParagraphs paragraphs, in two scenes: the second starts at paragraph
// secondScene, which the table of contents links to. A printed page starts every
// printedPageParagraphs paragraphs.
const (
	chapterParagraphs     = 14
	secondScene           = 7
	printedPageParagraphs = 4
)

// sentences are the text of the chapters, in French and English.
var sentences = []string{
	"Le vent soufflait sur la lande, et la nuit tombait déjà sur le village.",
	"Elle ouvrit la porte&nbsp;: personne. Seule une lanterne brûlait encore au fond de la cour.",
	"« Qui va là&nbsp;? » demanda-t-il, la main sur la poignée de son épée.",
	"The road wound through the hills, and every turn showed a valley greener than the last.",
	"Il fallait par&#173;tir avant l'aube, sans bruit, et ne jamais se retour&#173;ner.",
	"Les étoiles, disait sa grand-mère, sont les fenêtres d'une maison immense.",
	"Nobody remembered who had built the tower, nor why its only door faced the sea.",
	"Ils marchèrent longtemps, jusqu'à ce que les arbres cèdent la place aux rochers.",
}

// chapterXHTML returns chapter number of a book: a heading, then paragraphs, the first
// chapter with the illustration and the links, and the publisher's note at its end.
func chapterXHTML(item book, number int) string {
	var body strings.Builder
	fmt.Fprintf(&body, "  <h1>%s</h1>\n", chapterTitle(number))
	switch number {
	case 1:
		body.WriteString("  <p class=\"centre\"><img src=\"../images/plate.png\" alt=\"Illustration\"/></p>\n")
		body.WriteString(linksXHTML(item))
	case 2:
		body.WriteString("  <p class=\"centre\"><img src=\"../images/figure.svg\" alt=\"Illustration\"/></p>\n")
		body.WriteString(emblemSVG)
	}
	for paragraph := range chapterParagraphs {
		class := ""
		switch paragraph {
		case 0:
			class = ` class="first"`
		case secondScene:
			class = ` id="scene2"`
			// A scene break in a character that the book's typeface may lack.
			body.WriteString("  <p class=\"centre\">◆◆◆</p>\n")
		}
		var text strings.Builder
		for sentence := range 3 + paragraph%4 {
			if text.Len() > 0 {
				text.WriteString(" ")
			}
			text.WriteString(sentences[(item.ID+number*5+paragraph*3+sentence)%len(sentences)])
		}
		line := text.String()
		switch paragraph % 5 {
		case 1:
			line = "<em>" + line + "</em>"
		case 3:
			line = "<strong>Soudain.</strong> " + line
		}
		if number == 2 {
			// A drop cap, and an image floated at the right of the text.
			switch paragraph {
			case 0:
				line = `<span class="lettrine">I</span>l était une fois, dans la lande, un village sans nom. ` + line
			case 2:
				line = `<img class="vignette" src="../images/plate.png" alt="Vignette"/>` + line
			}
		}
		if paragraph%printedPageParagraphs == 0 {
			printed := (number-1)*printedPagesPerChapter + paragraph/printedPageParagraphs + 1
			line = fmt.Sprintf(`<span epub:type="pagebreak" id="page%d" title="%d"/>`, printed, printed) + line
		}
		fmt.Fprintf(&body, "  <p%s>%s</p>\n", class, line)
	}
	if number == 1 {
		body.WriteString(publisherNoteXHTML)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="fr">
<head>
  <title>%s</title>
  <link rel="stylesheet" type="text/css" href="../style.css"/>
</head>
<body>
%s</body>
</html>
`, chapterTitle(number), body.String())
}
