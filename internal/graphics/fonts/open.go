package fonts

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/gofont/goregular"
)

// firmwareFontDir holds the firmware's fonts, which every PocketBook app may use.
const firmwareFontDir = "/ebrmain/fonts"

// Family is a family of fonts: sans serif (the app's), serif (for book text) or monospace.
type Family int

// The families.
const (
	SansSerif Family = iota
	Serif
	Monospace
)

// Style is the weight and slant of a font.
type Style int

// The font styles.
const (
	Regular Style = iota
	Bold
	Italic
	BoldItalic
)

// fontFiles are the firmware's font files of each family and style, tried in order:
// libinkview's defaults (Liberation) first, then DejaVu. Open falls back to Go's fonts if none
// of them opens.
var fontFiles = map[Family]map[Style][]string{
	SansSerif: {
		Regular:    {"LiberationSans.ttf", "LiberationSans-Regular.ttf", "DejaVuSans.ttf"},
		Bold:       {"LiberationSans-Bold.ttf", "DejaVuSans-Bold.ttf"},
		Italic:     {"LiberationSans-Italic.ttf", "DejaVuSans-Oblique.ttf"},
		BoldItalic: {"LiberationSans-BoldItalic.ttf", "DejaVuSans-BoldOblique.ttf"},
	},
	Serif: {
		Regular:    {"LiberationSerif.ttf", "LiberationSerif-Regular.ttf", "DejaVuSerif.ttf"},
		Bold:       {"LiberationSerif-Bold.ttf", "DejaVuSerif-Bold.ttf"},
		Italic:     {"LiberationSerif-Italic.ttf", "DejaVuSerif-Italic.ttf"},
		BoldItalic: {"LiberationSerif-BoldItalic.ttf", "DejaVuSerif-BoldItalic.ttf"},
	},
	Monospace: {
		Regular:    {"LiberationMono.ttf", "LiberationMono-Regular.ttf", "DejaVuSansMono.ttf"},
		Bold:       {"LiberationMono-Bold.ttf", "DejaVuSansMono-Bold.ttf"},
		Italic:     {"LiberationMono-Italic.ttf", "DejaVuSansMono-Oblique.ttf"},
		BoldItalic: {"LiberationMono-BoldItalic.ttf", "DejaVuSansMono-BoldOblique.ttf"},
	},
}

// fallbackFiles are the firmware's font files of each style with the most characters, DejaVu
// Sans's: OpenFallbacks draws with them the characters other fonts lack.
var fallbackFiles = map[Style][]string{
	Regular:    {"DejaVuSans.ttf"},
	Bold:       {"DejaVuSans-Bold.ttf"},
	Italic:     {"DejaVuSans-Oblique.ttf"},
	BoldItalic: {"DejaVuSans-BoldOblique.ttf"},
}

// arabicFiles are the firmware's Arabic font of each style, which has the rules that join Arabic
// letters, as DejaVu Sans and Liberation do not. It has no italic.
var arabicFiles = map[Style][]string{
	Regular:    {"NotoSansArabicUI-Regular.ttf"},
	Bold:       {"NotoSansArabicUI-Bold.ttf"},
	Italic:     {"NotoSansArabicUI-Regular.ttf"},
	BoldItalic: {"NotoSansArabicUI-Bold.ttf"},
}

// goFonts are Go's fonts of each style, compiled into the program: the last resort, which
// always opens. Go has no serif font, nor Arabic or Hebrew letters.
var goFonts = map[Style][]byte{
	Regular:    goregular.TTF,
	Bold:       gobold.TTF,
	Italic:     goitalic.TTF,
	BoldItalic: gobolditalic.TTF,
}

// goMonoFonts are Go Mono's fonts of each style, the fallback of Monospace.
var goMonoFonts = map[Style][]byte{
	Regular:    gomono.TTF,
	Bold:       gomonobold.TTF,
	Italic:     gomonoitalic.TTF,
	BoldItalic: gomonobolditalic.TTF,
}

// fontDirs are where Open looks for fontFiles, in order. AddDir changes it at start, before
// any font is opened.
var fontDirs = []string{firmwareFontDir}

// AddDir makes Open look for the firmware's fonts in dir first. It is for running off the
// reader, where /ebrmain/fonts doesn't exist: the emulator keeps copies of the firmware's fonts
// so that its pages look as the reader's do.
func AddDir(dir string) {
	fontDirs = append([]string{dir}, fontDirs...)
}

// FirmwareFonts returns the names of the firmware's font files, for the log: which fonts a
// reader has varies with its firmware.
func FirmwareFonts() ([]string, error) {
	entries, err := os.ReadDir(firmwareFontDir)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for index, entry := range entries {
		names[index] = entry.Name()
	}
	return names, nil
}

// OpenInterfaceFont opens the default (sans serif) font, or its bold version, size pixels high.
func OpenInterfaceFont(size int, bold bool) (*Font, error) {
	style := Regular
	if bold {
		style = Bold
	}
	return Open(SansSerif, style, size)
}

// Open opens the firmware's font of family and style, size pixels high, or Go's font of
// that style (Go Mono's for Monospace) if none opens.
func Open(family Family, style Style, size int) (*Font, error) {
	if opened := openFirmwareFont(fontFiles[family][style], size); opened != nil {
		return opened, nil
	}
	if family == Monospace {
		return newFont("Go Mono", style, goMonoFonts[style], size)
	}
	return newFont("Go", style, goFonts[style], size)
}

// OpenFallbacks opens the fonts to try, in order, for the characters a font of style lacks, size
// pixels high: the firmware's DejaVu Sans and its Arabic font, those that open, then a CJK font
// (Noto Serif CJK for a serif text, Noto Sans CJK otherwise) for Chinese, Japanese and Korean,
// then Go's font. The CJK fonts have one weight: their bold is the regular emboldened.
func OpenFallbacks(style Style, size int, serif bool) ([]*Font, error) {
	var fallbacks []*Font
	for _, files := range []map[Style][]string{fallbackFiles, arabicFiles} {
		if opened := openFirmwareFont(files[style], size); opened != nil {
			fallbacks = append(fallbacks, opened)
		}
	}
	cjk := cjkSans
	if serif {
		cjk = cjkSerif
	}
	if file := cjk.parsed(); file != nil {
		opened, err := file.open(size, style == Bold || style == BoldItalic)
		if err == nil {
			fallbacks = append(fallbacks, opened)
		}
	}
	opened, err := newFont("Go", style, goFonts[style], size)
	if err != nil {
		for _, fallback := range fallbacks {
			fallback.Close()
		}
		return nil, err
	}
	return append(fallbacks, opened), nil
}

// openFirmwareFont opens the first of the font files names that fontDirs hold, size pixels
// high, or returns nil if none opens.
func openFirmwareFont(names []string, size int) *Font {
	for _, dir := range fontDirs {
		for _, name := range names {
			// A font that is missing or doesn't parse is skipped: the next one will do.
			path := filepath.Join(dir, name)
			file, err := parsedFile(path, path, nil)
			if err != nil {
				continue
			}
			if opened, err := file.Open(size); err == nil {
				return opened
			}
		}
	}
	return nil
}

// newFont returns the TrueType or OpenType font data of a style, named name, at size pixels high.
// It is parsed once for all sizes.
func newFont(name string, style Style, data []byte, size int) (*Font, error) {
	file, err := parsedFile(fmt.Sprint(name, " ", style), name, data)
	if err != nil {
		return nil, err
	}
	return file.Open(size)
}

// parsedFiles are the font files parsed so far, by path, or by name and style for Go's fonts, with the
// error of those that could not be read or parsed. A book's page opens its fonts at several
// sizes (headings, notes, small capitals), and reading and parsing a file again for each took
// longer than opening the size.
var parsedFiles = struct {
	sync.Mutex
	files  map[string]*File
	errors map[string]error
}{files: map[string]*File{}, errors: map[string]error{}}

// parsedFile returns the font file named name parsed, from key in parsedFiles: data if it is not
// nil, else the file at path name, read the first time.
func parsedFile(key, name string, data []byte) (*File, error) {
	parsedFiles.Lock()
	defer parsedFiles.Unlock()
	if file, ok := parsedFiles.files[key]; ok {
		return file, nil
	}
	if err, ok := parsedFiles.errors[key]; ok {
		return nil, err
	}
	var err error
	if data == nil {
		data, err = os.ReadFile(name)
	}
	var file *File
	if err == nil {
		file, err = Parse(name, data)
	}
	if err != nil {
		parsedFiles.errors[key] = err
		return nil, err
	}
	parsedFiles.files[key] = file
	return file, nil
}
