package document

// The units of the model's lengths and proportions. They are aliases of int, so they cost
// nothing and convert to nothing: they only say, in a field's type, what its number counts.
type (
	// Hundredths is a length in hundredths of an em: 150 is one em and a half.
	Hundredths = int
	// Percent is a proportion in percent: 100 is the whole.
	Percent = int
	// Pixels is a length in pixels of the screen.
	Pixels = int
)
