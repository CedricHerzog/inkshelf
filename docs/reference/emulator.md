# The emulator

[cmd/emulator](../../cmd/emulator) runs the app on your computer, on an emulated PocketBook. It is
the real app: the same event loop of [device/pocketbook](../../internal/device/pocketbook), the same
screens, the same server clients. Only the firmware is replaced, by `pocketbook.Emulator`, which
keeps the e-ink panel in memory. Use it to try every UI change before deploying.

What it shows faithfully: the panel as the reader would show it. Its panel is an image that only
changes on a refresh, rounded to 16 grays, so an area drawn but never refreshed stays stale, as
on the reader. What it cannot show: real ghosting, the reader's speed, Wi-Fi, sleep and the
firmware's own behavior. Those need the reader (see the end of this page).

## Interactive or headless

- **Interactive**: `make emu-fake` (built-in fake servers) or `make emu` (the servers of
  `emulator/config.json`, written as on the reader, gitignored). Open http://localhost:8740. A
  click is a tap, holding is a long press, dragging is a swipe. `[` and `]` are the page
  buttons, `m` is Menu, Enter is OK, Backspace is Back. The side panel has Home, Exit, Wi-Fi
  events and the battery; "Outline refreshes" draws each refresh and its waveform.
- **Headless**: `-script FILE` runs a script of taps, keys and waits, and saves screenshots of
  the panel. This is how UI changes are checked, and what `make verify` compares.

The emulated reader keeps its files in `emulator/` (`-dir`): `config.json`, `servers/<name>/`, and
with `-fake`, everything under `emulator/fake/`. `emulator/fonts/` holds the reader's fonts, so
text looks as it does on the reader. It has no CJK font: Chinese and Japanese show as boxes.

## Flags

| Flag | Default | What it does |
|---|---|---|
| `-dir DIR` | `emulator` | the emulated reader's files: `config.json`, `servers/`, `fonts/` |
| `-fake` | off | run built-in fake servers instead of reading `DIR/config.json`; their files go in `DIR/fake` |
| `-fake-type T` | `komga` | the kind of fake server: `komga`, `mealie`, `calibreweb`, or several separated by commas, used in turn |
| `-servers N` | 1 | with `-fake`, how many fake servers, to get the server screen |
| `-fresh` | off | start with no downloaded books and no saved filters |
| `-size WxH` | `1404x1872` | the screen size, portrait |
| `-eink` | off | refreshes take as long as on an InkPad 3 |
| `-addr ADDR` | `localhost:8740` | where the web page is served |
| `-script FILE` | none | run this script headless instead of serving the web page |
| `-out DIR` | `build/shots` | where the script's shots go |
| `-frozen-status` | off | a fixed clock and free space, and fixed addresses for the fake servers, so that two runs give byte-identical shots |
| `-profile DIR` | none | write the run's CPU profile, execution trace and other profiles to `DIR` ([profiling.md](profiling.md)) |

Pass flags through make with `EMUFLAGS`, e.g. `make emu-fake EMUFLAGS="-servers 2 -fake-type komga,mealie"`.

## Script commands

One command per line, `#` starts a comment, coordinates in screen pixels. The full reference is
at the top of [cmd/emulator/script.go](../../cmd/emulator/script.go).

| Command | What it does |
|---|---|
| `tap X Y` | touch and lift |
| `long X Y` | touch, hold past the long press delay, lift |
| `swipe X1 Y1 X2 Y2` | touch, move, lift |
| `key NAME` | press and release a button: `prev`, `next`, `left`, `right`, `up`, `down`, `menu`, `ok`, `back`, `plus`, `minus` |
| `hold NAME DURATION` | press a button, release it after `DURATION` (e.g. `2s`) |
| `home`, `resume` | send the app to the background, bring it back |
| `battery PERCENT [charging]` | set the battery |
| `net up`, `net down` | the firmware's network events |
| `wait DURATION` | sleep |
| `idle [TIMEOUT]` | wait until the app is idle (default timeout 2m) |
| `shot NAME` | wait until idle, then save the panel to `OUT/NAME.png` |
| `exit` | ask the app to exit, as the firmware does; the script ends there |

## Waiting for the app

A script never guesses how long something takes. Before each input command (`tap`, `long`,
`swipe`, `key`, `hold`), it waits until the app is **settled**: the event loop has handled the
input and repaints sent before, and the app's background work is over. `idle` and `shot` wait
until it is **idle**: settled, and the panel unchanged for half a second.

Background work is what the app started with `state.Go` (a download, loading a series' books,
loading a cover) and each wake-up of the preloader and the cover loader until they finish it.
The app counts it in `state.Work`, and the emulator reads it through `app.App`'s
`BackgroundWork`. See [app-threading.md](app-threading.md). So after `tap` on a book to download,
`shot` waits for the download. An `idle 60s` before a long download only raises the timeout.

## Make targets

| Target | What it runs |
|---|---|
| `make emu-fake` | interactive, against a fake Komga (add `EMUFLAGS`) |
| `make emu` | interactive, against `emulator/config.json` |
| `make shots` | headless: `SCRIPT` (default `emulator/scripts/smoke.txt`) against the fake Komga, shots in `build/shots` |
| `make shots-all` | every script of `emulator/scripts`, each with its flags, `-frozen-status`, shots in `build/shots/<script>/` |
| `make baseline` | `shots-all`, then saves the shots as `build/baseline/shots` |
| `make shots-compare` | `diff -r build/baseline/shots build/shots` |
| `make profile` | `SCRIPT` as in `shots-all`, with `-profile build/profile/<script>` ([profiling.md](profiling.md)) |
| `make verify` | build, `make lint` (golangci-lint on the host and for `GOOS=linux GOARCH=arm`), `go test`, `shots-all`, `shots-compare` |

Each script needs the fake servers it was written for. `shots-all` passes them for you
(`script-flags` in the [Makefile](../../Makefile)). With `make shots`, pass them yourself:

| Script | `EMUFLAGS` |
|---|---|
| `smoke.txt` | none (fake Komga) |
| `servers.txt` | `-servers 2` |
| `mealie.txt` | `-fake-type mealie` |
| `calibreweb-comics.txt`, `calibreweb-epub.txt`, `calibreweb-fxl.txt`, `calibreweb-links.txt`, `calibreweb-mathml.txt`, `calibreweb-showcase.txt`, `calibreweb-toc.txt` | `-fake-type calibreweb` |

For example: `make shots SCRIPT=emulator/scripts/calibreweb-epub.txt EMUFLAGS="-fake-type calibreweb"`.

## Verifying a UI change

1. Before changing anything, `make baseline` (if `build/baseline/shots` is missing or stale).
2. Make the change. Write or extend a script, with a `shot` after each step you want to see.
3. `make shots SCRIPT=…` and look at the PNGs in `build/shots`.
4. `make verify`. Shots that changed are listed by `shots-compare`. Open each one and its
   baseline: every difference must be one you meant. Then `make baseline` to accept them.

Without `-frozen-status`, shots differ between runs by the clock and the free space in the list
header.

## Screen coordinates

For the default 1404x1872 screen (an InkPad 3). The scripts in `emulator/scripts` use them and
their comments repeat them; sizes come from the constants of [app/widgets](../../internal/app/widgets)
and of each screen's package, scaled by `widgets.Px`.

- **Lists and contents**: header Back at (150, 84); footer at y 1790: its left third is
  ‹ Prev, its right third Next ›, and its middle the page number, which opens the A–Z jump on
  the series list.
- **Server screen** (`-servers N`): buttons 200 px high from y 580, 240 px apart (centres at
  y 680, 920…).
- **Series screen**: filter segments at x 207, 537, 867, 1197, at y 230 (libraries) and y 334
  (read status); series rows 261 px high from y 396 (centres 526, 787, 1048…), the first being
  the book to continue when there is one. Without libraries (Mealie, Calibre-Web), only the
  status filter, at y 230, and rows 282 px high from y 292. A–Z jump letters 191 px apart from
  (223, 686), 6 a line.
- **Books screen**: filter segments at x 262, 702, 1142, y 230; book rows 282 px high from y 292
  (centres 433, 715, 997, 1279, 1561); a row's Delete at x 1256; a book's hold menu (3 buttons)
  at y 813, 963, 1113.
- **End panel**: the first button at y 973. Delete question: Delete at y 918, Keep at y 1068.
- **Reader toolbar**: the direction button at (1116, 84). For an EPUB: A− at (906, 84), A+ at
  (1060, 84), Contents at (720, 84), and after a jump ‹ Return at (460, 84). They move a little
  with the width of the page label.
- **Contents screen**: tabs at x 262, 702, 1142, y 230 (when the book has printed pages or
  landmarks); lines 141 px high from y 292; printed pages in a grid of 5 columns centred at
  x 169, 435, 702, 968, 1234, on the same lines.
- **The fake EPUB** (Calibre-Web, *Les Étoiles*, chapter 1): its links on one line at y 1057:
  the note's call at x 392, the last chapter at 570, the notes at 875, the web at 1170; a
  publisher's note's call (no `epub:type`) at (520, 1115).

## On the reader

Ghosting, speed, Wi-Fi, sleep, USB and going to the background need the real reader: `make
deploy`, use the app, then read `applications/inkshelf.log` over USB. Each page turn logs its
load and refresh times, and the firmware code logs its refresh waits. See
[eink-and-firmware.md](eink-and-firmware.md). To profile the reader itself, see
[profiling.md](profiling.md).
