// Command emulator runs the app on the host, on an emulated PocketBook: the real event loop
// and screens, with the e-ink panel kept in memory. It runs in one of two ways:
//
//   - Interactive: the panel is served as a web page (by default at http://localhost:8740),
//     where clicks are touches and keys are the reader's buttons.
//   - Headless, with -script: a script of taps, keys and waits drives the app, and saves
//     screenshots of the panel (see script.go for its commands). This is how UI changes are
//     checked: make shots-all runs every script of emulator/scripts.
//
// The app reads its servers from DIR/config.json, as on the reader, or with -fake from
// built-in fake Komga, Mealie or Calibre-Web servers whose content never changes. With
// -frozen-status, the clock, the free space and the fake servers' addresses are fixed as well,
// so that two runs of a script give byte-identical screenshots.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/app"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/calibreweb/fakecalibreweb"
	"github.com/CedricHerzog/inkshelf/internal/servers/httpclient"
	"github.com/CedricHerzog/inkshelf/internal/servers/komga/fakekomga"
	"github.com/CedricHerzog/inkshelf/internal/servers/mealie/fakemealie"
	"github.com/CedricHerzog/inkshelf/internal/servers/registry"
	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// options are the command's flags.
type options struct {
	// dir holds the emulated reader's files: config.json, servers/ and fonts/.
	dir string
	// fake runs built-in fake servers, with their files in dir/fake.
	fake bool
	// servers is the number of fake servers.
	servers int
	// fakeTypes are the kinds of the fake servers, "komga", "mealie" or "calibreweb",
	// separated by commas: server i is of the kind i modulo their number.
	fakeTypes string
	// fresh starts with no downloaded books and no saved filters.
	fresh bool
	// noWiFi starts with the emulated Wi-Fi off: the servers cannot be reached until the
	// network comes up.
	noWiFi bool
	// dark starts with the emulated reader's dark mode on.
	dark bool
	// size is the screen size, as WIDTHxHEIGHT.
	size string
	// eink makes refreshes take as long as on an InkPad 3.
	eink bool
	// address is where the web page is served.
	address string
	// scriptPath is a script to run headless instead of serving the web page.
	scriptPath string
	// outDir is where the script's shots go.
	outDir string
	// frozenStatus shows a fixed time and free space, and gives the fake servers fixed
	// addresses, so that shots are the same on every run.
	frozenStatus bool
	// profileDir, if set, gets the CPU profile, the execution trace and the other profiles of
	// the run (docs/reference/profiling.md).
	profileDir string
}

func main() {
	var opts options
	flag.StringVar(&opts.dir, "dir", "emulator", "the emulated reader's files: config.json, servers/, fonts/")
	flag.BoolVar(&opts.fake, "fake", false, "use built-in fake servers (see -fake-type), with their files in DIR/fake")
	flag.IntVar(&opts.servers, "servers", 1, "with -fake, the number of fake servers to choose from")
	flag.StringVar(&opts.fakeTypes, "fake-type", "komga", "with -fake, the kind of fake server: komga, mealie or calibreweb, or several separated by commas, used in turn")
	flag.BoolVar(&opts.fresh, "fresh", false, "start with no downloaded books and no saved filters")
	flag.BoolVar(&opts.noWiFi, "no-wifi", false, "start with the Wi-Fi off, until the network comes up (net up)")
	flag.BoolVar(&opts.dark, "dark", false, "start with the reader's dark mode on (dark on|off in scripts)")
	flag.StringVar(&opts.size, "size", "1404x1872", "screen size in pixels (portrait)")
	flag.BoolVar(&opts.eink, "eink", false, "make refreshes take as long as on an InkPad 3")
	flag.StringVar(&opts.address, "addr", "localhost:8740", "address of the web page")
	flag.StringVar(&opts.scriptPath, "script", "", "run this script headless instead of serving the web page")
	flag.StringVar(&opts.outDir, "out", "build/shots", "where the script's shots go")
	flag.BoolVar(&opts.frozenStatus, "frozen-status", false, "show a fixed time and free space, and fixed addresses for the fake servers, so that shots are the same on every run")
	flag.StringVar(&opts.profileDir, "profile", "", "write the run's CPU profile, execution trace and other profiles to this directory")
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)
	if err := run(opts); err != nil {
		log.Printf("emu: %v", err)
		os.Exit(1)
	}
}

// run runs the app on an emulated reader until it exits, driven by the script if there is
// one, else by the web page.
func run(opts options) error {
	if opts.profileDir != "" {
		stopProfiling, err := timing.Capture(opts.profileDir, 0)
		if err != nil {
			return err
		}
		defer stopProfiling()
	}
	size, err := parseSize(opts.size)
	if err != nil {
		return err
	}
	steps, err := loadScript(opts.scriptPath)
	if err != nil {
		return err
	}
	appDir, addresses, stopServers, err := prepareAppDir(opts)
	defer stopServers()
	if err != nil {
		return err
	}
	fonts.AddDir(filepath.Join(opts.dir, "fonts"))

	emulator := pocketbook.NewEmulator(size, opts.eink)
	emulator.SetDarkMode(opts.dark)
	wifiOff.Store(opts.noWiFi)
	application := app.New(appDir, registry.Opener(redirectingClient(addresses)), registry.KindOf, withEmulatedWiFi(appOptions(opts)))
	if steps == nil {
		if err := serveWebPage(opts.address, emulator); err != nil {
			return err
		}
		return pocketbook.RunEmulated(application, emulator)
	}

	scriptDone := make(chan error, 1)
	go func() {
		scriptDone <- runScript(emulator, application.BackgroundWork, steps, opts.outDir)
		pocketbook.Exit()
	}()
	appErr := pocketbook.RunEmulated(application, emulator)
	if err := <-scriptDone; err != nil {
		return fmt.Errorf("%s: %w", opts.scriptPath, err)
	}
	return appErr
}

// The status that -frozen-status shows.
var (
	frozenTime      = time.Date(2025, time.March, 14, 9, 41, 0, 0, time.UTC)
	frozenFreeSpace = int64(12) << 30
)

// appOptions are the app's options for opts: the real clock and free space, unless frozen.
func appOptions(opts options) app.Options {
	if !opts.frozenStatus {
		return app.Options{}
	}
	return app.Options{
		Now:       func() time.Time { return frozenTime },
		FreeSpace: func(string) (int64, bool) { return frozenFreeSpace, true },
	}
}

// parseSize parses a screen size written WIDTHxHEIGHT.
func parseSize(text string) (image.Point, error) {
	var size image.Point
	if _, err := fmt.Sscanf(text, "%dx%d", &size.X, &size.Y); err != nil || size.X <= 0 || size.Y <= 0 {
		return image.Point{}, fmt.Errorf("bad -size %q, want WIDTHxHEIGHT", text)
	}
	return size, nil
}

// prepareAppDir returns the directory the app keeps its files in, after starting the fake
// servers that opts asks for and clearing the saved files if opts asks for a fresh
// start. addresses are the real addresses of the fake servers that config.json gives fixed
// ones. stopServers stops the fake servers; call it even if there is an error.
func prepareAppDir(opts options) (appDir string, addresses map[string]string, stopServers func(), err error) {
	appDir = opts.dir
	stopServers = func() {}
	if opts.fake {
		appDir = filepath.Join(opts.dir, "fake")
		var configs []servers.Config
		configs, addresses, stopServers, err = startFakeServers(strings.Split(opts.fakeTypes, ","), max(opts.servers, 1), opts.frozenStatus)
		if err != nil {
			return "", nil, stopServers, err
		}
		if err := writeConfig(appDir, configs); err != nil {
			return "", nil, stopServers, err
		}
	} else if _, err := os.Stat(filepath.Join(opts.dir, "config.json")); err != nil {
		log.Printf("no %s: write it as on the reader ([{\"name\": \"…\", \"server\": \"http://…\", \"apiKey\": \"…\"}]), or use -fake",
			filepath.Join(opts.dir, "config.json"))
	}
	if opts.fresh {
		for _, name := range []string{"servers", "books", "filters.json"} {
			if err := os.RemoveAll(filepath.Join(appDir, name)); err != nil {
				return "", nil, stopServers, err
			}
		}
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", nil, stopServers, err
	}
	return appDir, addresses, stopServers, nil
}

// fakeHandlers make the handler of a fake server of each kind.
var fakeHandlers = map[string]func() http.Handler{
	"komga":      fakekomga.NewHandler,
	"mealie":     fakemealie.NewHandler,
	"calibreweb": fakecalibreweb.NewHandler,
}

// frozenFirstPort is the port of the first fake server's address with -frozen-status, the
// next ones following: the server screen shows their addresses. The servers listen on any free
// port, so that runs side by side never compete for one; the app's HTTP client dials the real
// port when it is asked for the fixed one (see redirectingClient).
const frozenFirstPort = 18741

// startFakeServers starts count fake servers, of serverTypes in turn, and returns their configs
// and a function that stops them all. With frozen, the configs give the servers fixed
// addresses, and addresses maps each to the server's real one.
func startFakeServers(serverTypes []string, count int, frozen bool) (configs []servers.Config, addresses map[string]string, stopServers func(), err error) {
	for _, serverType := range serverTypes {
		if _, ok := fakeHandlers[serverType]; !ok {
			return nil, nil, func() {}, fmt.Errorf("no fake server of type %q", serverType)
		}
	}
	var fakeServers []*httptest.Server
	stopServers = func() {
		// The last server started stops first, as deferred calls in a loop would.
		for _, server := range slices.Backward(fakeServers) {
			server.Close()
		}
	}
	addresses = map[string]string{}
	for index := range count {
		serverType := serverTypes[index%len(serverTypes)]
		server := httptest.NewServer(fakeHandlers[serverType]())
		fakeServers = append(fakeServers, server)
		url := server.URL
		if frozen {
			shown := fmt.Sprintf("127.0.0.1:%d", frozenFirstPort+index)
			addresses[shown] = server.Listener.Addr().String()
			url = "http://" + shown
		}
		name := "Fake"
		if index > 0 {
			name = fmt.Sprintf("Fake %d", index+1)
		}
		configs = append(configs, servers.Config{Name: name, Type: serverType, Server: url, APIKey: "fake", Username: "fake", Password: "fake"})
		log.Printf("fake %s %q at %s (%s)", serverType, name, url, server.URL)
	}
	return configs, addresses, stopServers, nil
}

// redirectingClient returns the app's HTTP client, which connects to the real address of
// addresses when asked for one of their fixed ones, and to nothing while the emulated Wi-Fi is
// off.
func redirectingClient(addresses map[string]string) *http.Client {
	client := httpclient.New()
	transport := client.Transport.(*http.Transport)
	var dialer net.Dialer
	transport.DialContext = dialUnlessOff(func(ctx context.Context, network, address string) (net.Conn, error) {
		if target, ok := addresses[address]; ok {
			address = target
		}
		return dialer.DialContext(ctx, network, address)
	})
	return client
}

// serveWebPage serves the emulator's web page at address, in the background.
func serveWebPage(address string, emulator *pocketbook.Emulator) error {
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", address)
	if err != nil {
		return err
	}
	go func() {
		server := &http.Server{Handler: newServer(emulator), ReadHeaderTimeout: 10 * time.Second}
		if err := server.Serve(listener); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("web page: %v", err)
		}
	}()
	log.Printf("the reader is at http://%s", listener.Addr())
	return nil
}

// writeConfig writes dir/config.json, as copied to the reader over USB.
func writeConfig(dir string, configs []servers.Config) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(configs, "", "  ") //nolint:gosec // G117: config.json holds the API keys, as on the reader
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0o644)
}
