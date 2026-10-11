// Package servers is what the app needs from a server of books, said once for every kind of
// server. The app never talks to Komga, Mealie or Calibre-Web directly: it talks to a Server,
// and each kind of server has a package below this one (servers/komga, servers/mealie,
// servers/calibreweb) that turns the server's own API into a Server.
//
// # The model
//
// A server holds Series, each holding Books in an order (a Book's Number, which may be
// fractional: a novella numbered 2.5 sits between tomes 2 and 3). A server may also file its
// series in Libraries ("Manga", "Comics"), which the series list can filter on. These types are
// neutral: they hold only what the app shows or needs, whatever the server calls it. A Mealie
// recipe is a Book in the Series of its category, and a Kind gives the words to show for them.
//
// A Book's Format says how it is downloaded and read:
//   - PageImages: a book of pictures, fetched one page at a time through OpenBook (Komga, and
//     Mealie's recipes, which are drawn into pictures on the reader's side).
//   - ComicArchive: a CBZ fetched whole through OpenFile, its pages then taken out of it.
//   - Reflowable: an EPUB fetched whole through OpenFile, laid out on the reader as it is read.
//
// # Reading progress
//
// Some servers remember how far each book has been read (ReadingProgress, the last page read).
// The app sends its progress to them and takes theirs when they know better. A server that has
// no such thing answers with an error wrapping errors.ErrUnsupported, and so does a server for
// anything else it cannot do (Libraries, InProgress): the app then does without, and the
// reader's own progress is the only one.
//
// # Configuration
//
// The servers to show come from config.json, copied to the reader over USB: a list of Config,
// each naming its kind (Type) and how to log in. Package registry turns a Config into the
// Server of its kind.
//
// # Sorting titles
//
// Titles are sorted the way a reader expects in French and English: accents and leading
// articles left out ("Le Trône de Fer" files under T), numbers compared as numbers ("Tome 2"
// before "Tome 10"). SortKey and CompareTitles do it. Fold takes accents and ligatures out of
// a title, since the A–Z jump only knows the letters A to Z.
package servers
