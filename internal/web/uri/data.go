package uri

import (
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
)

// IsData reports whether uri is a data: URI.
func IsData(uri string) bool {
	return len(uri) >= len("data:") && strings.EqualFold(uri[:len("data:")], "data:")
}

// DecodeData returns the bytes of a data: URI, data:[media type][;base64],payload. A base64
// payload may hold spaces and lack its padding; any other is percent-encoded.
func DecodeData(uri string) ([]byte, error) {
	if !IsData(uri) {
		return nil, errors.New("not a data: URI")
	}
	header, payload, ok := strings.Cut(uri[len("data:"):], ",")
	if !ok {
		return nil, errors.New("data: URI without a comma")
	}
	if !strings.HasSuffix(strings.ToLower(header), ";base64") {
		text, err := url.PathUnescape(payload)
		return []byte(text), err
	}
	payload = strings.Map(func(character rune) rune {
		if strings.ContainsRune(" \t\r\n", character) {
			return -1
		}
		return character
	}, payload)
	return base64.RawStdEncoding.DecodeString(strings.TrimRight(payload, "="))
}
