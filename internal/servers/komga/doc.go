// Package komga reads a Komga server, a server of comics and manga, through its REST API.
//
// Every request carries the user's API key in the X-API-Key header (created in Komga under
// Account settings > API keys) and is answered in JSON, lists coming as pages of results that
// the client always asks for whole. The app reads series (/api/v1/series), the books of a
// series (/api/v1/series/{id}/books), libraries, thumbnails, and each page of a book as an
// image (/api/v1/books/{id}/pages/{number}). Komga remembers how far each book has been read:
// the app reads it with the books and writes it back with a PATCH of the book's read-progress.
//
// Komga can convert a page to JPEG or PNG on the fly. The app asks for JPEG, PNG and WebP
// pages as they are, since they are much smaller than Komga's PNG of them, and for the other
// formats as PNG, which loses nothing (see pageConversions).
//
// Client is the API, call by call; Server turns it into a servers.Server.
package komga
