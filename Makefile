# Build and deploy Inkshelf, the PocketBook app for Komga, Mealie and Calibre-Web servers.
# Pure Go: it cross-compiles on any host, without cgo or the PocketBook SDK.

APP := build/inkshelf.app

# The reader: InkPad 3 (B288, Cortex-A7), firmware 6.x.
TARGET := CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7

.PHONY: build test lint deploy emu emu-fake shots shots-all shots-compare baseline verify bench bench-baseline \
	bench-compare profile icons docs docs-build docs-shots clean

# Release build: optimised and stripped.
build:
	$(TARGET) go build -trimpath -ldflags="-s -w" -o $(APP) ./cmd/inkshelf

test:
	go test ./...

# golangci-lint (.golangci.yml, brew install golangci-lint), with go vet among its linters: on the
# host, then for the reader, whose _linux_arm.go files the host skips.
lint:
	golangci-lint run ./...
	GOOS=linux GOARCH=arm golangci-lint run ./...

deploy:
	./deploy.sh

# The app on an emulated reader, at http://localhost:8740: against the Komga in emulator/config.json,
# or a built-in fake one. EMUFLAGS adds the emulator's flags (e.g. -eink, -size 1072x1448).
emu:
	go run ./cmd/emulator $(EMUFLAGS)

emu-fake:
	go run ./cmd/emulator -fake $(EMUFLAGS)

# Headless: run SCRIPT against the fake Komga and save its shots in build/shots.
SCRIPT ?= emulator/scripts/smoke.txt
shots:
	rm -rf build/shots
	go run ./cmd/emulator -fake -fresh -script $(SCRIPT) -out build/shots $(EMUFLAGS)

# Every script of emulator/scripts, each against the fake servers it was written for, its shots in
# build/shots/<script>. The status shown is frozen, so that two runs give the same PNGs.
SCRIPTS := $(sort $(wildcard emulator/scripts/*.txt))
shots-all:
	rm -rf build/shots
	go build -o build/emulator ./cmd/emulator
	$(foreach script,$(SCRIPTS),$(call run-script,$(basename $(notdir $(script)))))

# run-script runs emulator/scripts/$(1).txt with the flags of its fake servers.
define run-script
build/emulator -fake -fresh -frozen-status -script emulator/scripts/$(1).txt -out build/shots/$(1) $(call script-flags,$(1))

endef
script-flags = $(if $(filter servers,$(1)),-servers 2,$(if $(filter mealie,$(1)),-fake-type mealie,$(if $(filter calibreweb-%,$(1)),-fake-type calibreweb)))

# The shots of shots-all, compared with those of make baseline: lists the PNGs that changed.
shots-compare:
	diff -r build/baseline/shots build/shots

# Saves the shots of every script as the baseline that shots-compare compares with.
baseline: shots-all
	rm -rf build/baseline/shots
	mkdir -p build/baseline
	cp -R build/shots build/baseline/shots

# What every change must pass: the build, the linters on the host and for the reader, the tests,
# and shots identical to the baseline.
verify: build lint
	go test ./...
	$(MAKE) shots-all
	$(MAKE) shots-compare

# Benchmarks (docs/reference/profiling.md): each 6 times, for benchstat, in build/bench/new.txt.
# BENCH (a pattern) and PKG narrow them: make bench BENCH=Paginate PKG=./internal/typesetting/layout.
# bench-baseline saves a run as the baseline that bench-compare compares the last run with.
BENCH ?= .
PKG ?= ./...
BENCHSTAT := go run golang.org/x/perf/cmd/benchstat@v0.0.0-20260929162123-406019bb8b68
bench:
	mkdir -p build/bench
	go test -run '^$$' -bench '$(BENCH)' -benchmem -count 6 $(PKG) > build/bench/new.txt; \
		status=$$?; cat build/bench/new.txt; exit $$status

bench-baseline: bench
	cp build/bench/new.txt build/bench/base.txt

bench-compare:
	$(BENCHSTAT) build/bench/base.txt build/bench/new.txt

# SCRIPT run headless against its fake servers, as in shots-all, with its CPU profile, execution
# trace and other profiles in build/profile/<script>.
profile: name = $(basename $(notdir $(SCRIPT)))
profile:
	go build -o build/emulator ./cmd/emulator
	rm -rf build/profile/$(name)
	build/emulator -fake -fresh -frozen-status -script $(SCRIPT) -out build/profile/$(name)/shots \
		-profile build/profile/$(name) $(call script-flags,$(name)) $(EMUFLAGS)
	@echo "go tool pprof -http=: build/profile/$(name)/cpu.pprof"
	@echo "go tool trace build/profile/$(name)/trace.out"

# Launcher icons: uncompressed 8-bit BMP with a Windows 3.x header (BMP3),
# committed so builds don't need ImageMagick and librsvg. Force a rebuild with make -B icons.
icons: icons/inkshelf.bmp icons/inkshelf_f.bmp internal/servers/komga/logo.png internal/servers/mealie/logo.png \
	internal/servers/calibreweb/logo.png

# The app's icon (inkshelf.svg), 98 px wide, centred on the launcher's 106x128 cell.
icons/inkshelf.bmp: icons/inkshelf.svg
	rsvg-convert -w 98 -b white $< | magick - -background white -gravity center -extent 106x128 \
		-colorspace Gray -type Palette -compress None BMP3:$@

# Focused (selected) icon: the same icon, inverted.
icons/inkshelf_f.bmp: icons/inkshelf.bmp
	magick $< -negate -colorspace Gray -type Palette -compress None BMP3:$@

# The logos the app shows for each kind of server: gray PNG, 256 px high, on white. The app
# fits them to their place when it starts.
internal/servers/komga/logo.png: icons/komga.svg
	rsvg-convert -h 256 -b white $< | magick - -colorspace Gray -strip $@
internal/servers/mealie/logo.png: icons/mealie.svg
	rsvg-convert -h 256 -b white $< | magick - -colorspace Gray -strip $@
internal/servers/calibreweb/logo.png: icons/calibreweb.svg
	rsvg-convert -h 256 -b white $< | magick - -colorspace Gray -strip $@

# The documentation site (docs, Docusaurus): the learning chapters and the reference, served
# with live reload at http://localhost:3000, or built to docs/build, which fails on a broken
# link. Needs Node 20 or later.
docs:
	npm --prefix docs install
	npm --prefix docs start

docs-build:
	npm --prefix docs ci
	npm --prefix docs run typecheck
	npm --prefix docs run build

# Refreshes the emulator's shots the site shows, at half size (committed, like the icons): run
# make shots-all first. docs/static/img/shots/<script>__<shot>.png comes from
# build/shots/<script>/<shot>.png; to show a new shot, copy it there under that name once.
DOCS_SHOTS := $(sort $(wildcard docs/static/img/shots/*__*.png))
docs-shots:
	$(foreach shot,$(DOCS_SHOTS),$(call docs-shot,$(shot)))

# docs-shot refreshes the site's shot $(1) from the emulator's.
define docs-shot
magick build/shots/$(subst __,/,$(notdir $(1))) -resize 50% -strip $(1)

endef

clean:
	rm -rf build
