package hyphen

import (
	"cmp"
	"embed"
	"log"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"
)

// patternFiles holds hyph-<tag>.pat.txt, one pattern per line ("hy3ph": letters with the value
// of the gaps between them, "." for the word's ends), and hyph-<tag>.hyp.txt, words hyphenated
// by hand ("as-so-ciate"), for some languages.
//
//go:embed patterns/*.txt
var patternFiles embed.FS

// load parses the hyphenator's pattern and exception files.
func (hyphenator *Hyphenator) load() {
	hyphenator.patterns = readPatterns("patterns/" + hyphenator.file + ".pat.txt")
	hyphenator.exceptions = readExceptions("patterns/" + hyphenator.file + ".hyp.txt")
}

// readPatterns returns the patterns of a .pat.txt file.
func readPatterns(path string) trie {
	data, err := patternFiles.ReadFile(path)
	if err != nil {
		log.Printf("hyphen: %v", err)
	}
	text := string(data)
	builder := newTrieBuilder(strings.Count(text, "\n"))
	var letters []rune
	var values []uint8
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		// A pattern starting with an apostrophe (French: 'in2a3nit) is for the part of a word after
		// one, which Points hyphenates as a word. Other patterns with an apostrophe or a hyphen
		// would only match across them, where Points never hyphenates.
		if afterApostrophe, found := strings.CutPrefix(line, "'"); found {
			line = "." + afterApostrophe
		} else if afterApostrophe, found := strings.CutPrefix(line, "’"); found {
			line = "." + afterApostrophe
		}
		if line == "" || strings.ContainsAny(line, "'’-") {
			continue
		}
		letters, values = letters[:0], append(values[:0], 0)
		for _, character := range line {
			if character >= '0' && character <= '9' {
				values[len(values)-1] = uint8(character - '0')
				continue
			}
			letters = append(letters, character)
			values = append(values, 0)
		}
		builder.add(letters, values)
	}
	return builder.trie()
}

// readExceptions returns the words of a .hyp.txt file, if the language has one, with their
// points.
func readExceptions(path string) map[string][]int {
	data, err := patternFiles.ReadFile(path)
	if err != nil {
		return nil
	}
	exceptions := make(map[string][]int)
	for line := range strings.Lines(string(data)) {
		var word strings.Builder
		var points []int
		for _, character := range strings.TrimSpace(line) {
			if character == '-' {
				points = append(points, utf8.RuneCountInString(word.String()))
				continue
			}
			word.WriteRune(character)
		}
		if word.Len() > 0 {
			exceptions[word.String()] = points
		}
	}
	return exceptions
}

// trie holds patterns by their letters: node 0 is the root, and a pattern's letters lead from it
// to the node holding its values. It is made of flat slices rather than maps:
// matching walks it from every letter of every word laid out, and slices are faster.
type trie struct {
	nodes    []trieNode
	letters  []rune  // the letters to each node's children, sorted, node after node
	children []int32 // the children, in the same order
	values   []uint8 // the values of the nodes' patterns, node after node
}

type trieNode struct {
	firstChild, endChild int32 // its children in letters and children
	firstValue, endValue int32 // the values of the pattern ending there, if any, in values
}

// match raises levels[gap] to the value that each pattern found at the start of text gives the
// gap before text[gap].
func (patterns *trie) match(text []rune, levels []uint8) {
	node := patterns.nodes[0]
	for _, letter := range text {
		index, found := slices.BinarySearch(patterns.letters[node.firstChild:node.endChild], letter)
		if !found {
			return
		}
		node = patterns.nodes[patterns.children[int(node.firstChild)+index]]
		for gap, value := range patterns.values[node.firstValue:node.endValue] {
			levels[gap] = max(levels[gap], value)
		}
	}
}

// trieBuilder builds a trie: its children are in a map until they are sorted.
type trieBuilder struct {
	children map[edge]int32
	nodes    []trieNode
	values   []uint8
}

// edge is the way from a node to its child for a letter.
type edge struct {
	node   int32
	letter rune
}

// newTrieBuilder returns a builder with room for about patternCount patterns.
func newTrieBuilder(patternCount int) *trieBuilder {
	// A pattern adds 1.6 nodes and 6 values on average (German: 36,709 patterns, 57,875 nodes).
	return &trieBuilder{
		children: make(map[edge]int32, patternCount*2),
		nodes:    append(make([]trieNode, 0, patternCount*2), trieNode{}),
		values:   make([]uint8, 0, patternCount*7),
	}
}

// add adds a pattern: its letters, and values, the value of the gap before each letter and after
// the last.
func (builder *trieBuilder) add(letters []rune, values []uint8) {
	node := int32(0)
	for _, letter := range letters {
		child, found := builder.children[edge{node, letter}]
		if !found {
			child = int32(len(builder.nodes))
			builder.nodes = append(builder.nodes, trieNode{})
			builder.children[edge{node, letter}] = child
		}
		node = child
	}
	builder.nodes[node].firstValue = int32(len(builder.values))
	builder.values = append(builder.values, values...)
	builder.nodes[node].endValue = int32(len(builder.values))
}

// trie returns the trie, its children sorted by node, then by letter.
func (builder *trieBuilder) trie() trie {
	edges := slices.SortedFunc(maps.Keys(builder.children), func(first, second edge) int {
		return cmp.Or(cmp.Compare(first.node, second.node), cmp.Compare(first.letter, second.letter))
	})
	patterns := trie{
		nodes:    builder.nodes,
		letters:  make([]rune, len(edges)),
		children: make([]int32, len(edges)),
		values:   builder.values,
	}
	for index, edge := range edges {
		patterns.letters[index] = edge.letter
		patterns.children[index] = builder.children[edge]
		parent := &patterns.nodes[edge.node]
		if parent.firstChild == parent.endChild {
			parent.firstChild = int32(index)
		}
		parent.endChild = int32(index + 1)
	}
	return patterns
}
