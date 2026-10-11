package container

import (
	"encoding/xml"
	"errors"
	"strings"
)

// Link is a link of the navigation as the book writes it: its title, the path in the zip and
// the fragment it leads to, its depth in its list (0 for the top entries), and a landmark's
// epub:type as Kind.
type Link struct {
	Title, Path, Fragment string
	Level                 int
	Kind                  string
}

// Navigation is the lists of links of a book's navigation: its table of contents, the pages
// of the printed book and the landmarks.
type Navigation struct {
	TOC, PageList, Landmarks []Link
}

// NavigationPath returns the path in the zip of the EPUB 3 navigation document, "" if none.
func (book *Book) NavigationPath() string { return book.navPath }

// Navigation reads the EPUB 3 navigation document, and the table of contents of the EPUB 2
// NCX if that has none.
func (book *Book) Navigation() (Navigation, error) {
	var found Navigation
	var err error
	if book.navPath != "" {
		found, err = book.navLinks()
	}
	if len(found.TOC) == 0 && book.ncxPath != "" {
		found.TOC, err = book.ncxLinks()
	}
	if len(found.TOC) == 0 && err == nil {
		err = errors.New("the book has none")
	}
	return found, err
}

// navNode is an element of the navigation document, with its children.
type navNode struct {
	XMLName xml.Name
	// Type is the epub:type attribute: encoding/xml matches an attribute's local name in any
	// namespace.
	Type     string    `xml:"type,attr"`
	Href     string    `xml:"href,attr"`
	Alt      string    `xml:"alt,attr"` // of an image
	Text     string    `xml:",chardata"`
	Children []navNode `xml:",any"`
}

// unreadNav is the type given to the links of a nav that is not read: one of another type, or
// the second of a type.
const unreadNav = "#unread"

// navLinks returns the links of the navigation document's navs, with their depth: those of
// the first nav of each type (the table of contents being an untyped nav if none is "toc").
func (book *Book) navLinks() (Navigation, error) {
	var root navNode
	if err := book.readXML(book.navPath, &root); err != nil {
		return Navigation{}, err
	}
	navs := map[string][]Link{} // by the nav's type
	var walk func(node navNode, navType string, depth int)
	walk = func(node navNode, navType string, depth int) {
		switch strings.ToLower(node.XMLName.Local) {
		case "nav":
			navType = navTypeOf(node.Type)
			if len(navs[navType]) > 0 {
				navType = unreadNav
			}
		case "ol":
			depth++
		case "a":
			if navType != unreadNav && node.Href != "" {
				linkPath, fragment := book.target(book.navPath, node.Href)
				link := Link{Title: collapse(textOf(node)), Path: linkPath, Fragment: fragment, Level: max(depth-1, 0)}
				if navType == "landmarks" {
					link.Kind = collapse(node.Type)
				}
				navs[navType] = append(navs[navType], link)
				return
			}
		}
		for _, child := range node.Children {
			walk(child, navType, depth)
		}
	}
	walk(root, unreadNav, 0)
	found := Navigation{TOC: navs["toc"], PageList: navs["page-list"], Landmarks: navs["landmarks"]}
	if len(found.TOC) == 0 {
		found.TOC = navs[""]
	}
	return found, nil
}

// navTypeOf returns the type of a nav whose epub:type is declared: "toc", "page-list",
// "landmarks", "" if it has none, or unreadNav.
func navTypeOf(declared string) string {
	types := strings.Fields(declared)
	if len(types) == 0 {
		return ""
	}
	for _, navType := range types {
		switch navType {
		case "toc", "page-list", "landmarks":
			return navType
		}
	}
	return unreadNav
}

// textOf returns the text of node and its descendants, an image's being its alt text.
func textOf(node navNode) string {
	var text strings.Builder
	text.WriteString(node.Text + " " + node.Alt)
	for _, child := range node.Children {
		text.WriteString(" " + textOf(child))
	}
	return text.String()
}

// ncxPoint is a navPoint of an NCX.
type ncxPoint struct {
	Label   string `xml:"navLabel>text"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Points []ncxPoint `xml:"navPoint"`
}

// ncxLinks returns the navPoints of the NCX, with their depth.
func (book *Book) ncxLinks() ([]Link, error) {
	var ncx struct {
		Points []ncxPoint `xml:"navMap>navPoint"`
	}
	if err := book.readXML(book.ncxPath, &ncx); err != nil {
		return nil, err
	}
	var links []Link
	var walk func(points []ncxPoint, level int)
	walk = func(points []ncxPoint, level int) {
		for _, point := range points {
			linkPath, fragment := book.target(book.ncxPath, point.Content.Src)
			links = append(links, Link{Title: collapse(point.Label), Path: linkPath, Fragment: fragment, Level: level})
			walk(point.Points, level+1)
		}
	}
	walk(ncx.Points, 0)
	return links, nil
}

// target returns the path in the zip and the fragment of a link in the file at base.
func (book *Book) target(base, href string) (string, string) {
	_, fragment, _ := strings.Cut(href, "#")
	return resolve(base, href), fragment
}
