package fakecalibreweb

import (
	"fmt"
	"html"
	"image"
	"image/color"
	"image/draw"
)

// fixedPageSize is the viewport of the pages of the fixed-layout EPUB, in CSS pixels.
var fixedPageSize = image.Pt(1200, 1800)

// makeFixedLayoutEPUB returns the EPUB of a fixed-layout book (rendition:layout
// pre-paginated, rendition:spread auto) of 7 pages, read from left to right:
//  1. one image, alone on the right, as a book's first page;
//  2. text placed with position: absolute over a background image that its stylesheet names,
//     pinned to the left, and
//  3. an SVG, pinned to the right: the first spread;
//  4. a page centred, alone (page-spread-center);
//  5. a page meant for portrait (rendition:orientation-portrait), alone;
//  6. squared paper tiled by its body's background, the initial containing block's grid, and
//  7. an image: the second spread, by alternation.
func makeFixedLayoutEPUB(item book) ([]byte, error) {
	page1, err := framedPage(170, item.Title, "Page 1")
	if err != nil {
		return nil, err
	}
	page4, err := framedPage(220, "Page 4", "centred, alone")
	if err != nil {
		return nil, err
	}
	page5, err := framedPage(140, "Page 5", "for portrait")
	if err != nil {
		return nil, err
	}
	page7, err := framedPage(190, "Page 7", "beside the grid")
	if err != nil {
		return nil, err
	}
	// The background: light gray stripes, darker towards the bottom.
	sky := image.NewGray(image.Rectangle{Max: fixedPageSize})
	for index := range sky.Pix {
		row := index / fixedPageSize.X
		sky.Pix[index] = uint8(235 - row*100/fixedPageSize.Y - row/60%2*20)
	}
	background, err := encodeJPEG(sky)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{
		"META-INF/container.xml":      []byte(containerXML),
		"OEBPS/content.opf":           []byte(fixedLayoutOPF(item)),
		"OEBPS/nav.xhtml":             []byte(fixedLayoutNav),
		"OEBPS/css/page.css":          []byte(fixedLayoutCSS),
		"OEBPS/images/page1.jpg":      page1,
		"OEBPS/images/background.jpg": background,
		"OEBPS/text/page1.xhtml":      []byte(fixedLayoutPage("Page 1", "", `<img src="../images/page1.jpg" alt="Page 1" style="width: 1200px; height: 1800px"/>`)),
		"OEBPS/text/page2.xhtml": []byte(fixedLayoutPage("Page 2", "text", `<div class="band"></div>
<h1 class="title">Le voyage</h1>
<div class="panel"></div>
<div class="caption"><p>Le train partit à l'aube. Par la fenêtre, les champs défilaient, puis les collines, puis la mer.</p>
<p class="second">Personne ne savait où il allait s'arrêter.</p></div>`)),
		"OEBPS/images/page3.svg": []byte(fixedLayoutSVG),
		"OEBPS/images/page4.jpg": page4,
		"OEBPS/images/page5.jpg": page5,
		"OEBPS/images/page7.jpg": page7,
		"OEBPS/text/page4.xhtml": []byte(fixedLayoutPage("Page 4", "", `<img src="../images/page4.jpg" alt="Page 4" style="width: 1200px; height: 1800px"/>`)),
		"OEBPS/text/page5.xhtml": []byte(fixedLayoutPage("Page 5", "", `<img src="../images/page5.jpg" alt="Page 5" style="width: 1200px; height: 1800px"/>`)),
		"OEBPS/text/page6.xhtml": []byte(fixedLayoutPage("Page 6", "grid", `<h1 class="title">Squared paper</h1>`)),
		"OEBPS/text/page7.xhtml": []byte(fixedLayoutPage("Page 7", "", `<img src="../images/page7.jpg" alt="Page 7" style="width: 1200px; height: 1800px"/>`)),
	}
	return zipFiles([]storedFile{{"mimetype", []byte("application/epub+zip")}}, files)
}

// framedPage returns a page that is one image: a black frame around gray, with lines of text,
// as a JPEG.
func framedPage(gray uint8, lines ...string) ([]byte, error) {
	picture := image.NewRGBA(image.Rectangle{Max: fixedPageSize})
	draw.Draw(picture, picture.Bounds(), image.White, image.Point{}, draw.Src)
	frame := image.Rect(80, 80, 1120, 1720)
	draw.Draw(picture, frame, image.Black, image.Point{}, draw.Src)
	draw.Draw(picture, frame.Inset(16), image.NewUniform(color.Gray{Y: gray}), image.Point{}, draw.Src)
	if err := drawLines(picture, frame, 96, lines); err != nil {
		return nil, err
	}
	return encodeJPEG(picture)
}

// fixedLayoutPage returns a page of the fixed-layout EPUB: its viewport, its stylesheet, and
// its body, of class bodyClass.
func fixedLayoutPage(title, bodyClass, body string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="fr">
<head>
  <title>%s</title>
  <meta name="viewport" content="width=%d, height=%d"/>
  <link rel="stylesheet" type="text/css" href="../css/page.css"/>
</head>
<body class="%s">
%s
</body>
</html>
`, title, fixedPageSize.X, fixedPageSize.Y, bodyClass, body)
}

// fixedLayoutCSS is the stylesheet of the fixed-layout pages: the second's background, which
// its URL names relative to the stylesheet, a gradient band at its top, a translucent panel
// under its caption, and its positioned title and caption.
const fixedLayoutCSS = `body { margin: 0; width: 1200px; height: 1800px; }
body.text { background-image: url(../images/background.jpg); }
body.grid { background: repeating-linear-gradient(transparent 0 58px, #888 58px 60px),
  repeating-linear-gradient(90deg, transparent 0 58px, #888 58px 60px) 0 0 / 120px 120px, white; }
.band { position: absolute; left: 0; top: 0; width: 1200px; height: 120px; background: linear-gradient(to right, #fff, #444 70%, black); }
.panel { position: absolute; left: 110px; top: 1100px; width: 980px; height: 540px; background-color: rgba(255, 255, 255, 0.75); }
.title { position: absolute; left: 100px; top: 150px; width: 1000px; margin: 0; font-size: 96px; text-align: center; }
.caption { position: absolute; left: 150px; top: 1150px; width: 900px; height: 450px; font-size: 44px; }
.caption p { margin: 0 0 30px; }
.second { font-style: italic; text-align: right; }
`

// fixedLayoutSVG is the third page: a picture drawn in vector shapes, the size of the
// viewport, styled by its <style>, the triangle filled with a pattern, and a title in <text>.
const fixedLayoutSVG = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="1800" viewBox="0 0 1200 1800">
  <style>
    circle { fill: #444; }
    .roof { fill: url(#bricks); stroke: black; stroke-width: 8; }
    text { font-family: sans-serif; font-size: 110px; font-weight: bold; text-anchor: middle; }
    .subtitle { font-size: 60px; font-weight: normal; font-style: italic; fill: #555; }
  </style>
  <defs>
    <pattern id="bricks" width="40" height="20" patternUnits="userSpaceOnUse">
      <rect width="40" height="20" fill="#999"/><path d="M0 10 H40" stroke="black"/>
    </pattern>
  </defs>
  <rect x="0" y="0" width="1200" height="1800" fill="white"/>
  <rect x="80" y="80" width="1040" height="1640" fill="none" stroke="black" stroke-width="16"/>
  <text x="600" y="300">Fin<tspan class="subtitle" dx="20">du voyage</tspan></text>
  <circle cx="600" cy="700" r="300"/>
  <path class="roof" d="M200 1500 L600 1100 L1000 1500 Z"/>
</svg>
`

// fixedLayoutOPF returns the package document of the fixed-layout book.
func fixedLayoutOPF(item book) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:calibre:%d</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:creator>%s</dc:creator>
    <dc:language>fr</dc:language>
    <meta property="dcterms:modified">2026-10-01T12:00:00Z</meta>
    <meta property="rendition:layout">pre-paginated</meta>
    <meta property="rendition:spread">auto</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="css" href="css/page.css" media-type="text/css"/>
    <item id="page1-image" href="images/page1.jpg" media-type="image/jpeg"/>
    <item id="background" href="images/background.jpg" media-type="image/jpeg"/>
    <item id="page1" href="text/page1.xhtml" media-type="application/xhtml+xml"/>
    <item id="page2" href="text/page2.xhtml" media-type="application/xhtml+xml"/>
    <item id="page3" href="images/page3.svg" media-type="image/svg+xml"/>
    <item id="page4-image" href="images/page4.jpg" media-type="image/jpeg"/>
    <item id="page5-image" href="images/page5.jpg" media-type="image/jpeg"/>
    <item id="page7-image" href="images/page7.jpg" media-type="image/jpeg"/>
    <item id="page4" href="text/page4.xhtml" media-type="application/xhtml+xml"/>
    <item id="page5" href="text/page5.xhtml" media-type="application/xhtml+xml"/>
    <item id="page6" href="text/page6.xhtml" media-type="application/xhtml+xml"/>
    <item id="page7" href="text/page7.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine page-progression-direction="ltr">
    <itemref idref="page1"/>
    <itemref idref="page2" properties="page-spread-left"/>
    <itemref idref="page3" properties="page-spread-right"/>
    <itemref idref="page4" properties="page-spread-center"/>
    <itemref idref="page5" properties="rendition:orientation-portrait"/>
    <itemref idref="page6"/>
    <itemref idref="page7"/>
  </spine>
</package>
`, item.ID, html.EscapeString(item.Title), html.EscapeString(item.Author))
}

// fixedLayoutNav is the navigation document of the fixed-layout book.
const fixedLayoutNav = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>Sommaire</title></head>
<body>
  <nav epub:type="toc"><ol>
    <li><a href="text/page1.xhtml">Page 1</a></li>
    <li><a href="text/page2.xhtml">Page 2</a></li>
    <li><a href="images/page3.svg">Page 3</a></li>
    <li><a href="text/page4.xhtml">Page 4</a></li>
    <li><a href="text/page5.xhtml">Page 5</a></li>
    <li><a href="text/page6.xhtml">Page 6</a></li>
    <li><a href="text/page7.xhtml">Page 7</a></li>
  </ol></nav>
</body>
</html>
`
