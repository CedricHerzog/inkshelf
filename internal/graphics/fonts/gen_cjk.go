//go:build ignore

// gen_cjk compresses the Noto CJK fonts that make cjk-fonts downloads into the files that
// cjk.go embeds: each font compressed with Brotli at its highest quality, and sizes.txt, the
// size of each font once decompressed, which tells a cached copy that is whole from one that
// is not. Run it from the repository root: go run ./internal/graphics/fonts/gen_cjk.go
// build/cjk internal/graphics/fonts/cjk.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/andybalholm/brotli"
)

// fontNames are the fonts compressed, as cjk.go names them.
var fontNames = []string{"NotoSansCJKjp-Regular.otf", "NotoSerifCJKjp-Regular.otf"}

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: gen_cjk <directory of the .otf files> <directory of the .br files>")
	}
	source, destination := os.Args[1], os.Args[2]
	var sizes strings.Builder
	for _, name := range fontNames {
		data, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			log.Fatal(err)
		}
		if err := compress(data, filepath.Join(destination, name+".br")); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(&sizes, "%s %d\n", name, len(data))
	}
	if err := os.WriteFile(filepath.Join(destination, "sizes.txt"), []byte(sizes.String()), 0o644); err != nil {
		log.Fatal(err)
	}
}

// compress writes data compressed with Brotli at quality 11, the slowest and smallest: it is
// done once, and only decompressing runs on the reader.
func compress(data []byte, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := brotli.NewWriterOptions(file, brotli.WriterOptions{Quality: brotli.BestCompression, LGWin: 24})
	if _, err := writer.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
