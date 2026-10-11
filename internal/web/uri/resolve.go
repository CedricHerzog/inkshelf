package uri

import (
	"net/url"
	"path"
	"strings"
)

// Resolve returns the path of the file that href, in the file at base, links to: without its
// fragment, and percent-decoded. As in a URL, ".." at the root stays there.
func Resolve(base, href string) string {
	href, _, _ = strings.Cut(href, "#")
	if unescaped, err := url.PathUnescape(href); err == nil {
		href = unescaped
	}
	if !strings.HasPrefix(href, "/") {
		href = path.Join(path.Dir(base), href)
	}
	return strings.TrimPrefix(path.Join("/", href), "/")
}
