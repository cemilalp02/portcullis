package source

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// LineIndex maps byte offsets in a text to 1-based line and column numbers.
// Columns count Unicode code points, matching SARIF's "unicodeCodePoints".
type LineIndex struct {
	text   string
	starts []int
}

// NewLineIndex indexes the line starts of text. Lines end at '\n'.
func NewLineIndex(text string) *LineIndex {
	starts := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &LineIndex{text: text, starts: starts}
}

// Position returns the line and column of byte offset off (clamped to the text).
func (li *LineIndex) Position(off int) (line, col int) {
	off = max(0, min(off, len(li.text)))
	i := sort.Search(len(li.starts), func(i int) bool { return li.starts[i] > off }) - 1
	return i + 1, utf8.RuneCountInString(li.text[li.starts[i]:off]) + 1
}

// Count returns the number of lines.
func (li *LineIndex) Count() int { return len(li.starts) }

// Line returns the text of the 1-based line n without its line terminator, and
// the byte offset where it starts. Out-of-range lines return "" and -1.
func (li *LineIndex) Line(n int) (string, int) {
	if n < 1 || n > len(li.starts) {
		return "", -1
	}
	start := li.starts[n-1]
	end := len(li.text)
	if n < len(li.starts) {
		end = li.starts[n] - 1
	}
	return strings.TrimSuffix(li.text[start:end], "\r"), start
}

// Document is a decoded target file.
type Document struct {
	Path     string // relative to the scan root, '/'-separated
	Text     string // decoded UTF-8
	Encoding Encoding
	Lines    *LineIndex
}

// NewDocument decodes raw and indexes its lines.
func NewDocument(path string, raw []byte) *Document {
	text, enc := Decode(raw)
	return &Document{Path: path, Text: text, Encoding: enc, Lines: NewLineIndex(text)}
}
