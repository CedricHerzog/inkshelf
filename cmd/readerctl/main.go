// Command readerctl drives a reader that runs the app under remote control, over Wi-Fi: it runs
// scripts on it, saves its panel and its shots, follows its log and fetches its profiles
// (docs/reference/remote-control.md).
//
//	readerctl shot [FILE]          save the panel (default build/reader-shots/panel.png)
//	readerctl script FILE          run a script, its shots into build/reader-shots/<script>/
//	readerctl log [-f]             print the log; -f prints what comes next, until interrupted
//	readerctl profile              fetch the profiles into build/reader-profile/
//	readerctl deploy               replace the app with build/inkshelf.app, and wait for it to restart
//
// The reader's address and token come from emulator/reader.json ({"address": "host:8740",
// "token": "..."}), or from READER and READER_TOKEN.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Where readerctl reads its settings and saves what it fetches.
const (
	configPath = "emulator/reader.json"
	shotsDir   = "build/reader-shots"
	profileDir = "build/reader-profile"
	appBuild   = "build/inkshelf.app"
)

// How long a request may take, but for a script, which takes as long as its steps; how often
// log -f asks for more; how long deploy waits for the new app to answer.
const (
	requestTimeout = 30 * time.Second
	followInterval = time.Second
	restartTimeout = time.Minute
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("readerctl: ")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: readerctl shot [FILE] | script FILE | log [-f] | profile | deploy")
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	reader, err := loadReader()
	if err != nil {
		log.Fatal(err)
	}
	if err := run(reader, flag.Arg(0), flag.Args()[1:]); err != nil {
		log.Fatal(err)
	}
}

// run runs a subcommand with its arguments.
func run(reader *reader, command string, arguments []string) error {
	switch command {
	case "shot":
		path := filepath.Join(shotsDir, "panel.png")
		if len(arguments) > 0 {
			path = arguments[0]
		}
		return reader.saveTo(path, "/panel.png", requestTimeout)
	case "script":
		if len(arguments) != 1 {
			return errors.New("usage: readerctl script FILE")
		}
		return reader.runScript(arguments[0])
	case "log":
		follow := flag.NewFlagSet("log", flag.ExitOnError)
		following := follow.Bool("f", false, "print what comes next, until interrupted")
		if err := follow.Parse(arguments); err != nil {
			return err
		}
		return reader.printLog(*following)
	case "profile":
		return reader.fetchProfiles()
	case "deploy":
		return reader.deploy()
	default:
		flag.Usage()
		os.Exit(2)
		return nil
	}
}

// reader is a reader under remote control.
type reader struct {
	Address string `json:"address"`
	Token   string `json:"token"`
}

// loadReader reads the reader's address and token from configPath, or from READER and
// READER_TOKEN, which win.
func loadReader() (*reader, error) {
	var settings reader
	if contents, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(contents, &settings); err != nil {
			return nil, fmt.Errorf("%s: %w", configPath, err)
		}
	}
	if address := os.Getenv("READER"); address != "" {
		settings.Address = address
	}
	if token := os.Getenv("READER_TOKEN"); token != "" {
		settings.Token = token
	}
	if settings.Address == "" || settings.Token == "" {
		return nil, fmt.Errorf("no reader: write %s ({\"address\": \"host:8740\", \"token\": \"...\"}) or set READER and READER_TOKEN", configPath)
	}
	if !strings.Contains(settings.Address, ":") {
		settings.Address += ":8740"
	}
	return &settings, nil
}

// request sends a request to the reader and returns its response, which is an error unless its
// status is OK. timeout 0 means none. Close the body.
func (reader *reader) request(method, path string, body io.Reader, timeout time.Duration) (*http.Response, error) {
	request, err := http.NewRequestWithContext(context.Background(), method, "http://"+reader.Address+path, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+reader.Token)
	response, err := (&http.Client{Timeout: timeout}).Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 1<<10))
		response.Body.Close()
		return nil, fmt.Errorf("%s %s: %s: %s", method, path, response.Status, strings.TrimSpace(string(message)))
	}
	return response, nil
}

// saveTo saves what the reader sends at path on the reader to file.
func (reader *reader) saveTo(file, path string, timeout time.Duration) error {
	response, err := reader.request(http.MethodGet, path, nil, timeout)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	output, err := os.Create(file)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, response.Body); err != nil {
		_ = output.Close()
		return err
	}
	fmt.Println(file)
	return output.Close()
}

// scriptResult is the reader's reply to a script.
type scriptResult struct {
	Shots []string `json:"shots"`
	Error string   `json:"error"`
}

// runScript runs the script at path on the reader and saves its shots, even when it failed
// midway, into shotsDir/<script name>/.
func (reader *reader) runScript(path string) error {
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()
	response, err := reader.request(http.MethodPost, "/script", source, 0)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var result scriptResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return err
	}
	outDir := filepath.Join(shotsDir, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	for _, name := range result.Shots {
		if err := reader.saveTo(filepath.Join(outDir, name+".png"), "/shots/"+name+".png", requestTimeout); err != nil {
			return err
		}
	}
	if result.Error != "" {
		return fmt.Errorf("%s: %s", path, result.Error)
	}
	return nil
}

// printLog prints the log. Following, it prints only what comes next, until interrupted.
func (reader *reader) printLog(follow bool) error {
	offset := int64(-1) // follow starts at the end, which the first answer tells
	for {
		start := max(offset, 0)
		response, err := reader.request(http.MethodGet, "/log?offset="+strconv.FormatInt(start, 10), nil, requestTimeout)
		if err != nil {
			if follow && offset >= 0 {
				// The app may be restarting: keep following.
				log.Print(err)
				time.Sleep(followInterval)
				continue
			}
			return err
		}
		sent, _ := strconv.ParseInt(response.Header.Get("X-Log-Offset"), 10, 64)
		size, _ := strconv.ParseInt(response.Header.Get("X-Log-Size"), 10, 64)
		var received int64
		if follow && offset < 0 {
			received, err = io.Copy(io.Discard, response.Body)
		} else {
			received, err = io.Copy(os.Stdout, response.Body)
		}
		response.Body.Close()
		if err != nil {
			return err
		}
		offset = sent + received
		if offset < size {
			continue // more than one chunk
		}
		if !follow {
			return nil
		}
		time.Sleep(followInterval)
	}
}

// fetchProfiles saves every file of the reader's profile directory into profileDir.
func (reader *reader) fetchProfiles() error {
	response, err := reader.request(http.MethodGet, "/profile/", nil, requestTimeout)
	if err != nil {
		return err
	}
	list, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		return err
	}
	names := strings.Fields(string(list))
	if len(names) == 0 {
		return errors.New("no profile on the reader: create applications/inkshelf/profile and restart the app")
	}
	for _, name := range names {
		if err := reader.saveTo(filepath.Join(profileDir, name), "/profile/"+name, 0); err != nil {
			return err
		}
	}
	return nil
}

// deploy uploads appBuild to the reader, which replaces its app with it and restarts, then
// waits until the new app answers: until a reply says the app started at another time.
func (reader *reader) deploy() error {
	program, err := os.Open(appBuild)
	if err != nil {
		return fmt.Errorf("%w: run make build first", err)
	}
	defer program.Close()
	response, err := reader.request(http.MethodPut, "/app", program, restartTimeout)
	if err != nil {
		return err
	}
	response.Body.Close()
	oldStart := response.Header.Get("X-Started")
	fmt.Println("uploaded, restarting")
	deadline := time.Now().Add(restartTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(followInterval)
		response, err := reader.request(http.MethodGet, "/status", nil, requestTimeout)
		if err != nil {
			continue // closing or starting
		}
		response.Body.Close()
		if response.Header.Get("X-Started") != oldStart {
			fmt.Println("the new app answers")
			return nil
		}
	}
	return fmt.Errorf("the new app did not answer within %v: start it on the reader, and read its log over USB", restartTimeout)
}
