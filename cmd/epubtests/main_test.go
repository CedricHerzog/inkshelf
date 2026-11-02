package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/epubtest"
)

// writeSuite writes a suite of two tests as w3c/epub-tests keeps them, each an unzipped EPUB
// in a folder of tests/: a reflowable book and a fixed-layout one.
func writeSuite(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	books := map[string]epubtest.Options{
		"reflowable-text": {Chapters: 2, Paragraphs: 3, Words: 20},
		"fxl-pages":       {Chapters: 2, FixedLayout: true},
		"xx-template":     {Chapters: 1, Paragraphs: 1, Words: 5}, // the suite's templates are not tests
	}
	for name, bookOptions := range books {
		files := epubtest.Files(t, bookOptions)
		files["mimetype"] = "application/epub+zip"
		for path, content := range files {
			full := filepath.Join(root, "tests", name, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func TestRunDrawsEachTestAndReportsResults(t *testing.T) {
	suite := writeSuite(t)
	out := filepath.Join(t.TempDir(), "out")
	judgments := filepath.Join(t.TempDir(), "judgments.tsv")
	verdicts := "# test\tarea\tverdict\treason\nreflowable-text\tSpine\tpass\t\nfxl-pages\tFixed layout\tfail\tno spreads\n"
	if err := os.WriteFile(judgments, []byte(verdicts), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := options{tests: suite, out: out, judgments: judgments, fontsDir: "../../emulator/fonts", pages: 2}

	var report bytes.Buffer
	if err := run(opts, &report); err != nil {
		t.Fatal(err)
	}
	words, err := os.ReadFile(filepath.Join(out, "reflowable-text", "words.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(words), "page 1") || strings.HasPrefix(string(words), "error") {
		t.Errorf("reflowable words.txt = %q; want the laid-out text of its pages", words)
	}
	for _, name := range []string{"reflowable-text/page-1.png", "fxl-pages/page-1.png", "fxl-pages/page-2.png"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("%s was not drawn: %v", name, err)
		}
	}
	for _, want := range []string{"2 tests read", "| Fixed layout | 1 | 0 | 1 | 0 |", "| **Total** | **2** | **1** | **1** | **0** |", "Of the 2 tests in scope, 1 pass"} {
		if !strings.Contains(report.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, report.String())
		}
	}

	// A second run over the first's output sees nothing changed: the drawing is deterministic.
	opts.previous, opts.out = out, filepath.Join(t.TempDir(), "again")
	report.Reset()
	if err := run(opts, &report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.String(), "0 changed since") {
		t.Errorf("a second run reported changes:\n%s", report.String())
	}
}

func TestRunListsTestsWithoutVerdict(t *testing.T) {
	opts := options{tests: writeSuite(t), out: t.TempDir(), judgments: filepath.Join(t.TempDir(), "none.tsv"), fontsDir: "../../emulator/fonts", pages: 1}
	var report bytes.Buffer
	if err := run(opts, &report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.String(), "2 tests have no verdict") {
		t.Errorf("report does not list the unjudged tests:\n%s", report.String())
	}
}

func TestReadJudgmentsRejectsUnknownVerdict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "judgments.tsv")
	if err := os.WriteFile(path, []byte("some-test\tSpine\tmaybe\twho knows\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readJudgments(path); err == nil {
		t.Error("a verdict other than pass, fail, out-of-scope or deprecated was accepted")
	}
}
