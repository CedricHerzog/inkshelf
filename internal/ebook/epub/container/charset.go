package container

import (
	"fmt"
	"io"

	"golang.org/x/text/encoding/htmlindex"
)

// charsetReader decodes input from charset (an XML declaration's encoding) into UTF-8.
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	encoding, err := htmlindex.Get(charset)
	if err != nil {
		return nil, fmt.Errorf("charset %q: %w", charset, err)
	}
	return encoding.NewDecoder().Reader(input), nil
}
