// Command inkshelf reads the books of Komga and Calibre-Web servers and the recipes of Mealie
// servers on PocketBook e-readers.
package main

import (
	"log"
	"os"
	"runtime"

	"github.com/CedricHerzog/inkshelf/internal/app"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/servers/httpclient"
	"github.com/CedricHerzog/inkshelf/internal/servers/registry"
)

// appDir holds config.json, copied over USB, and the downloaded books.
const appDir = "/mnt/ext1/applications/inkshelf"

func main() {
	if err := setupLogging(); err != nil {
		log.Printf("logging to %s disabled: %v", logPath, err)
	}
	log.Printf("inkshelf starting (%s, pid %d, %d cores)", runtime.Version(), os.Getpid(), runtime.NumCPU())

	stopProfiling := startProfiling()
	application := app.New(appDir, registry.Opener(httpclient.New()), registry.KindOf, app.Options{})
	// Run returns the error that stopped loading, already logged and shown.
	err := pocketbook.Run(application)
	stopProfiling()
	if err != nil {
		os.Exit(1)
	}
}
