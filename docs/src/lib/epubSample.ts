// The small EPUB 3 book that Inkshelf's tests open (epub3 in
// internal/ebook/epub/epub_test.go), file by file, a few long lines trimmed. The site's
// widgets explore it.

import type {ItemRef, ManifestItem} from './epubSpine';

export type SampleFile = {
  path: string;
  /** What the file is, in a few words. */
  role: string;
  /** Its content, or a description in brackets for a binary file. */
  content: string;
  /** Paths this file points to, which the explorer draws arrows to. */
  links?: readonly string[];
  /** Links that only a link (not "Next page") follows. */
  weakLinks?: readonly string[];
  binary?: boolean;
};

export const containerXml = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf"
              media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`;

export const contentOpf = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title> Le Livre </dc:title>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="css" href="css/style.css" media-type="text/css"/>
    <item id="cover" href="images/cover%20page.png" media-type="image/png" properties="cover-image"/>
    <item id="coverpage" href="text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="c1" href="text/c1.xhtml" media-type="application/xhtml+xml"/>
    <item id="notes" href="text/notes.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="text/c2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="coverpage"/>
    <itemref idref="c1"/>
    <itemref idref="notes" linear="no"/>
    <itemref idref="c2"/>
  </spine>
</package>`;

export const sampleFiles: readonly SampleFile[] = [
  {
    path: 'mimetype',
    role: 'The first file, stored without compression: it says “I am an EPUB”.',
    content: 'application/epub+zip',
  },
  {
    path: 'META-INF/container.xml',
    role: 'The only fixed address in the zip. It points to the package document.',
    content: containerXml,
    links: ['OEBPS/content.opf'],
  },
  {
    path: 'OEBPS/content.opf',
    role: 'The package document: metadata, manifest (every file) and spine (the reading order).',
    content: contentOpf,
    links: ['OEBPS/text/cover.xhtml', 'OEBPS/text/c1.xhtml', 'OEBPS/text/c2.xhtml', 'OEBPS/nav.xhtml'],
    weakLinks: ['OEBPS/text/notes.xhtml'],
  },
  {
    path: 'OEBPS/nav.xhtml',
    role: 'The navigation document: the table of contents, as a web page.',
    content: `<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"><body>
<nav epub:type="toc"><ol>
  <li><a href="text/c1.xhtml">Chapitre <i>un</i></a>
    <ol><li><a href="text/c1.xhtml#here">Ici</a></li></ol></li>
  <li><a href="text/c2.xhtml">Chapitre deux</a></li>
</ol></nav>
<nav epub:type="landmarks"><ol>
  <li><a href="text/cover.xhtml">Cover</a></li>
</ol></nav>
</body></html>`,
    links: ['OEBPS/text/c1.xhtml', 'OEBPS/text/c2.xhtml'],
  },
  {
    path: 'OEBPS/toc.ncx',
    role: 'The EPUB 2 table of contents, kept for old readers. Read only when there is no nav.',
    content: `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/"><navMap>
<navPoint>
  <navLabel><text>NCX one</text></navLabel>
  <content src="text/c1.xhtml"/>
</navPoint>
</navMap></ncx>`,
    links: ['OEBPS/text/c1.xhtml'],
  },
  {
    path: 'OEBPS/css/style.css',
    role: 'The book’s stylesheet, shared by its chapters.',
    content: `/* the book's style */
p { text-indent: 1.5em; margin: 0 }
p.first, .noindent { text-indent: 0 }
.center { text-align: center }
span.it { font-style: italic }
.hidden { display: none }
div.chapter h2 { page-break-before: always; font-weight: normal }
@media amzn-kf8 { p { margin: 3em } }`,
  },
  {
    path: 'OEBPS/images/cover page.png',
    role: 'The cover image, 600×900 pixels. Its href in the OPF writes the space as %20.',
    content: '[a PNG image, 600 × 900 pixels]',
    binary: true,
  },
  {
    path: 'OEBPS/text/cover.xhtml',
    role: 'The first item of the spine: a page holding the cover in an SVG.',
    content: `<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:xlink="http://www.w3.org/1999/xlink"><body>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 900">
  <image width="600" height="900"
         xlink:href="../images/cover%20page.png"/>
</svg>
</body></html>`,
    links: ['OEBPS/images/cover page.png'],
  },
  {
    path: 'OEBPS/text/c1.xhtml',
    role: 'Chapter one: a web page (XHTML) styled by CSS.',
    content: `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Un</title>
<link rel="stylesheet" type="text/css" href="../css/style.css"/>
<style>.small { font-size: 80% }</style></head>
<body><div class="chapter">
<h1 id="one">Chapitre&nbsp;1</h1>
<p class="first">Il   était
 une <em>fois</em>, <span class="it">dit-il</span>&#160;: <b>fin</b>.</p>
<p>Deuxième<br/>ligne <span class="hidden">cachée</span>suite.</p>
<p id="here" class="center small">Centré</p>
<ul><li>un</li><li>deux</li></ul>
…
</div></body></html>`,
    links: ['OEBPS/css/style.css'],
  },
  {
    path: 'OEBPS/text/notes.xhtml',
    role: 'Notes, marked linear="no" in the spine: reached only through a link.',
    content: `<html xmlns="http://www.w3.org/1999/xhtml"><body>
<p>Note</p>
</body></html>`,
  },
  {
    path: 'OEBPS/text/c2.xhtml',
    role: 'Chapter two, written in Latin-1 rather than UTF-8: Inkshelf decodes it.',
    content: `<?xml version="1.0" encoding="iso-8859-1"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body>
<h2>Deux</h2>
<p>Café crème.</p>
</body></html>`,
  },
];

/** The manifest and spine of content.opf, for the spine widget. */
export const sampleManifest: readonly ManifestItem[] = [
  {id: 'nav', href: 'nav.xhtml', mediaType: 'application/xhtml+xml', properties: 'nav'},
  {id: 'ncx', href: 'toc.ncx', mediaType: 'application/x-dtbncx+xml'},
  {id: 'css', href: 'css/style.css', mediaType: 'text/css'},
  {id: 'cover', href: 'images/cover%20page.png', mediaType: 'image/png', properties: 'cover-image'},
  {id: 'coverpage', href: 'text/cover.xhtml', mediaType: 'application/xhtml+xml'},
  {id: 'c1', href: 'text/c1.xhtml', mediaType: 'application/xhtml+xml'},
  {id: 'notes', href: 'text/notes.xhtml', mediaType: 'application/xhtml+xml'},
  {id: 'c2', href: 'text/c2.xhtml', mediaType: 'application/xhtml+xml'},
];

export const sampleSpine: readonly ItemRef[] = [
  {idref: 'coverpage'},
  {idref: 'c1'},
  {idref: 'notes', linear: 'no'},
  {idref: 'c2'},
];

/** The fallback chains of TestFallbacks (epub_test.go): a PDF, and a loop. */
export const fallbackChain: readonly ManifestItem[] = [
  {id: 'pdf', href: 'part.pdf', mediaType: 'application/pdf', fallback: 'dtbook'},
  {id: 'dtbook', href: 'part.dtb', mediaType: 'application/x-dtbook+xml', fallback: 'part'},
  {id: 'part', href: 'part.xhtml', mediaType: 'application/xhtml+xml'},
];

export const fallbackLoop: readonly ManifestItem[] = [
  {id: 'loop1', href: 'loop1.bin', mediaType: 'application/x-one', fallback: 'loop2'},
  {id: 'loop2', href: 'loop2.bin', mediaType: 'application/x-two', fallback: 'loop1'},
];
