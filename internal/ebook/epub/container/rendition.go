package container

import (
	"cmp"
	"strings"
)

// The values of rendition:layout. Roll, from EPUB 3.4's draft, is a strip of images read one
// after the other, as a webtoon is.
const (
	Reflowable   = "reflowable"
	PrePaginated = "pre-paginated"
	Roll         = "roll"
)

// The values of rendition:flow: how the content is meant to move, by pages or by scrolling.
const (
	FlowAuto               = "auto"
	FlowPaginated          = "paginated"
	FlowScrolledContinuous = "scrolled-continuous"
	FlowScrolledDoc        = "scrolled-doc"
)

// The values of rendition:spread: in which orientations two pages show side by side.
const (
	SpreadNone      = "none"
	SpreadLandscape = "landscape"
	SpreadBoth      = "both"
	SpreadAuto      = "auto"
)

// The values of rendition:orientation: the orientation the publisher meant the pages for.
const (
	OrientationAuto      = "auto"
	OrientationLandscape = "landscape"
	OrientationPortrait  = "portrait"
)

// The page-spread properties of a spine item: the side of a spread its page takes, or the
// middle, alone, for a page meant to be seen whole.
const (
	PageSpreadLeft   = "left"
	PageSpreadRight  = "right"
	PageSpreadCenter = "center"
)

// spreadPortrait is the deprecated rendition:spread value "portrait", which EPUB 3.3 reads as
// "both".
const spreadPortrait = "portrait"

// Rendition is how the publisher asks for the book to be shown. The package sets it for the
// whole book, with <meta property="rendition:…"> elements; an itemref of the spine may set it
// for its own item with properties such as rendition:spread-none, which win over the package's.
// A value the app does not know is ignored, as EPUB 3.3 asks: the default stays.
type Rendition struct {
	Layout      string // Reflowable, PrePaginated or Roll
	Flow        string // FlowAuto, FlowPaginated, FlowScrolledContinuous or FlowScrolledDoc
	Spread      string // SpreadNone, SpreadLandscape, SpreadBoth or SpreadAuto
	Orientation string // OrientationAuto, OrientationLandscape or OrientationPortrait
	// PageSpread is set on spine items only: PageSpreadLeft, PageSpreadRight, PageSpreadCenter,
	// or "" for a page that takes whichever side comes.
	PageSpread string
}

// defaultRendition is what a book that says nothing gets.
var defaultRendition = Rendition{Layout: Reflowable, Flow: FlowAuto, Spread: SpreadAuto, Orientation: OrientationAuto}

// renditionValues lists the values each rendition property may take, by the property's name
// after "rendition:".
var renditionValues = map[string][]string{
	"layout":      {Reflowable, PrePaginated, Roll},
	"flow":        {FlowAuto, FlowPaginated, FlowScrolledContinuous, FlowScrolledDoc},
	"spread":      {SpreadNone, SpreadLandscape, SpreadBoth, SpreadAuto, spreadPortrait},
	"orientation": {OrientationAuto, OrientationLandscape, OrientationPortrait},
}

// set sets the property name to value unless the property is set already, so that the first
// valid value given wins. A name or value it does not know sets nothing.
func (rendition *Rendition) set(name, value string) {
	var field *string
	switch name {
	case "layout":
		field = &rendition.Layout
	case "flow":
		field = &rendition.Flow
	case "spread":
		field = &rendition.Spread
	case "orientation":
		field = &rendition.Orientation
	default:
		return
	}
	valid := false
	for _, known := range renditionValues[name] {
		valid = valid || known == value
	}
	if !valid {
		return
	}
	if name == "spread" && value == spreadPortrait {
		value = SpreadBoth
	}
	if *field == "" {
		*field = value
	}
}

// readPackageRendition reads the package's rendition metas, those that refine nothing, and
// fills what they leave unset with the defaults.
func readPackageRendition(opf packageDocument) Rendition {
	var rendition Rendition
	for _, meta := range opf.Metadata.Metas {
		if name, ok := strings.CutPrefix(meta.Property, "rendition:"); ok && meta.Refines == "" {
			rendition.set(name, collapse(meta.Value))
		}
	}
	return rendition.withDefaults(defaultRendition)
}

// pageSpreads are the itemref properties that place a page in a spread. EPUB 3.0 wrote left
// and right without the rendition: prefix, and EPUB 3.3 keeps both spellings.
var pageSpreads = map[string]string{
	"page-spread-left":             PageSpreadLeft,
	"rendition:page-spread-left":   PageSpreadLeft,
	"page-spread-right":            PageSpreadRight,
	"rendition:page-spread-right":  PageSpreadRight,
	"page-spread-center":           PageSpreadCenter,
	"rendition:page-spread-center": PageSpreadCenter,
}

// readItemRendition reads the rendition properties of an itemref's properties attribute. What
// they leave unset stays "", for the package's value.
func readItemRendition(properties string) Rendition {
	var rendition Rendition
	for property := range strings.FieldsSeq(properties) {
		if side, ok := pageSpreads[property]; ok {
			if rendition.PageSpread == "" {
				rendition.PageSpread = side
			}
			continue
		}
		setting, ok := strings.CutPrefix(property, "rendition:")
		if !ok {
			continue
		}
		if name, value, ok := strings.Cut(setting, "-"); ok {
			rendition.set(name, value)
		}
	}
	return rendition
}

// withDefaults returns rendition with each property it leaves unset taken from defaults.
func (rendition Rendition) withDefaults(defaults Rendition) Rendition {
	return Rendition{
		Layout:      cmp.Or(rendition.Layout, defaults.Layout),
		Flow:        cmp.Or(rendition.Flow, defaults.Flow),
		Spread:      cmp.Or(rendition.Spread, defaults.Spread),
		Orientation: cmp.Or(rendition.Orientation, defaults.Orientation),
		PageSpread:  cmp.Or(rendition.PageSpread, defaults.PageSpread),
	}
}

// RenditionOf returns how item is to be shown: its itemref's properties, and the package's for
// what they leave unset.
func (book *Book) RenditionOf(item SpineItem) Rendition {
	return item.Rendition.withDefaults(book.Rendition)
}
