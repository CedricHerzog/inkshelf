package epub

import "net/url"

// linkTarget returns the target of a link to href in the document at chapterPath: its path in
// the book with its fragment, or an absolute http(s) or mailto URL; "" for another URL.
func linkTarget(chapterPath, href string) string {
	parsed, err := url.Parse(href)
	switch {
	case err != nil:
		return ""
	case parsed.Scheme == "http" || parsed.Scheme == "https" || parsed.Scheme == "mailto":
		return href
	case parsed.Scheme != "":
		return ""
	}
	target := chapterPath
	if parsed.Path != "" {
		target = resolve(chapterPath, href)
	}
	if parsed.Fragment != "" {
		target += "#" + parsed.Fragment
	}
	return target
}

// anchor records id at the current position.
func (parser *builder) anchor(id string) {
	if id == "" {
		return
	}
	if parser.doc.IDs == nil {
		parser.doc.IDs = map[string]int{}
	}
	if _, ok := parser.doc.IDs[id]; !ok {
		parser.doc.IDs[id] = parser.doc.Chars + parser.pendingRunes
	}
}
