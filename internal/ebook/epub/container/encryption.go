package container

import (
	"errors"
	"slices"

	"github.com/CedricHerzog/inkshelf/internal/web/webfont"
)

// ErrDRM is the error of a book whose content is encrypted (fonts obfuscated only are fine).
var ErrDRM = errors.New("the book is protected by DRM")

// fontObfuscations are the encryption algorithms that only obfuscate fonts.
var fontObfuscations = []string{webfont.IDPFAlgorithm, webfont.AdobeAlgorithm}

// checkEncryption returns ErrDRM if META-INF/encryption.xml encrypts more than fonts, and
// records the obfuscated fonts.
func (book *Book) checkEncryption() error {
	if _, ok := book.files["META-INF/encryption.xml"]; !ok {
		return nil
	}
	var encryption struct {
		Data []struct {
			Method struct {
				Algorithm string `xml:"Algorithm,attr"`
			} `xml:"EncryptionMethod"`
			Reference struct {
				URI string `xml:"URI,attr"`
			} `xml:"CipherData>CipherReference"`
		} `xml:"EncryptedData"`
	}
	if err := book.readXML("META-INF/encryption.xml", &encryption); err != nil {
		return err
	}
	book.obfuscated = map[string]string{}
	for _, data := range encryption.Data {
		if !slices.Contains(fontObfuscations, data.Method.Algorithm) {
			return ErrDRM
		}
		book.obfuscated[resolve("", data.Reference.URI)] = data.Method.Algorithm // relative to the root
	}
	return nil
}
