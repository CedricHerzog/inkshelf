package css

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// tokenKind is the kind of a CSS token, as in CSS Syntax 3's tokenizer.
type tokenKind int

const (
	identToken     tokenKind = iota
	functionToken            // a name followed by "(": the arguments follow, then a closeParenToken
	atKeywordToken           // "@" and a name
	hashToken                // "#" and a name
	stringToken
	badStringToken // a string cut by a newline
	urlToken       // an unquoted url(…)
	badURLToken
	delimToken // any other character
	numberToken
	percentageToken
	dimensionToken // a number and a unit
	whitespaceToken
	cdoToken // "<!--"
	cdcToken // "-->"
	colonToken
	semicolonToken
	commaToken
	openParenToken
	closeParenToken
	openSquareToken
	closeSquareToken
	openCurlyToken
	closeCurlyToken
)

// token is a CSS token.
type token struct {
	kind tokenKind
	// value is the name of an ident, function, at-keyword or hash, the text of a string or a
	// URL (escapes resolved), the character of a delim, the number of a numeric token as written.
	value string
	unit  string // a dimension's unit, as written
	raw   string // the source text of the token
}

// tokenize splits css into tokens. Comments are dropped; nothing is an error.
func tokenize(css string) []token {
	if strings.ContainsAny(css, "\r\f\x00") {
		css = strings.NewReplacer("\r\n", "\n", "\r", "\n", "\f", "\n", "\x00", "�").Replace(css)
	}
	scanner := tokenizer{css: css}
	var tokens []token
	for {
		scanner.skipComments()
		if scanner.position >= len(css) {
			return tokens
		}
		start := scanner.position
		next := scanner.consume()
		next.raw = css[start:scanner.position]
		tokens = append(tokens, next)
	}
}

// tokenizer reads the tokens of css from position on.
type tokenizer struct {
	css      string
	position int
}

// at returns the byte at offset, 0 past the end (the source has no NUL left).
func (scanner *tokenizer) at(offset int) byte {
	if offset < len(scanner.css) {
		return scanner.css[offset]
	}
	return 0
}

// skipComments skips the comments at position; an unclosed one runs to the end.
func (scanner *tokenizer) skipComments() {
	for strings.HasPrefix(scanner.css[scanner.position:], "/*") {
		end := strings.Index(scanner.css[scanner.position+2:], "*/")
		if end < 0 {
			scanner.position = len(scanner.css)
			return
		}
		scanner.position += 2 + end + 2
	}
}

// singleCharacterTokens are the tokens of one character.
var singleCharacterTokens = map[byte]tokenKind{
	':': colonToken, ';': semicolonToken, ',': commaToken, '(': openParenToken, ')': closeParenToken,
	'[': openSquareToken, ']': closeSquareToken, '{': openCurlyToken, '}': closeCurlyToken,
}

// consume reads the token at position, which is not at the end.
func (scanner *tokenizer) consume() token {
	char := scanner.css[scanner.position]
	if kind, ok := singleCharacterTokens[char]; ok {
		scanner.position++
		return token{kind: kind}
	}
	switch {
	case isWhitespace(char):
		for isWhitespace(scanner.at(scanner.position)) {
			scanner.position++
		}
		return token{kind: whitespaceToken}
	case char == '"' || char == '\'':
		return scanner.consumeString(char)
	case char == '#' && (isNameCharacter(scanner.at(scanner.position+1)) || scanner.startsEscape(scanner.position+1)):
		scanner.position++
		return token{kind: hashToken, value: scanner.consumeName()}
	case (char == '+' || char == '-' || char == '.') && scanner.startsNumber(scanner.position):
		return scanner.consumeNumeric()
	case char == '-' && strings.HasPrefix(scanner.css[scanner.position:], "-->"):
		scanner.position += 3
		return token{kind: cdcToken}
	case char == '<' && strings.HasPrefix(scanner.css[scanner.position:], "<!--"):
		scanner.position += 4
		return token{kind: cdoToken}
	case char == '@' && scanner.startsIdent(scanner.position+1):
		scanner.position++
		return token{kind: atKeywordToken, value: scanner.consumeName()}
	case isDigit(char):
		return scanner.consumeNumeric()
	case scanner.startsIdent(scanner.position):
		return scanner.consumeIdentLike()
	}
	scanner.position++
	return token{kind: delimToken, value: string(char)}
}

// consumeString reads a string that quote opens. A newline ends it as a bad string.
func (scanner *tokenizer) consumeString(quote byte) token {
	scanner.position++
	var text strings.Builder
	for {
		char := scanner.at(scanner.position)
		switch {
		case scanner.position >= len(scanner.css):
			return token{kind: stringToken, value: text.String()}
		case char == quote:
			scanner.position++
			return token{kind: stringToken, value: text.String()}
		case char == '\n':
			return token{kind: badStringToken}
		case char == '\\' && scanner.at(scanner.position+1) == '\n':
			scanner.position += 2 // an escaped newline continues the string
		case char == '\\' && scanner.position+1 >= len(scanner.css):
			scanner.position++
		case char == '\\':
			scanner.position++
			text.WriteRune(scanner.consumeEscape())
		default:
			text.WriteByte(char)
			scanner.position++
		}
	}
}

// consumeNumeric reads a number, percentage or dimension.
func (scanner *tokenizer) consumeNumeric() token {
	start := scanner.position
	if char := scanner.at(scanner.position); char == '+' || char == '-' {
		scanner.position++
	}
	scanner.skipDigits()
	if scanner.at(scanner.position) == '.' && isDigit(scanner.at(scanner.position+1)) {
		scanner.position++
		scanner.skipDigits()
	}
	if char := scanner.at(scanner.position); char == 'e' || char == 'E' {
		next := scanner.at(scanner.position + 1)
		if isDigit(next) {
			scanner.position++
			scanner.skipDigits()
		} else if (next == '+' || next == '-') && isDigit(scanner.at(scanner.position+2)) {
			scanner.position += 2
			scanner.skipDigits()
		}
	}
	number := scanner.css[start:scanner.position]
	switch {
	case scanner.startsIdent(scanner.position):
		return token{kind: dimensionToken, value: number, unit: scanner.consumeName()}
	case scanner.at(scanner.position) == '%':
		scanner.position++
		return token{kind: percentageToken, value: number}
	}
	return token{kind: numberToken, value: number}
}

// skipDigits moves past the digits at position.
func (scanner *tokenizer) skipDigits() {
	for isDigit(scanner.at(scanner.position)) {
		scanner.position++
	}
}

// consumeIdentLike reads an ident, a function or a url(…).
func (scanner *tokenizer) consumeIdentLike() token {
	name := scanner.consumeName()
	if scanner.at(scanner.position) != '(' {
		return token{kind: identToken, value: name}
	}
	scanner.position++
	if !strings.EqualFold(name, "url") {
		return token{kind: functionToken, value: name}
	}
	for isWhitespace(scanner.at(scanner.position)) {
		scanner.position++
	}
	if char := scanner.at(scanner.position); char == '"' || char == '\'' {
		return token{kind: functionToken, value: name} // url("…"): a function with a string
	}
	return scanner.consumeURL()
}

// consumeURL reads the rest of an unquoted url(…), its leading whitespace skipped.
func (scanner *tokenizer) consumeURL() token {
	var address strings.Builder
	for {
		char := scanner.at(scanner.position)
		switch {
		case scanner.position >= len(scanner.css):
			return token{kind: urlToken, value: address.String()}
		case char == ')':
			scanner.position++
			return token{kind: urlToken, value: address.String()}
		case isWhitespace(char):
			for isWhitespace(scanner.at(scanner.position)) {
				scanner.position++
			}
			if scanner.at(scanner.position) == ')' || scanner.position >= len(scanner.css) {
				continue
			}
			return scanner.consumeBadURL()
		case char == '"' || char == '\'' || char == '(' || char < ' ' || char == 0x7f:
			return scanner.consumeBadURL()
		case char == '\\':
			if !scanner.startsEscape(scanner.position) {
				return scanner.consumeBadURL()
			}
			scanner.position++
			address.WriteRune(scanner.consumeEscape())
		default:
			address.WriteByte(char)
			scanner.position++
		}
	}
}

// consumeBadURL skips the rest of a broken url(…), up to its ")".
func (scanner *tokenizer) consumeBadURL() token {
	for scanner.position < len(scanner.css) {
		char := scanner.css[scanner.position]
		scanner.position++
		if char == ')' {
			break
		}
		if char == '\\' && scanner.position < len(scanner.css) {
			scanner.position++
		}
	}
	return token{kind: badURLToken}
}

// consumeName reads a name: name characters and escapes.
func (scanner *tokenizer) consumeName() string {
	var name strings.Builder
	for {
		char := scanner.at(scanner.position)
		switch {
		case isNameCharacter(char):
			name.WriteByte(char)
			scanner.position++
		case scanner.startsEscape(scanner.position):
			scanner.position++
			name.WriteRune(scanner.consumeEscape())
		default:
			return name.String()
		}
	}
}

// consumeEscape reads an escape, its backslash already read: up to 6 hex digits and a
// whitespace, or any other character as itself.
func (scanner *tokenizer) consumeEscape() rune {
	start := scanner.position
	for scanner.position-start < 6 && isHexDigit(scanner.at(scanner.position)) {
		scanner.position++
	}
	if scanner.position == start {
		if scanner.position >= len(scanner.css) {
			return utf8.RuneError
		}
		char, size := utf8.DecodeRuneInString(scanner.css[scanner.position:])
		scanner.position += size
		return char
	}
	codePoint, _ := strconv.ParseUint(scanner.css[start:scanner.position], 16, 32)
	if isWhitespace(scanner.at(scanner.position)) {
		scanner.position++
	}
	if codePoint == 0 || codePoint > utf8.MaxRune || (codePoint >= 0xd800 && codePoint <= 0xdfff) {
		return utf8.RuneError
	}
	return rune(codePoint)
}

// startsEscape reports whether a valid escape starts at offset: a backslash not before a
// newline or the end.
func (scanner *tokenizer) startsEscape(offset int) bool {
	return scanner.at(offset) == '\\' && offset+1 < len(scanner.css) && scanner.css[offset+1] != '\n'
}

// startsIdent reports whether an ident starts at offset.
func (scanner *tokenizer) startsIdent(offset int) bool {
	switch char := scanner.at(offset); {
	case char == '-':
		next := scanner.at(offset + 1)
		return isNameStart(next) || next == '-' || scanner.startsEscape(offset+1)
	case isNameStart(char):
		return true
	default:
		return scanner.startsEscape(offset)
	}
}

// startsNumber reports whether a number starts at offset.
func (scanner *tokenizer) startsNumber(offset int) bool {
	char := scanner.at(offset)
	if char == '+' || char == '-' {
		offset++
		char = scanner.at(offset)
	}
	if char == '.' {
		return isDigit(scanner.at(offset + 1))
	}
	return isDigit(char)
}

func isWhitespace(char byte) bool { return char == ' ' || char == '\t' || char == '\n' }
func isDigit(char byte) bool      { return char >= '0' && char <= '9' }

func isHexDigit(char byte) bool {
	return isDigit(char) || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')
}

// isNameStart reports whether char can start a name: a letter, "_", or a byte of a non-ASCII
// character.
func isNameStart(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_' || char >= 0x80
}

func isNameCharacter(char byte) bool { return isNameStart(char) || isDigit(char) || char == '-' }

// componentEnd returns the index after the component value at index in tokens: the token, or
// a whole block or function up to its closing token (or the end).
func componentEnd(tokens []token, index int) int {
	var closers []tokenKind
	for ; index < len(tokens); index++ {
		switch kind := tokens[index].kind; kind {
		case functionToken, openParenToken:
			closers = append(closers, closeParenToken)
		case openSquareToken:
			closers = append(closers, closeSquareToken)
		case openCurlyToken:
			closers = append(closers, closeCurlyToken)
		default:
			if len(closers) > 0 && kind == closers[len(closers)-1] {
				closers = closers[:len(closers)-1]
			}
		}
		if len(closers) == 0 {
			return index + 1
		}
	}
	return index
}

// components returns the component values of tokens, whitespace left out.
func components(tokens []token) [][]token {
	var parts [][]token
	for index := 0; index < len(tokens); {
		end := componentEnd(tokens, index)
		if tokens[index].kind != whitespaceToken {
			parts = append(parts, tokens[index:end])
		}
		index = end
	}
	return parts
}

// splitAt splits tokens at the top-level tokens of kind.
func splitAt(tokens []token, kind tokenKind) [][]token {
	var parts [][]token
	start := 0
	for index := 0; index < len(tokens); {
		if tokens[index].kind == kind {
			parts = append(parts, tokens[start:index])
			start = index + 1
		}
		index = componentEnd(tokens, index)
	}
	return append(parts, tokens[start:])
}

// trimWhitespace returns tokens without their leading and trailing whitespace.
func trimWhitespace(tokens []token) []token {
	for len(tokens) > 0 && tokens[0].kind == whitespaceToken {
		tokens = tokens[1:]
	}
	for len(tokens) > 0 && tokens[len(tokens)-1].kind == whitespaceToken {
		tokens = tokens[:len(tokens)-1]
	}
	return tokens
}

// serialize returns the source text of tokens, whitespace collapsed to single spaces and
// trimmed.
func serialize(tokens []token) string {
	var text strings.Builder
	afterSpace := false
	for _, current := range trimWhitespace(tokens) {
		if current.kind != whitespaceToken {
			text.WriteString(current.raw)
		} else if !afterSpace {
			text.WriteByte(' ') // whitespace around a comment makes two tokens
		}
		afterSpace = current.kind == whitespaceToken
	}
	return text.String()
}

// isIdent reports whether current is the ident name, in any case.
func isIdent(current token, name string) bool {
	return current.kind == identToken && strings.EqualFold(current.value, name)
}

// isDelim reports whether current is the delim char.
func isDelim(current token, char string) bool {
	return current.kind == delimToken && current.value == char
}
