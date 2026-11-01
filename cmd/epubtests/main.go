package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/app/openbook"
	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/reading"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
)

// screenSize is the InkPad 3's screen, in portrait, which the tests are drawn for.
var screenSize = image.Pt(1404, 1872)

// options are the command's flags.
type options struct {
	tests, out, previous, judgments, fontsDir string
	pages                                     int
	verbose                                   bool
}

func main() {
	var opts options
	flag.StringVar(&opts.tests, "tests", "", "a checkout of w3c/epub-tests, or its tests folder")
	flag.StringVar(&opts.out, "out", "build/epub-tests", "where each test's words.txt and page-N.png go")
	flag.StringVar(&opts.previous, "previous", "", "the output of the previous run, to list the tests that changed")
	flag.StringVar(&opts.judgments, "judgments", "cmd/epubtests/judgments.tsv", "the verdicts so far: test, area, verdict, reason")
	flag.StringVar(&opts.fontsDir, "fonts", "emulator/fonts", "fonts to add to the firmware's, as the emulator does")
	flag.IntVar(&opts.pages, "pages", 3, "how many pages of each test to draw")
	flag.BoolVar(&opts.verbose, "v", false, "keep the app's log, which says what each chapter and page cost")
	flag.Parse()
	if err := run(opts, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "epubtests:", err)
		os.Exit(1)
	}
}

// run reads every test, then reports what changed since the previous run and the results by
// area.
func run(opts options, report io.Writer) error {
	if opts.tests == "" {
		return errors.New("-tests is required: a checkout of https://github.com/w3c/epub-tests")
	}
	if !opts.verbose {
		log.SetOutput(io.Discard)
	}
	fonts.AddDir(opts.fontsDir)
	tests, err := findTests(opts.tests)
	if err != nil {
		return err
	}
	if len(tests) == 0 {
		return fmt.Errorf("no test found in %s", opts.tests)
	}
	work, err := os.MkdirTemp("", "epubtests")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	for _, test := range tests {
		outDir := filepath.Join(opts.out, test.name)
		if err := os.RemoveAll(outDir); err != nil {
			return err
		}
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}
		if err := readTest(test, work, outDir, opts.pages); err != nil {
			// A test that cannot be read is a result too: its words say why.
			if err := os.WriteFile(filepath.Join(outDir, "words.txt"), []byte("error: "+err.Error()+"\n"), 0o644); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(report, "%d tests read into %s\n", len(tests), opts.out)

	if opts.previous != "" {
		changed, err := changedTests(tests, opts.previous, opts.out)
		if err != nil {
			return err
		}
		fmt.Fprintf(report, "\n%d changed since %s, to look at again:\n", len(changed), opts.previous)
		for _, name := range changed {
			fmt.Fprintln(report, " ", name)
		}
	}
	judgments, err := readJudgments(opts.judgments)
	if err != nil {
		return err
	}
	writeResults(report, tests, judgments)
	return nil
}

// test is one folder of the suite: an unzipped EPUB.
type test struct {
	name, dir string
}

// findTests returns the tests under root, sorted by name: the folders that hold a mimetype
// file, in root itself or in its tests folder, as w3c/epub-tests keeps them. The suite's
// xx- folders are templates for writing new tests, not tests, so they are left out.
func findTests(root string) ([]test, error) {
	if info, err := os.Stat(filepath.Join(root, "tests")); err == nil && info.IsDir() {
		root = filepath.Join(root, "tests")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var tests []test
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "xx-") {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "mimetype")); entry.IsDir() && err == nil {
			tests = append(tests, test{name: entry.Name(), dir: dir})
		}
	}
	return tests, nil
}

// readTest zips a test into a book on a shelf in work, as a download leaves it, then draws its
// first pages into outDir as the app would show them.
func readTest(current test, work, outDir string, pages int) error {
	data, err := zipTest(current.dir)
	if err != nil {
		return err
	}
	store := shelf.Store{Dir: filepath.Join(work, current.name)}
	book := shelf.Book{ID: current.name, Title: current.name, Format: servers.Reflowable, Pages: 1}
	epubPath := store.EPUBPath(book.ID)
	if err := os.MkdirAll(filepath.Dir(epubPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(epubPath, data, 0o644); err != nil {
		return err
	}
	opened, err := epub.Open(epubPath)
	if err != nil {
		return err
	}
	if opened.DrawnAsPages() {
		defer opened.Close()
		return drawFixedLayout(opened, outDir, pages)
	}
	opened.Close()
	return drawReflowable(store, book, outDir, pages)
}

// drawFixedLayout draws the first pages of a fixed-layout book or a roll at the screen's size,
// as the download does (package shelf).
func drawFixedLayout(book *epub.Book, outDir string, pages int) error {
	var words strings.Builder
	if book.Roll() {
		fmt.Fprintf(&words, "a roll of %d pages\n", book.PageCount(screenSize))
	}
	for index := range min(pages, book.PageCount(screenSize)) {
		drawn, err := book.RenderPage(index, screenSize)
		if err != nil {
			fmt.Fprintf(&words, "page %d: error: %v\n", index+1, err)
			continue
		}
		fmt.Fprintf(&words, "page %d: drawn whole, %v\n", index+1, drawn.Bounds().Size())
		if err := writePNG(filepath.Join(outDir, fmt.Sprintf("page-%d.png", index+1)), drawn); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(outDir, "words.txt"), []byte(words.String()), 0o644)
}

// drawReflowable lays out a reflowable book with the reader's settings and draws its first
// pages, writing the words of each page as laid out.
func drawReflowable(store shelf.Store, book shelf.Book, outDir string, pages int) error {
	text, err := reading.Open(store, book, readerSettings())
	if err != nil {
		return err
	}
	defer text.Close()
	var words strings.Builder
	at := text.At()
	for number := 1; number <= pages; number++ {
		page, err := text.Page(at)
		if err != nil {
			return err
		}
		fmt.Fprintf(&words, "page %d (chapter %d)\n", number, at.Chapter)
		writeWords(&words, page)
		drawn, err := text.Draw(at)
		if err != nil {
			fmt.Fprintf(&words, "draw error: %v\n", err)
		} else if err := writePNG(filepath.Join(outDir, fmt.Sprintf("page-%d.png", number)), drawn); err != nil {
			return err
		}
		next, ok, err := text.Next(at)
		if err != nil || !ok {
			break
		}
		at = next
	}
	return os.WriteFile(filepath.Join(outDir, "words.txt"), []byte(words.String()), 0o644)
}

// readerSettings are those of package openbook at the default text size, in InkPad 3 pixels,
// which are the screen's here.
func readerSettings() layout.Settings {
	return layout.Settings{
		Width: screenSize.X, Height: screenSize.Y,
		Margins:       image.Rect(openbook.TextMarginSide, openbook.TextMarginTop, openbook.TextMarginSide, openbook.TextMarginBottom),
		FontSize:      openbook.DefaultTextSize,
		LineSpacing:   openbook.TextLineSpacing,
		Justify:       true,
		HideFootnotes: true,
	}
}

// writeWords writes the text of each line of page, its words separated by spaces, and the
// images and formulas it shows.
func writeWords(words *strings.Builder, page layout.Page) {
	for _, line := range page.Lines {
		texts := make([]string, 0, len(line.Words))
		for _, word := range line.Words {
			texts = append(texts, word.Text)
		}
		fmt.Fprintf(words, "  %s\n", strings.Join(texts, " "))
	}
	for _, placed := range page.Images {
		fmt.Fprintf(words, "  [image %s at %v]\n", placed.Path, placed.Rect)
	}
}

// zipTest zips the folder of a test into an EPUB: mimetype first and stored, as OCF requires,
// then every other file. The folder is read through an os.Root, so that a symbolic link in a
// test cannot lead the zip outside it.
func zipTest(dir string) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	files := root.FS()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	mimetype, err := fs.ReadFile(files, "mimetype")
	if err != nil {
		return nil, err
	}
	writer, err := archive.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(mimetype); err != nil {
		return nil, err
	}
	err = fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || name == "mimetype" {
			return err
		}
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		writer, err := archive.Create(name)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// writePNG writes a drawn page.
func writePNG(path string, drawn image.Image) error {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, drawn); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o644)
}

// changedTests returns the tests whose output differs from the previous run's: a file added,
// removed or changed. A test the previous run did not have counts as changed.
func changedTests(tests []test, previous, out string) ([]string, error) {
	var changed []string
	for _, current := range tests {
		same, err := sameDir(filepath.Join(previous, current.name), filepath.Join(out, current.name))
		if err != nil {
			return nil, err
		}
		if !same {
			changed = append(changed, current.name)
		}
	}
	return changed, nil
}

// sameDir reports whether two folders hold the same files with the same bytes. A folder that
// does not exist holds nothing.
func sameDir(before, after string) (bool, error) {
	beforeFiles, err := readFiles(before)
	if err != nil {
		return false, err
	}
	afterFiles, err := readFiles(after)
	if err != nil {
		return false, err
	}
	if len(beforeFiles) != len(afterFiles) {
		return false, nil
	}
	for name, data := range afterFiles {
		if previous, ok := beforeFiles[name]; !ok || !bytes.Equal(previous, data) {
			return false, nil
		}
	}
	return true, nil
}

// readFiles returns the files of a folder by name; none if it does not exist.
func readFiles(dir string) (map[string][]byte, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string][]byte{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		files[entry.Name()] = data
	}
	return files, nil
}

// The verdicts a test may have. A deprecated test is left out of the counts, as
// epub-conformance.md leaves the suite's deprecated tests out.
const (
	verdictPass       = "pass"
	verdictFail       = "fail"
	verdictOutOfScope = "out-of-scope"
	verdictDeprecated = "deprecated"
)

// judgment is what a person decided about a test, after looking at its output.
type judgment struct {
	area, verdict, reason string
}

// readJudgments reads the verdicts file: one test per line, test, area, verdict and reason
// separated by tabs; # starts a comment. A missing file is no verdicts yet.
func readJudgments(path string) (map[string]judgment, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]judgment{}, nil
	}
	if err != nil {
		return nil, err
	}
	judgments := map[string]judgment{}
	for number, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			return nil, fmt.Errorf("%s:%d: want test, area, verdict and reason, separated by tabs", path, number+1)
		}
		verdict := fields[2]
		if !slices.Contains([]string{verdictPass, verdictFail, verdictOutOfScope, verdictDeprecated}, verdict) {
			return nil, fmt.Errorf("%s:%d: verdict %q is not pass, fail, out-of-scope or deprecated", path, number+1, verdict)
		}
		reason := ""
		if len(fields) > 3 {
			reason = fields[3]
		}
		judgments[fields[0]] = judgment{area: fields[1], verdict: verdict, reason: reason}
	}
	return judgments, nil
}

// areaResult counts the verdicts of one area.
type areaResult struct {
	tests, pass, fail, outOfScope int
}

// writeResults prints the table of results by area, in the order the areas first appear in
// the judgments' tests, then the tests that have no verdict yet.
func writeResults(report io.Writer, tests []test, judgments map[string]judgment) {
	results := map[string]*areaResult{}
	var areas, unjudged []string
	var total areaResult
	for _, current := range tests {
		judged, ok := judgments[current.name]
		if !ok {
			unjudged = append(unjudged, current.name)
			continue
		}
		if judged.verdict == verdictDeprecated {
			continue
		}
		result := results[judged.area]
		if result == nil {
			result = &areaResult{}
			results[judged.area] = result
			areas = append(areas, judged.area)
		}
		for _, counts := range []*areaResult{result, &total} {
			counts.tests++
			switch judged.verdict {
			case verdictPass:
				counts.pass++
			case verdictFail:
				counts.fail++
			case verdictOutOfScope:
				counts.outOfScope++
			}
		}
	}
	slices.Sort(areas)
	fmt.Fprintln(report, "\n| Area | Tests | Pass | Fail | Out of scope |")
	fmt.Fprintln(report, "|---|---|---|---|---|")
	for _, area := range areas {
		result := results[area]
		fmt.Fprintf(report, "| %s | %d | %d | %d | %d |\n", area, result.tests, result.pass, result.fail, result.outOfScope)
	}
	fmt.Fprintf(report, "| **Total** | **%d** | **%d** | **%d** | **%d** |\n", total.tests, total.pass, total.fail, total.outOfScope)
	if inScope := total.pass + total.fail; inScope > 0 {
		fmt.Fprintf(report, "\nOf the %d tests in scope, %d pass (%d%%).\n", inScope, total.pass, total.pass*100/inScope)
	}
	if len(unjudged) > 0 {
		fmt.Fprintf(report, "\n%d tests have no verdict in the judgments yet:\n", len(unjudged))
		for _, name := range unjudged {
			fmt.Fprintln(report, " ", name)
		}
	}
}
