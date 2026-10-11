package servers

// Kind is what the app shows of a kind of server: its name, its logo, and its words for a
// series and a book.
type Kind struct {
	Name  string // e.g. "Komga"
	Logo  []byte // a PNG, black on white; nil if none
	Words Words
}

// Words are how a kind of server calls its series and books, in lower case.
type Words struct {
	Series, SeriesPlural string // e.g. "series", "series"; "category", "categories"
	Book, BookPlural     string // e.g. "book", "books"; "recipe", "recipes"
}

// BookWords are the words of servers of books: series and books.
var BookWords = Words{Series: "series", SeriesPlural: "series", Book: "book", BookPlural: "books"}

// KindOf returns the Kind of a server of config.json.
type KindOf func(config Config) Kind
