// Package app is Inkshelf's user interface: it shows one screen at a time, turns the reader's
// buttons and touches into what the screen shown does, and starts the work done in the
// background.
//
// # Packages
//
// The app is split by what each part knows:
//
//   - state holds everything the app knows while it runs, and the mutex that guards it.
//   - widgets draws what every screen draws with: buttons, panels, text that fits, logos.
//   - The services (session, catalog, downloadqueue, covers, openbook) are functions over the
//     state that do the app's work: loading the lists, downloading books, opening a book.
//   - The screens (startup, serieslist, booklist, reader, contents, coverview, and list, which
//     the lists share) draw the state and react to input by calling services.
//
// A screen never imports another one. It moves to another screen by its ScreenID, with
// state.ShowLocked, and leaves to the services whatever another screen must see changed: the
// reader closing a book tells the catalog the book's progress, which the book list then shows.
//
// # Threading
//
// The event loop of pocketbook calls the app (Init, Draw, Key, Pointer, System, Close) from
// one goroutine: the main thread. Drawing runs there, and so do the closures of the targets
// and the screens' commands.
//
// Work that takes a while runs on goroutines of its own: loading the session, fetching a
// series' books, downloading, loading covers, preloading pages. They never draw. They change
// the state inside state.Update, which takes the mutex and asks the event loop for a repaint;
// the next Draw then shows the change. Only the main thread starts goroutines, with state.Go,
// so that Close can wait for all of them.
//
// Draw holds the mutex while a screen draws, and a screen registers its targets as it draws
// them. A touch is hit-tested against those targets with the mutex held, but what the target
// does runs after the mutex is released: it takes the mutex itself, through state.Update.
//
// # Locks
//
// Three locks are taken in one order, never the other way round:
//
//  1. the progress.Syncer's lock, while it sends the reading progress to the server;
//  2. the state's mutex;
//  3. the lock of the reading engine's renderer, while it lays out or draws a page of text.
//
// So code that holds the state's mutex may lay out text, but must never call the Syncer, and
// the renderer never calls back into the app. Functions whose name ends in Locked run with the
// state's mutex held and never take it.
package app
