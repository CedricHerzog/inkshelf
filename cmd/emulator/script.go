package main

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook/script"
)

// scriptTarget is the emulator as a script's target. The emulator's network lives in this
// command (wifiOff), so turning it on or off is added here.
type scriptTarget struct {
	*pocketbook.Emulator
}

// SetNetwork turns the emulated network on or off.
func (target scriptTarget) SetNetwork(up bool) { setNetwork(target.Emulator, up) }

// loadScript reads and parses the script at path. It returns no steps if path is empty.
func loadScript(path string) ([]script.Step, error) {
	if path == "" {
		return nil, nil
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	steps, err := script.Parse(string(source))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return steps, nil
}

// runScript runs steps on emulator, saving shots into outDir as NAME.png.
func runScript(emulator *pocketbook.Emulator, work script.BackgroundWork, steps []script.Step, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	return script.Run(scriptTarget{emulator}, work, steps, func(name string, panel *image.Gray) error {
		return savePNG(filepath.Join(outDir, name+".png"), panel)
	})
}

// savePNG writes img to path as a PNG file.
func savePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, img); err != nil {
		// The encoding error is the one to report: the file is incomplete anyway.
		_ = file.Close()
		return err
	}
	log.Printf("script: saved %s", path)
	return file.Close()
}
