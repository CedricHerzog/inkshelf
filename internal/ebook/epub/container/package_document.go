package container

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// The values of rendition:layout.
const (
	Reflowable   = "reflowable"
	PrePaginated = "pre-paginated"
)

// documentTypes are the media types of the chapters read as XHTML.
var documentTypes = []string{"application/xhtml+xml", "text/html"}

// IsDocumentType reports whether mediaType is that of a chapter read as XHTML.
func IsDocumentType(mediaType string) bool { return slices.Contains(documentTypes, mediaType) }

// imageTypes are the media types of the images the book can show: EPUB's core media types,
// image/jpg, which books write for image/jpeg, and image/jxl. An AVIF image goes to its
// fallback.
var imageTypes = []string{"image/gif", "image/jpeg", "image/jpg", "image/jxl", "image/png", "image/svg+xml", "image/webp"}

// SpineItem is an item of the spine: a part of the book, in reading order.
type SpineItem struct {
	Path      string // the manifest item's path, which links name
	MediaType string
	// Content and ContentType are the file shown and its media type: the item's, or the first
	// of its fallback chain that the book can show. Content is "" if there is none.
	Content, ContentType string
	Linear               bool
	Layout               string // from the itemref's properties, "" for the book's Layout
}

// manifestItem is a file of the book, as its package document lists it.
type manifestItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
	Fallback   string `xml:"fallback,attr"`
}

// packageDocument is the parts of the OPF that the app uses.
type packageDocument struct {
	UniqueIdentifier string `xml:"unique-identifier,attr"`
	Metadata         struct {
		Titles      []string `xml:"title"`
		Languages   []string `xml:"language"`
		Creators    []string `xml:"creator"`
		Identifiers []struct {
			ID    string `xml:"id,attr"`
			Value string `xml:",chardata"`
		} `xml:"identifier"`
		Metas []struct {
			Name     string `xml:"name,attr"`
			Content  string `xml:"content,attr"`
			Property string `xml:"property,attr"`
			Refines  string `xml:"refines,attr"`
			Value    string `xml:",chardata"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Manifest []manifestItem `xml:"manifest>item"`
	Spine    struct {
		TOC             string `xml:"toc,attr"`
		PageProgression string `xml:"page-progression-direction,attr"`
		ItemRefs        []struct {
			IDRef      string `xml:"idref,attr"`
			Linear     string `xml:"linear,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

// packageDocumentPath returns the path of the package document that META-INF/container.xml
// names. The first one is the book; any after it are other renditions of it.
func (book *Book) packageDocumentPath() (string, error) {
	var container struct {
		Rootfiles []struct {
			FullPath  string `xml:"full-path,attr"`
			MediaType string `xml:"media-type,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := book.readXML("META-INF/container.xml", &container); err != nil {
		return "", err
	}
	for _, rootfile := range container.Rootfiles {
		if rootfile.MediaType == "application/oebps-package+xml" {
			if rootfile.FullPath != "" {
				return rootfile.FullPath, nil
			}
			break
		}
	}
	if len(container.Rootfiles) > 0 && container.Rootfiles[0].FullPath != "" {
		return container.Rootfiles[0].FullPath, nil
	}
	return "", errors.New("no package document in META-INF/container.xml")
}

// readManifest reads the files that the package document lists, their paths resolved against
// it, and finds the navigation document, the NCX and the cover among them.
func (book *Book) readManifest(opf packageDocument, opfPath string) {
	book.manifest, book.manifestIDs = map[string]manifestItem{}, map[string]string{}
	for _, item := range opf.Manifest {
		item.Href = resolve(opfPath, item.Href)
		item.MediaType = mediaType(item.MediaType)
		book.manifest[item.ID] = item
		book.manifestIDs[item.Href] = item.ID
		properties := strings.Fields(item.Properties)
		switch {
		case slices.Contains(properties, "nav"):
			book.navPath = item.Href
		case slices.Contains(properties, "cover-image"):
			book.coverPath = item.Href
		}
	}
	if ncx, ok := book.manifest[opf.Spine.TOC]; ok {
		book.ncxPath = ncx.Href
	}
	if book.coverPath == "" {
		// EPUB 2 names its cover in a <meta name="cover">.
		for _, meta := range opf.Metadata.Metas {
			if cover, ok := book.manifest[meta.Content]; meta.Name == "cover" && ok {
				book.coverPath = cover.Href
			}
		}
	}
}

// readSpine reads the book's reading order: every spine item, and the linear ones as its
// chapters. An item the app cannot show is replaced by the first of its manifest fallbacks it
// can show.
func (book *Book) readSpine(opf packageDocument) error {
	switch direction := collapse(opf.Spine.PageProgression); direction {
	case "ltr", "rtl":
		book.PageProgression = direction
	}
	isContent := func(item manifestItem) bool {
		if item.Fallback != "" && slices.Contains(strings.Fields(item.Properties), "scripted") {
			return false // the app runs no scripts: its fallback is shown
		}
		return slices.Contains(documentTypes, item.MediaType) || slices.Contains(imageTypes, item.MediaType)
	}
	for _, itemRef := range opf.Spine.ItemRefs {
		item, ok := book.manifest[itemRef.IDRef]
		if !ok {
			return fmt.Errorf("spine item %q is not in the manifest", itemRef.IDRef)
		}
		spine := SpineItem{Path: item.Href, MediaType: item.MediaType, Linear: itemRef.Linear != "no"}
		if content, ok := book.firstSupported(item.ID, isContent); ok {
			spine.Content, spine.ContentType = content.Href, content.MediaType
		} else if item.MediaType == "" || strings.Contains(item.MediaType, "xml") || strings.Contains(item.MediaType, "html") {
			// No fallback, and a media type books get wrong (text/xml, none): read it as XHTML.
			spine.Content, spine.ContentType = item.Href, "application/xhtml+xml"
		}
		for property := range strings.FieldsSeq(itemRef.Properties) {
			switch {
			case spine.Layout != "": // the first layout given wins
			case property == "rendition:layout-pre-paginated":
				spine.Layout = PrePaginated
			case property == "rendition:layout-reflowable":
				spine.Layout = Reflowable
			}
		}
		book.spine = append(book.spine, spine)
		if !spine.Linear {
			continue // shown only when a link leads to it
		}
		if _, ok := book.files[spine.Content]; spine.Content != "" && !ok {
			return fmt.Errorf("chapter %s is missing", spine.Content)
		}
		book.chapters = append(book.chapters, spine)
	}
	return nil
}

// readMetadata reads the title, language, creators, identifier and layout of the package
// document, their whitespace collapsed.
func (book *Book) readMetadata(opf packageDocument) {
	metadata := opf.Metadata
	if len(metadata.Titles) > 0 {
		book.Title = collapse(metadata.Titles[0])
	}
	if len(metadata.Languages) > 0 {
		book.Language = collapse(metadata.Languages[0])
	}
	for _, creator := range metadata.Creators {
		if creator := collapse(creator); creator != "" {
			book.Creators = append(book.Creators, creator)
		}
	}
	for _, identifier := range metadata.Identifiers {
		if identifier.ID == opf.UniqueIdentifier {
			book.Identifier = collapse(identifier.Value)
			break
		}
	}
	if book.Identifier == "" && len(metadata.Identifiers) > 0 {
		book.Identifier = collapse(metadata.Identifiers[0].Value) // the package names none
	}
	book.identifiers = []string{book.Identifier}
	for _, identifier := range metadata.Identifiers {
		if identifier := collapse(identifier.Value); identifier != book.Identifier {
			book.identifiers = append(book.identifiers, identifier)
		}
	}
	book.Layout = Reflowable
	for _, meta := range metadata.Metas {
		if meta.Property == "rendition:layout" && meta.Refines == "" {
			if collapse(meta.Value) == PrePaginated {
				book.Layout = PrePaginated
			}
			break // the first one counts
		}
	}
}

// firstSupported follows the manifest's fallback chain from item id to the first supported
// item, stopping at the first repeated item. It returns false if none is.
func (book *Book) firstSupported(id string, supported func(item manifestItem) bool) (manifestItem, bool) {
	seen := map[string]bool{}
	for !seen[id] {
		item, ok := book.manifest[id]
		if !ok {
			break
		}
		if supported(item) {
			return item, true
		}
		seen[id] = true
		id = item.Fallback
	}
	return manifestItem{}, false
}

// SupportedImage returns the image to show for the file at filePath: the first item of its
// manifest fallback chain that is an image the book can show. It returns false if the manifest
// does not list filePath or no item of the chain is such an image.
func (book *Book) SupportedImage(filePath string) (string, bool) {
	id, ok := book.manifestIDs[filePath]
	if !ok {
		return "", false
	}
	item, ok := book.firstSupported(id, func(item manifestItem) bool { return slices.Contains(imageTypes, item.MediaType) })
	return item.Href, ok
}

// mediaType returns a media type of the manifest in lower case, without its parameters.
func mediaType(declared string) string {
	declared, _, _ = strings.Cut(declared, ";")
	return strings.ToLower(strings.TrimSpace(declared))
}
