package fonts

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"

	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// cjkFiles are the CJK fonts compiled into the program, compressed with Brotli (make
// cjk-fonts), with sizes.txt, the size of each once decompressed, and their license, OFL.txt.
// The firmware has no font with Chinese, Japanese or Korean characters, and the app is copied
// to the reader as one file, so it carries its own.
//
//go:embed cjk/NotoSansCJKjp-Regular.otf.br cjk/NotoSerifCJKjp-Regular.otf.br cjk/sizes.txt cjk/OFL.txt
var cjkFiles embed.FS

// cjkFont is one of the CJK fonts of cjkFiles, decompressed and parsed the first time a
// character needs it: decompressing takes about a second on the reader, and the font is large.
type cjkFont struct {
	name string
	once sync.Once
	file *File // nil if it could not be unpacked or parsed
}

// The CJK fonts: Noto Sans CJK and Noto Serif CJK, in their Japanese forms, which have
// Chinese and Korean characters too.
var (
	cjkSans  = &cjkFont{name: "NotoSansCJKjp-Regular.otf"}
	cjkSerif = &cjkFont{name: "NotoSerifCJKjp-Regular.otf"}
)

// cacheDir is where the CJK fonts are kept decompressed, "" to decompress them in memory on
// every start. SetCacheDir sets it.
var cacheDir string

// SetCacheDir makes the CJK fonts decompress once into dir/fonts, and later starts read them
// from there. Call it at start, before any font is opened. Without it, they are decompressed
// in memory each time the program needs them, which tests and tools can afford.
func SetCacheDir(dir string) { cacheDir = dir }

// parsed returns the font parsed, unpacking it on first use, or nil if it cannot be: the
// characters it would draw stay boxes.
func (font *cjkFont) parsed() *File {
	font.once.Do(func() {
		data, err := unpackFont(font.name, cacheDir)
		if err == nil {
			font.file, err = Parse(font.name, data)
		}
		if err != nil {
			log.Printf("fonts: %v", err)
		}
	})
	return font.file
}

// unpackFont returns the CJK font name decompressed: from dir/fonts, where it is written the
// first time, or in memory if dir is "". A cached file whose size is not the font's (written in
// part, or by another version) is written again.
func unpackFont(name, dir string) ([]byte, error) {
	span := timing.Start(context.Background(), "unpack font")
	defer span.Log("%s", name)
	size, err := unpackedSize(name)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "fonts", name)
	if dir != "" {
		if info, err := os.Stat(path); err == nil && info.Size() == int64(size) {
			return os.ReadFile(path)
		}
	}
	compressed, err := cjkFiles.ReadFile("cjk/" + name + ".br")
	if err != nil {
		return nil, err
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(brotli.NewReader(bytes.NewReader(compressed)), data); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if dir != "" {
		// A cache that cannot be written costs the decompression at the next start, no more.
		if err := writeAtomically(path, data); err != nil {
			log.Printf("fonts: %v", err)
		}
	}
	return data, nil
}

// unpackedSize returns the size of the CJK font name decompressed, from sizes.txt.
func unpackedSize(name string) (int, error) {
	sizes, err := cjkFiles.ReadFile("cjk/sizes.txt")
	if err != nil {
		return 0, err
	}
	for line := range strings.Lines(string(sizes)) {
		if fileName, size, ok := strings.Cut(strings.TrimSpace(line), " "); ok && fileName == name {
			return strconv.Atoi(size)
		}
	}
	return 0, fmt.Errorf("%s: no size in sizes.txt", name)
}

// writeAtomically writes data to path through a temporary file renamed over it, so that a
// reader switched off while writing leaves no half font where the next start would look.
func writeAtomically(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	_, err = temporary.Write(data)
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temporary.Name(), path)
	}
	if err != nil {
		os.Remove(temporary.Name())
	}
	return err
}
