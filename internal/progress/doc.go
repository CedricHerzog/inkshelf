// Package progress keeps the reading progress of the books on the reader and on their server
// in agreement.
//
// The reader saves a book's progress on every page turn, and marks it unsynced
// (shelf.Book.Unsynced) until the server has it. The Syncer sends the unsynced progress a few
// seconds after the last page turn, so that paging through a book sends it once, and again at
// the moments the app chooses (closing a book, the network coming back, the app closing).
// When a book opens, Merge decides whose progress wins: the server's, read on another device,
// unless the reader has progress the server hasn't seen yet. A server that keeps no progress
// (Mealie) answers errors.ErrUnsupported, which counts as synced: the reader's progress is all
// there is.
//
// # Locks
//
// The Syncer sends one push at a time, under a mutex of its own, and while pushing it takes the
// app's mutex for short moments, to read which server to push to and to mark books synced. The
// order is fixed: the Syncer's mutex first, then the app's; the app never pushes while holding
// its mutex.
package progress
