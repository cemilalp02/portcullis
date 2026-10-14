// Package jsonc parses JSON with comments and trailing commas (the dialect of
// VS Code, dev containers and Claude settings) into a positioned tree and
// records duplicate object keys (ADR-0004, REQUIREMENTS FR-06, FR-08).
package jsonc

import (
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// MaxDepth limits nesting of objects and arrays.
const MaxDepth = 256

// Kind is the type of a JSON value.
type Kind int

// Value kinds.
const (
	Null Kind = iota
	Bool
	Number
	String
	Array
	Object
)

func (k Kind) String() string {
	return [...]string{"null", "bool", "number", "string", "array", "object"}[k]
}

// Node is a parsed value. Start and End are byte offsets of the value in the
// source ([Start, End)).
type Node struct {
	Kind       Kind
	Start, End int
	Str        string // decoded string value, or the literal text of a number
	Bool       bool
	Members    []Member // objects, in source order (duplicates included)
	Elems      []*Node  // arrays
}

// Member is one key/value pair of an object.
type Member struct {
	Key      string
	KeyStart int
	Value    *Node
}

// Get returns the value of the last member named key, mirroring consumers
// such as encoding/json and VS Code where the last duplicate wins, or nil.
func (n *Node) Get(key string) *Node {
	if n == nil || n.Kind != Object {
		return nil
	}
	for i := len(n.Members) - 1; i >= 0; i-- {
		if n.Members[i].Key == key {
			return n.Members[i].Value
		}
	}
	return nil
}

// StringValue returns the string value and true if n is a string.
func (n *Node) StringValue() (string, bool) {
	if n == nil || n.Kind != String {
		return "", false
	}
	return n.Str, true
}

// DuplicateKey is a key that appears more than once in the same object.
type DuplicateKey struct {
	Key         string
	Offset      int // offset of the repeated key
	FirstOffset int // offset of the first occurrence
}

// SyntaxError describes invalid input.
type SyntaxError struct {
	Offset int
	Msg    string
}

func (e *SyntaxError) Error() string { return fmt.Sprintf("offset %d: %s", e.Offset, e.Msg) }

// Parse parses src. On a syntax error the returned node is nil; duplicate keys
// found before the error are still returned.
func Parse(src string) (*Node, []DuplicateKey, error) {
	p := &parser{src: src}
	if err := p.skip(); err != nil {
		return nil, p.dups, err
	}
	if p.pos >= len(src) {
		return nil, p.dups, p.errorf("empty document")
	}
	n, err := p.value()
	if err != nil {
		return nil, p.dups, err
	}
	if err := p.skip(); err != nil {
		return nil, p.dups, err
	}
	if p.pos < len(src) {
		return nil, p.dups, p.errorf("unexpected %s after top-level value", p.describe())
	}
	return n, p.dups, nil
}

type parser struct {
	src   string
	pos   int
	depth int
	dups  []DuplicateKey
}

func (p *parser) errorf(format string, args ...any) error {
	return &SyntaxError{Offset: p.pos, Msg: fmt.Sprintf(format, args...)}
}

func (p *parser) describe() string {
	if p.pos >= len(p.src) {
		return "end of input"
	}
	r, _ := utf8.DecodeRuneInString(p.src[p.pos:])
	return fmt.Sprintf("character %q", r)
}

// skip consumes whitespace and comments.
func (p *parser) skip() error {
	for p.pos < len(p.src) {
		switch c := p.src[p.pos]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			p.pos++
		case strings.HasPrefix(p.src[p.pos:], "//"):
			end := strings.IndexByte(p.src[p.pos:], '\n')
			if end < 0 {
				p.pos = len(p.src)
			} else {
				p.pos += end + 1
			}
		case strings.HasPrefix(p.src[p.pos:], "/*"):
			end := strings.Index(p.src[p.pos+2:], "*/")
			if end < 0 {
				return p.errorf("unterminated block comment")
			}
			p.pos += 2 + end + 2
		default:
			return nil
		}
	}
	return nil
}

func (p *parser) value() (*Node, error) {
	switch c := p.src[p.pos]; {
	case c == '{':
		return p.container(Object)
	case c == '[':
		return p.container(Array)
	case c == '"':
		start := p.pos
		s, err := p.str()
		if err != nil {
			return nil, err
		}
		return &Node{Kind: String, Start: start, End: p.pos, Str: s}, nil
	case c == '-' || (c >= '0' && c <= '9'):
		return p.number()
	case strings.HasPrefix(p.src[p.pos:], "true"):
		return p.literal("true", Bool, true), nil
	case strings.HasPrefix(p.src[p.pos:], "false"):
		return p.literal("false", Bool, false), nil
	case strings.HasPrefix(p.src[p.pos:], "null"):
		return p.literal("null", Null, false), nil
	}
	return nil, p.errorf("unexpected %s", p.describe())
}

func (p *parser) literal(word string, k Kind, b bool) *Node {
	n := &Node{Kind: k, Start: p.pos, End: p.pos + len(word), Str: word, Bool: b}
	p.pos += len(word)
	return n
}

func (p *parser) container(k Kind) (*Node, error) {
	p.depth++
	if p.depth > MaxDepth {
		return nil, p.errorf("nesting deeper than %d", MaxDepth)
	}
	defer func() { p.depth-- }()

	closer := byte(']')
	if k == Object {
		closer = '}'
	}
	n := &Node{Kind: k, Start: p.pos}
	p.pos++ // open
	var seen map[string]int
	if k == Object {
		seen = make(map[string]int)
	}
	for first := true; ; first = false {
		if err := p.skip(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.src) {
			return nil, p.errorf("unterminated %s started at offset %d", k, n.Start)
		}
		if p.src[p.pos] == closer {
			p.pos++
			n.End = p.pos
			return n, nil
		}
		if !first {
			if p.src[p.pos] != ',' {
				return nil, p.errorf("expected ',' or %q, found %s", closer, p.describe())
			}
			p.pos++
			if err := p.skip(); err != nil {
				return nil, err
			}
			if p.pos < len(p.src) && p.src[p.pos] == closer { // trailing comma
				p.pos++
				n.End = p.pos
				return n, nil
			}
			if p.pos >= len(p.src) {
				return nil, p.errorf("unterminated %s started at offset %d", k, n.Start)
			}
		}
		if k == Array {
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			n.Elems = append(n.Elems, v)
			continue
		}
		if p.src[p.pos] != '"' {
			return nil, p.errorf("expected object key string, found %s", p.describe())
		}
		keyStart := p.pos
		key, err := p.str()
		if err != nil {
			return nil, err
		}
		if err := p.skip(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.src) || p.src[p.pos] != ':' {
			return nil, p.errorf("expected ':' after object key, found %s", p.describe())
		}
		p.pos++
		if err := p.skip(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.src) {
			return nil, p.errorf("missing value for key %q", key)
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		if firstAt, dup := seen[key]; dup {
			p.dups = append(p.dups, DuplicateKey{Key: key, Offset: keyStart, FirstOffset: firstAt})
		} else {
			seen[key] = keyStart
		}
		n.Members = append(n.Members, Member{Key: key, KeyStart: keyStart, Value: v})
	}
}

func (p *parser) number() (*Node, error) {
	start := p.pos
	s := p.src
	i := p.pos
	if s[i] == '-' {
		i++
	}
	switch {
	case i < len(s) && s[i] == '0':
		i++
	case i < len(s) && s[i] >= '1' && s[i] <= '9':
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	default:
		p.pos = i
		return nil, p.errorf("invalid number")
	}
	if i < len(s) && s[i] == '.' {
		i++
		j := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == j {
			p.pos = i
			return nil, p.errorf("invalid number: missing fraction digits")
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		j := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == j {
			p.pos = i
			return nil, p.errorf("invalid number: missing exponent digits")
		}
	}
	p.pos = i
	return &Node{Kind: Number, Start: start, End: i, Str: s[start:i]}, nil
}

// str parses a string literal starting at the opening quote.
func (p *parser) str() (string, error) {
	start := p.pos
	p.pos++ // opening quote
	var b strings.Builder
	for {
		if p.pos >= len(p.src) {
			p.pos = start
			return "", p.errorf("unterminated string")
		}
		c := p.src[p.pos]
		switch {
		case c == '"':
			p.pos++
			return b.String(), nil
		case c < 0x20:
			return "", p.errorf("control character U+%04X in string", c)
		case c == '\\':
			if err := p.escape(&b); err != nil {
				return "", err
			}
		default:
			r, size := utf8.DecodeRuneInString(p.src[p.pos:])
			b.WriteRune(r)
			p.pos += size
		}
	}
}

// simpleEscapes maps the character after a backslash to its value.
var simpleEscapes = map[byte]byte{'"': '"', '\\': '\\', '/': '/', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}

func (p *parser) escape(b *strings.Builder) error {
	if p.pos+1 >= len(p.src) {
		return p.errorf("unterminated escape")
	}
	c := p.src[p.pos+1]
	if r, ok := simpleEscapes[c]; ok {
		b.WriteByte(r)
		p.pos += 2
		return nil
	}
	if c != 'u' {
		return p.errorf("invalid escape %q", "\\"+string(c))
	}
	r1, ok := hex4(p.src, p.pos+2)
	if !ok {
		return p.errorf("invalid unicode escape")
	}
	p.pos += 6
	if utf16.IsSurrogate(r1) {
		if r2, ok := hex4(p.src, p.pos+2); ok && strings.HasPrefix(p.src[p.pos:], "\\u") {
			if dec := utf16.DecodeRune(r1, r2); dec != utf8.RuneError {
				b.WriteRune(dec)
				p.pos += 6
				return nil
			}
		}
		r1 = utf8.RuneError // lone surrogate, as encoding/json does
	}
	b.WriteRune(r1)
	return nil
}

func hex4(s string, i int) (rune, bool) {
	if i+4 > len(s) {
		return 0, false
	}
	var r rune
	for _, c := range []byte(s[i : i+4]) {
		r <<= 4
		switch {
		case c >= '0' && c <= '9':
			r |= rune(c - '0')
		case c >= 'a' && c <= 'f':
			r |= rune(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			r |= rune(c - 'A' + 10)
		default:
			return 0, false
		}
	}
	return r, true
}
