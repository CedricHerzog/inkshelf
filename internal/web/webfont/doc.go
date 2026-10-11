// Package webfont turns the fonts a book embeds into TrueType or OpenType data, the formats
// that golang.org/x/image/font/sfnt reads.
//
// Books embed fonts in three formats: TrueType or OpenType as they are (sfnt), WOFF (each
// sfnt table compressed with zlib) and WOFF2 (the whole font compressed with Brotli, some
// tables transformed to compress better). ToSFNT rebuilds an sfnt from any of them.
//
// Publishers often obfuscate the fonts they embed, so that they cannot simply be copied out of
// the book: the first bytes of the font are XORed with a key made from the book's identifier.
// Deobfuscate undoes EPUB's (IDPF) and Adobe's older obfuscation. Some books lost the
// identifier that keyed Adobe's obfuscation; RecoverAdobe finds the key from the font itself,
// whose table directory has bytes known in advance.
package webfont
