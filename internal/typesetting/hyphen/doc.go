// Package hyphen finds where a word may be hyphenated, with Frank Liang's algorithm, the one TeX
// uses, and the hyph-utf8 patterns of a few languages, embedded in the program (patterns/, with
// their licences in patterns/LICENSES).
//
// Liang's algorithm needs no dictionary. A language has a few thousand patterns, short pieces of
// words with a digit in some of the gaps between their letters, such as "hy3ph", "he2n" or ".ex1".
// A dot stands for the start or the end of the word. To hyphenate a word, every pattern found
// anywhere in it sets the digits on the gaps it covers, and each gap keeps the highest digit any
// pattern gave it. An odd digit allows a hyphen in that gap; an even one forbids it, which lets
// later patterns overrule earlier, more general ones. A few words that the patterns get wrong are
// listed whole, hyphens included, as exceptions.
//
// The patterns are kept in a trie, a tree of letters, so that looking up every pattern that starts
// at a letter of the word walks the tree once from that letter. TeX also forbids hyphens too near
// the ends of a word (\lefthyphenmin and \righthyphenmin): "a-" alone at the end of a line reads
// badly.
package hyphen
