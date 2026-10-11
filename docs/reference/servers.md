# Servers

Inkshelf reads books from three kinds of servers: Komga (comics and manga), Mealie (recipes)
and Calibre-Web (ebooks). The app never talks to any of them directly. It talks to one
interface, `servers.Server`, and each kind of server has a package that turns its own API into
that interface. A fourth kind is one more package.

## The Server interface and its model

[internal/servers](../../internal/servers) says, once for every kind, what the app needs:

- `Series`, `Libraries`, `Books(seriesID)`, `InProgress`: the lists.
- `OpenBook` (then `Pages.Page(n)`) for a book of page images, `OpenFile` for a book fetched
  whole (a CBZ or an EPUB), `FirstPage`: downloading.
- `SeriesThumbnail`, `BookThumbnail`: the small covers.
- `SetProgress`, `ClearProgress`: the reading progress (see [progress-sync.md](progress-sync.md)).

The model is neutral: it holds only what the app shows or needs, whatever the server calls it.

| Type | What it is |
|---|---|
| `Series` | a series of books, with its counts of books read and in progress |
| `Book` | a book of a series; its `Number` (may be 2.5) orders it; its `Format` says how it is downloaded |
| `Library` | a group of series (Komga's "Manga", "Comics"), for the series list's filter |
| `Format` | `PageImages` (fetched page by page), `ComicArchive` (a CBZ, fetched whole), `Reflowable` (an EPUB, fetched whole) |
| `ReadingProgress` | the last page read and whether the book is completed |
| `Direction` | `LeftToRight` or `RightToLeft`, or "" when the server does not say |
| `Kind` | a kind of server's name, logo and `Words` ("category"/"recipe" for Mealie) |
| `Config` | one entry of `config.json` |

`servers` also sorts titles for French and English readers (`SortKey`, `CompareTitles`, `Fold`):
leading articles and accents left out, numbers compared as numbers through
[naturalorder](../../internal/naturalorder).

### What a server cannot do

A server that cannot do something returns an error wrapping `errors.ErrUnsupported`, and the app
does without: no library filter, or no progress on the server. Never return an empty list for
"not supported": the app would believe there is nothing.

## The three kinds

| Kind | Package | API | Login | Notes |
|---|---|---|---|---|
| Komga | [servers/komga](../../internal/servers/komga) | REST, JSON | API key in `X-API-Key` | Books are `PageImages`: pages fetched one by one, as originals when JPEG/PNG/WebP, else converted to PNG by Komga (`pageConversions`). Keeps progress and libraries. |
| Mealie | [servers/mealie](../../internal/servers/mealie) | REST, JSON | API token as `Authorization: Bearer` | One series per category, one book per recipe. [servers/mealie/recipepages](../../internal/servers/mealie/recipepages) lays a recipe out and draws its pages as images. No progress, no libraries. |
| Calibre-Web | [servers/calibreweb](../../internal/servers/calibreweb) | OPDS (Atom feeds) | HTTP Basic auth | Reads the whole catalog. A book is an EPUB (`Reflowable`) or a CBZ (`ComicArchive`), the EPUB preferred. No progress, no libraries. |

Each package has a `Client` (the API, call by call) and a `Server` (the `servers.Server` built on
it), made with `NewServer(config, httpClient)`, and a `kind.go` declaring its `Kind`, whose logo
is a `logo.png` embedded with `//go:embed` (made from `icons/*.svg` by `make icons`).

## Shared pieces

- [servers/httpclient](../../internal/servers/httpclient): `New` makes the one HTTP client the app
  sends every request through (more idle connections than Go's default, a one-minute timeout).
  `Do`, `GetJSON` and `GetBytes` send a request a server built and turn a non-2xx answer into a
  `*StatusError`, so callers can tell a 404 or a 401 from a network error with `errors.As`.
  Calibre-Web downloads files through a copy of the client without the timeout.
- [servers/registry](../../internal/servers/registry): `Opener` and `KindOf` switch on
  `Config.Type` (`"komga"` or empty, `"mealie"`, `"calibreweb"`). Only the programs in
  [cmd](../../cmd) import it. This is what keeps `app` and `shelf` from knowing any one server
  (see [architecture.md](architecture.md)).

## Fakes

| Fake | For |
|---|---|
| [servers/servertest](../../internal/servers/servertest) | `Fake`, a `Server` in memory whose content is its fields. It records the pages asked for and the progress sent. For the tests of `shelf` and `progress`. |
| [servers/komga/fakekomga](../../internal/servers/komga/fakekomga), [servers/mealie/fakemealie](../../internal/servers/mealie/fakemealie), [servers/calibreweb/fakecalibreweb](../../internal/servers/calibreweb/fakecalibreweb) | `NewHandler()`: an `http.Handler` that answers like the real server, with fixed generated content (series, pages, recipes and photos, EPUBs and CBZs). The emulator runs them (`-fake`, `-fake-type`), and their content never changes, so screenshots compare byte for byte. |

The fakes speak HTTP, so the real client code runs against them: a change to the Komga client
shows in the emulator's shots.

## Adding a kind of server

1. Create `internal/servers/<kind>/` with a `doc.go` that explains the server and its API, a
   `Client`, and a `Server` that implements `servers.Server`. Return errors wrapping
   `errors.ErrUnsupported` for what it cannot do. Send requests through `httpclient.Do`,
   `GetJSON` or `GetBytes`.
2. Add `kind.go`: its `Kind` (name, `Words`, embedded `logo.png`). Draw `icons/<kind>.svg` and
   add a rule for its `logo.png` to the `icons` target of the [Makefile](../../Makefile).
3. Add a case for its `type` in both switches of [servers/registry](../../internal/servers/registry).
4. Write tests against an `httptest` server, as `komga/client_test.go` does.
5. Write `internal/servers/<kind>/fake<kind>/` with a `NewHandler()`, add it to `fakeHandlers` in
   [cmd/emulator/main.go](../../cmd/emulator/main.go), and write a script in
   [emulator/scripts](../../emulator/scripts). If the script needs `-fake-type <kind>`, add it to
   `script-flags` in the Makefile, then `make baseline` once its shots look right.
6. Document the `config.json` entry in [usage.md](usage.md).
7. Run `make verify` and the dependency check of [architecture.md](architecture.md).
