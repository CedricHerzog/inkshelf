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
// The InkPad 3's firmware 6.x has Noto CJK of its own, which the program prefers; these are
// its fallback, for a firmware without them, and for the emulator.
//
//go:embed cjk/NotoSansCJKjp-Regular.otf.br cjk/NotoSerifCJKjp-Regular.otf.br cjk/sizes.txt cjk/OFL.txt
var cjkFiles embed.FS

// cjkFont is a CJK font, parsed the first time a character needs it: the firmware's, a font of
// its collection firmware, if it has it, else the program's, name in cjkFiles, decompressed
// (6 seconds on the reader, once, as the decompressed font is kept).
type cjkFont struct {
	firmware, family string
	name             string
	once             sync.Once
	file             *File // nil if neither could be read and parsed
}

// The CJK fonts: Noto Sans CJK and Noto Serif CJK, in their Japanese forms, which have
// Chinese and Korean characters too.
var (
	cjkSans  = &cjkFont{firmware: "NotoSansCJK-Regular.ttc", family: "Noto Sans CJK JP", name: "NotoSansCJKjp-Regular.otf"}
	cjkSerif = &cjkFont{firmware: "NotoSerifCJK-Regular.ttc", family: "Noto Serif CJK JP", name: "NotoSerifCJKjp-Regular.otf"}
)

// cacheDir is where the CJK fonts are kept decompressed, "" to decompress them in memory on
// every start. SetCacheDir sets it.
var cacheDir string

// SetCacheDir makes the CJK fonts decompress once into dir/fonts, and later starts read them
// from there. Call it at start, before any font is opened. Without it, they are decompressed
// in memory each time the program needs them, which tests and tools can afford.
func SetCacheDir(dir string) { cacheDir = dir }

// parsed returns the font parsed, the firmware's or else the program's unpacked, on first use, or
// nil if neither can be: the characters it would draw stay boxes.
func (font *cjkFont) parsed() *File {
	font.once.Do(func() {
		if font.file = font.firmwareFont(); font.file != nil {
			return
		}
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

// firmwareFont returns the firmware's font, the first of fontDirs that has its collection, or
// nil.
func (font *cjkFont) firmwareFont() *File {
	span := timing.Start(context.Background(), "read firmware font")
	defer span.Log("%s", font.firmware)
	for _, dir := range fontDirs {
		path := filepath.Join(dir, font.firmware)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		file, err := parseCollectionFace(path, data, font.family)
		if err != nil {
			log.Printf("fonts: %v", err)
			continue
		}
		return file
	}
	return nil
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
