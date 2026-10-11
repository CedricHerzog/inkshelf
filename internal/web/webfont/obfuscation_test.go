package webfont

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// uuid is the publication identifier of the tests.
const uuid = "urn:uuid:A1B0D67E-2E81-4DF5-9E67-A64CBE366809"

// checkKey checks that Deobfuscate XORs the first obfuscatedLength bytes of zeros with
// wantKey repeated, and leaves the rest and its input alone.
func checkKey(t *testing.T, algorithm, identifier, wantKey string, obfuscatedLength int) {
	t.Helper()
	key, err := hex.DecodeString(wantKey)
	if err != nil {
		t.Fatal(err)
	}
	zeros := make([]byte, 2000)
	result, err := Deobfuscate(zeros, algorithm, identifier)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(zeros, make([]byte, 2000)) {
		t.Fatalf("Deobfuscate modified its input")
	}
	for index, value := range result {
		want := byte(0)
		if index < obfuscatedLength {
			want = key[index%len(key)]
		}
		if value != want {
			t.Fatalf("byte %d is %#x, want %#x", index, value, want)
		}
	}
}

func TestDeobfuscateIDPF(t *testing.T) {
	// The key is the SHA-1 of the identifier without whitespace (printf %s "$uuid" | shasum).
	checkKey(t, IDPFAlgorithm, " \t"+uuid+"\r\n", "cadfa4eec33f47580cdf7152e2a1954a626803f4", 1040)
}

func TestDeobfuscateAdobe(t *testing.T) {
	checkKey(t, AdobeAlgorithm, uuid, "a1b0d67e2e814df59e67a64cbe366809", 1024)
	if _, err := Deobfuscate(goregular.TTF, AdobeAlgorithm, "isbn:9780000000000"); err == nil {
		t.Errorf("an identifier without a UUID: no error")
	}
}

func TestDeobfuscateRoundTrip(t *testing.T) {
	for _, algorithm := range []string{IDPFAlgorithm, AdobeAlgorithm} {
		obfuscated, err := Deobfuscate(goregular.TTF, algorithm, uuid)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(obfuscated, goregular.TTF) {
			t.Fatalf("%s: not obfuscated", algorithm)
		}
		plain, err := Deobfuscate(obfuscated, algorithm, uuid)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(plain, goregular.TTF) {
			t.Errorf("%s: the round trip changed the font", algorithm)
		}
	}
	if _, err := Deobfuscate(goregular.TTF, "http://www.w3.org/2001/04/xmlenc#aes128-cbc", uuid); !errors.Is(err, ErrUnknownAlgorithm) {
		t.Errorf("AES: %v, want ErrUnknownAlgorithm", err)
	}
}

func TestRecoverAdobe(t *testing.T) {
	katex, err := os.ReadFile("testdata/KaTeX_SansSerif-Regular.woff2")
	if err != nil {
		t.Fatal(err)
	}
	katexSFNT, err := ToSFNT(katex)
	if err != nil {
		t.Fatal(err)
	}
	for name, font := range map[string][]byte{"Go Regular": goregular.TTF, "Go Bold": gobold.TTF, "KaTeX": katexSFNT} {
		obfuscated, err := Deobfuscate(font, AdobeAlgorithm, uuid)
		if err != nil {
			t.Fatal(err)
		}
		plain, err := RecoverAdobe(obfuscated)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !bytes.Equal(plain, font) {
			t.Errorf("%s: the recovered font differs from the original", name)
		}
	}
	for name, junk := range map[string][]byte{"short": goregular.TTF[:20], "junk": bytes.Repeat([]byte("junk"), 500)} {
		if _, err := RecoverAdobe(junk); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
