# Remote control

The emulator shows the app as the reader would, but not the reader's speed, real ghosting, Wi-Fi
or sleep. Those need the reader. Over USB, the reader can't run the app: in "PC link" it only
lends its storage to the computer. So without remote control, every check on the reader is done
by hand. Someone deploys, taps through the app, then reads the log over USB.

Remote control removes the hands. A computer on the same network can, over Wi-Fi:

- run the emulator's [scripts](emulator.md#script-commands) on the real app;
- save its panel;
- follow its log;
- fetch its profiles;
- install a new build and restart it.

It is off unless a file on the reader turns it on.

## Turning it on

1. Pick a token: any string of 16 characters or more (`openssl rand -hex 16`).
2. Over USB, write it into `applications/inkshelf/remote`. Deploy the app as usual (`make
   deploy`), then start it on the reader.
3. On the computer, write `emulator/reader.json` (gitignored):

   ```json
   {"address": "192.168.1.42:8740", "token": "the same token"}
   ```

   The log tells the address: `remote control on http://192.168.1.42:8740`. `READER` and
   `READER_TOKEN` override the file.

From then on, the reader needs no hands as long as the app stays in front:

```sh
make reader-deploy                                        # build, upload, restart, wait for it
make reader-shots READER_SCRIPT=emulator/scripts/reader/smoke.txt   # shots -> build/reader-shots/smoke/
make reader-log                                           # follow the log, until Ctrl-C
make reader-profile                                       # build/reader-profile/
go run ./cmd/readerctl shot                               # the panel -> build/reader-shots/panel.png
go run ./cmd/readerctl capture 90s                        # profile 90 s now, then fetch the profiles
go run ./cmd/readerctl stats -every 1m -for 15m           # CPU, heap, battery -> build/investigation/*.csv
```

`readerctl script` also saves what the app logged during the script, as `log.txt` next to its
shots. So each run comes with its own page-turn and refresh times.

To turn it off, delete `applications/inkshelf/remote` and restart the app. Without the file,
the app runs exactly as before. It opens no port and starts no extra goroutine.

## How it works

[cmd/inkshelf/remote_control.go](../../cmd/inkshelf/remote_control.go) reads the marker
(`remoteToken`). If it holds a token, `main` makes a `pocketbook.RemoteControl` and starts the
server (`startRemoteControl`), then runs the app with `pocketbook.RunRemoteControlled` in place
of `Run`.

`RemoteControl` ([remote_control.go](../../internal/device/pocketbook/remote_control.go)) sits
between the event loop and the reader's firmware. It implements the same `firmware` interface as
the reader and the `Emulator` ([eink-and-firmware.md](eink-and-firmware.md#the-emulator)), and
passes everything through, with two additions:

- **Input.** A touch or a button of a script is queued as the same raw `/hwevent` bytes the
  firmware sends (`injectedInput`, shared with the `Emulator`). `nextHardwareEvent` returns that
  input before the reader's own, so the real event loop decodes it. Queuing it wakes the loop
  through `inputWake`, as the kernel's signal does for the reader's own input
  ([eink-and-firmware.md](eink-and-firmware.md#the-event-loop)). The reader's queue itself only
  takes the firmware's input: it refuses to let the app write to it.
  - While the app is in the background, injected input is dropped, as the firmware does.
  - `Pending` tells a script when the event loop has handled it all.
- **The panel.** After each refresh, `flush` copies the refreshed area of the canvas into a
  shadow panel, rounded to the waveform's grays as the emulator does (`copyToPanel`).
  - The copy only knows the app's own refreshes. While another app is in front, it still shows
    the app's last screen.
  - The copy runs on the app's goroutine and costs about 30 ms for a full-screen refresh on
    the InkPad 3. Its grays are rounded through a lookup table: dividing per pixel cost about
    130 ms, since the reader's ARM code divides in software. Subtract it from page-turn times
    measured under remote control.

The script language is the emulator's: [device/pocketbook/script](../../internal/device/pocketbook/script/doc.go).
Both readers are a `script.Target`. Only the emulator is also `script.Emulated`, so `home`,
`resume`, `battery`, `net` and `dark` fail on the reader before the script starts.

While on, the app:

- connects to Wi-Fi at start (`KeepWiFiOn`). The app's own connections then find Wi-Fi on, and
  leave it on when they end.
- calls `PreventSleepFor` every minute (`awakeInterval`), because a script's input is not real
  input and the reader would fall asleep.

So measurements under remote control see Wi-Fi always on and no sleep. Idle CPU and memory are
real, but the battery drain is not the one of normal use.

## The HTTP API

Port 8740 (`remotePort`). Every request needs `Authorization: Bearer <token>`. The token is
compared in constant time, and a request without it gets 401. Every reply has `X-Started`: when
the app started. A new value means the app restarted.

| Request | Reply |
|---|---|
| `GET /status` | `{"ready": true, "logSize": N}`: whether the app has opened the reader, and the log's length, where the next lines will start |
| `GET /stats` | Go heap (in use, from the system, allocated since start), garbage collections and their pauses, goroutines, the process's CPU time in nanoseconds (`CLOCK_PROCESS_CPUTIME_ID`; the ticks of `/proc/self/stat` can stand still for minutes after a restart), `/proc/loadavg`, the battery. Reading memory stats pauses the app for a moment, so sample every few seconds at most. |
| `POST /profile?for=90s` | starts a [profile capture](profiling.md#profiling-a-run) into `applications/inkshelf-profile/` (default 90 s, at most 10 min); 409 if one is running |
| `GET /panel.png` | the shadow panel |
| `POST /script` (the script as the body) | `{"shots": [...], "error": "..."}` once the script is over. One script runs at a time. A script that does not parse gets 400. |
| `GET /shots/NAME.png` | a shot of the last script |
| `GET /log?offset=N` | the log from byte N, at most 1 MB. `X-Log-Offset` says where the bytes start, `X-Log-Size` how long the log is. When the log is shorter than N, it was truncated, and the bytes start over from 0. |
| `GET /profile/` | the files of `applications/inkshelf-profile/`, one a line |
| `GET /profile/NAME` | one of them, opened through an `os.Root` so no name can leave the folder |
| `PUT /app` (the program as the body) | `restarting`, then the app restarts |

`PUT /app` refuses anything but a 32-bit little-endian ARM ELF file of 64 MB at most. It
installs the program in four steps:

1. Write it to `inkshelf.app.new` and sync it.
2. Rename it over `inkshelf.app`. The running program keeps its own copy open.
3. Close the app as on exit, saving progress.
4. In `main`, `syscall.Exec` the new program (`execNewApp`). It keeps the process id and group,
   by which the firmware knows the app's task, so the launcher never sees the app go.

The framebuffer is opened close-on-exec, and the message queues are close-on-exec by default,
so the new program inherits none of them. If the exec fails, the app exits and the log says
why. `readerctl deploy` then times out after a minute, and the app must be started on the
reader.

## Limits

- **The app must be running and in front.** Remote control lives in the app. While the app is
  in the background, input goes nowhere and a script times out. Once the firmware closes it,
  nothing answers.
- **No firmware events.** A script can't press Home, change the battery, or turn Wi-Fi or dark
  mode on the reader. Those stay manual, or emulator-only.
- **Real servers.** The scripts of `emulator/scripts/` expect the fake servers, so their
  coordinates land on other things on the reader. Scripts for the reader go in
  `emulator/scripts/reader/`, which `shots-all` does not run. Write them for any content, and
  avoid steps that change the servers. Reading a book syncs its progress to the server.
- **Security.** Anyone on the network with the token can drive the app, read its log and
  replace its program. Use it on a trusted network, keep the token out of the repository, and
  delete the marker when done. The server speaks plain HTTP.
