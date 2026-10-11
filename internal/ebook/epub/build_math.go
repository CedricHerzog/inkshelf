package epub

import (
	"encoding/xml"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/container"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/mathml"
)

// formulaMargin is the space above and below a displayed formula, in hundredths of an em.
const formulaMargin = 50

// inlineMath shows the <math> just opened, whose markup data starts with, as a formula laid
// out with the page (an image run, or a block if its display is block): decoder then skips its
// content. One past the limits of package mathml shows its image or its text alternative,
// else the text of its tokens, read as the chapter's.
func (parser *builder) inlineMath(decoder *xml.Decoder, data []byte) {
	formula := parser.top()
	if formula.hidden {
		return
	}
	root, err := mathml.Read(container.NewDecoder(data))
	if err == nil && decoder.Skip() == nil {
		if root.Display() {
			parser.addImageBlock(mathml.Path(root), 0)
			block := &parser.doc.Blocks[len(parser.doc.Blocks)-1]
			block.MarginTop, block.MarginBottom = max(block.MarginTop, formulaMargin), max(block.MarginBottom, formulaMargin)
		} else {
			run := formula.run
			if parser.spaceNeeded && parser.pendingRunes > 0 {
				run.Text = " "
				parser.appendRun(run)
			}
			parser.spaceNeeded = false
			run.Text, run.Image, run.Style = "", mathml.Path(root), run.Style&document.Small
			parser.appendRun(run)
		}
		parser.end("math")
		return
	}
	switch attributes := formula.attributes; {
	case attributes["altimg"] != "":
		parser.addImage(attributes["altimg"], formula)
		formula.hidden = true
	case attributes["alttext"] != "":
		parser.appendText(attributes["alttext"], formula, false)
		formula.hidden = true
	}
}
