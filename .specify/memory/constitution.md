<!--
Sync Impact Report
- Version change: template → 1.0.0 (first ratification)
- Principles: I. Pure Go, static, for the reader; II. Domain layering; III. Files on the reader
  stay readable; IV. Hot paths keep their speed; V. One thread draws; VI. Done means verified;
  VII. Docs follow code (the template's five slots extended to seven)
- Added sections: Target and Scope; Development Workflow; Governance
- Removed sections: none
- Templates: plan-template.md, spec-template.md and tasks-template.md read this file at run time
  and need no change
- Deferred TODOs: none
Remove this comment before committing.
-->

# Inkshelf Constitution

These are the rules a plan must pass. Each one names its check. The details behind each rule
live in [CLAUDE.md](../../CLAUDE.md) and [docs/reference](../../docs/reference): this file states
the rules and links to them rather than copying them. Style conventions (names, comments,
loops, sizes in `widgets.Px`) are not principles; CLAUDE.md and golangci-lint hold them.

## Core Principles

### I. Pure Go, static, for the reader

The app MUST build as one static binary with `CGO_ENABLED=0` for the reader's ARM. It MUST NOT
use cgo, the PocketBook SDK or libinkview. Every dependency MUST be pure Go, and no third-party
or standard-library code may be copied into the repository.

Rationale: the reader runs the binary as it is, and the app talks to the framebuffer, the
e-ink panel and the firmware itself.

Check: `make build` succeeds, and no new dependency in `go.mod` needs cgo.

### II. Domain layering

A package MUST import only from its own domain or lower ones, in the order of
[architecture.md](../../docs/reference/architecture.md). Screens MUST NOT import each other.
`app` and `shelf` MUST NOT import a specific server. Names outside the packages about one device
or one server MUST stay free of that device and that server.

Rationale: a Kobo or Kindle port and more kinds of servers may come; each must be one new
package, not a rewrite of the app.

Check: `go list -deps ./internal/app/... ./internal/shelf | grep -E
'servers/(komga|mealie|calibreweb)'` prints nothing, and the plan places each new package in the
domain map.

### III. Files on the reader stay readable

On-disk formats MUST NOT change: JSON tags, the order of `servers.Format`, `.pg4` and file
names stay as they are. A change to a chapter's rune count MUST bump `epub.IndexVersion`. See
[on-disk-formats.md](../../docs/reference/on-disk-formats.md).

Rationale: every later version must read the files an earlier one saved on the reader.

Check: the golden tests pass unchanged.

### IV. Hot paths keep their speed

A function marked `HOT PATH` MUST change only with its benchmark no slower under `benchstat`
and the same inlining under `GOOS=linux GOARCH=arm GOARM=7 go build -gcflags=-m`. Work that can
take a while MUST run in a `timing.Start` span, never inside a hot path. New costly code MUST
come with a benchmark. See [image-pipeline.md](../../docs/reference/image-pipeline.md#hot-paths)
and [profiling.md](../../docs/reference/profiling.md).

Rationale: these functions are tuned against the ARM assembly, and a page turn on the reader
depends on them.

Check: a plan that touches a hot path includes `make bench-baseline` before and
`make bench-compare` after, and compares the inlining list.

### V. One thread draws

The event loop MUST call the app on one goroutine. Other goroutines MUST change the state only
inside `state.Update` and MUST NOT draw. Only the main thread starts goroutines, with `state.Go`
or `GoLoop`. A function whose name ends in `Locked` MUST NOT take the state's mutex. Locks MUST
be taken in the order `progress.Syncer`, then the state, then a `reading.TextBook`'s renderer,
and the Syncer MUST NOT be called with the state's mutex held. See
[app-threading.md](../../docs/reference/app-threading.md).

Rationale: the screen, the state and the sync have one owner each, so they never race or
deadlock.

Check: the plan names the goroutine each new piece of work runs on, and the locks it takes.

### VI. Done means verified

A change MUST pass `make verify`. Any difference `shots-compare` reports MUST be intended and
looked at. A UI change MUST add a script step and a `shot` that shows it. What only the reader
can show (ghosting, speed, Wi-Fi, sleep, USB, the app in the background) MUST be checked on the
reader, as CLAUDE.md describes.

Rationale: the emulator runs the real screens, so its shots catch regressions; the reader
catches the rest.

Check: `tasks.md` includes these steps, and `/speckit-implement` does not finish without them.

### VII. Docs follow code

A change that makes a page of `docs/reference` or a chapter of `docs/learn` wrong MUST update it
in the same change, as [WRITING.md](../../docs/WRITING.md) describes. A spec in `specs/NNN-*/`
records what a feature intends; it MUST NOT replace `docs/reference`.

Rationale: the reference is the precise description of the code, and a spec ages once the
feature ships.

Check: the plan lists the pages it changes, or states that none change.

## Target and Scope

Inkshelf targets the PocketBook InkPad 3 (PB740, 1404x1872, B288) on firmware 6.x. It reads from
Komga (comics and manga), Mealie (recipes) and Calibre-Web (EPUB and CBZ). Other devices and
servers may come, which is why principle II keeps the device and the servers in their own
packages.

## Development Workflow

- One change per commit, with a short imperative message, and `make verify` before each.
- Each feature gets a Spec Kit branch named `NNN-slug`, and its spec, plan and tasks go in
  `specs/NNN-slug/`.
- `/speckit-plan` fills its Constitution Check from the principles above, and `/speckit-analyze`
  treats a break of a MUST rule as critical.

## Governance

CLAUDE.md is the runtime guidance file and holds the details. This constitution states the
rules a plan must pass.

A principle changes only through `/speckit-constitution`, with a semantic version bump:
- MAJOR when a principle is removed or redefined;
- MINOR when a principle or section is added, or its guidance is materially expanded;
- PATCH when it is only reworded.

When CLAUDE.md changes a rule that a principle states, the constitution is amended in the same
change.

A plan that breaks a principle MUST either justify it in its Complexity Tracking table or change
until it complies. Reviews check plans and tasks against the Check line of every principle.

**Version**: 1.0.0 | **Ratified**: 2026-11-01 | **Last Amended**: 2026-11-01
