// Package reading is the engine behind the reading of a text book (an EPUB whose text flows to
// fit the screen): where the reader is in it, turning its pages across chapters, drawing them,
// following its links and notes, and its table of contents. It knows nothing of the screen it
// is shown on: the app gives it the page's size, margins and font size as layout.Settings, and
// draws the images it returns.
//
// A text book has no fixed pages. Its position is a chapter and a page in that chapter's
// layout (PagePosition), which changes when the font size does; what is saved is a
// shelf.Location, a chapter and the offset of a page's first character, which survives a new
// font size. For the lists and the progress sync, a book also has virtual pages of
// shelf.CharsPerPage characters each.
//
// # Locks
//
// A TextBook's position and its pages drawn belong to the app: the app reads and changes them
// with its own mutex held, as the rest of its state. Laying out and drawing chapters is slow, so
// the app's preloading goroutine does it for the neighbouring pages without holding the app's
// mutex; that work is guarded by a second mutex, inside the TextBook. The order is fixed: the
// app's mutex first, then the TextBook's, never the other way round. The TextBook never takes
// the app's mutex, and its methods that lay out or draw only take its own.
package reading
