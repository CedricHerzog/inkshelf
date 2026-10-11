// Package reader holds the screens of the open book: the reader itself, and the question
// whether to delete a book just finished.
//
// The reader shows a page of the book, and over it the toolbar, or the popup of a note a link
// led to. Past the last page it shows the end panel, which offers the next book of the series.
// A tap on the side of the page turns it, by the book's reading direction: right to left for
// manga, a tap on the left side is the next page. A tap in the middle shows the toolbar.
//
// An e-ink panel is slow to refresh, and a full refresh flashes. So the reader refreshes itself,
// only where the panel changed: the page, the toolbar or the popup. It keeps what the panel
// shows in the state's Reader.Shown, and refreshes the whole panel every few turns, to clear
// the ghosting that partial refreshes leave.
package reader
