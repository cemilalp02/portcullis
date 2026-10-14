// Package source decodes target files into UTF-8 text and maps byte offsets
// to line and column positions (REQUIREMENTS FR-05, FR-11).
package source

import (
	"bytes"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Encoding names the byte encoding a file was decoded from.
type Encoding string

// Encodings Portcullis recognises.
const (
	UTF8         Encoding = "utf-8"
	UTF8BOM      Encoding = "utf-8 with BOM"
	UTF16LE      Encoding = "utf-16le"
	UTF16BE      Encoding = "utf-16be"
	UTF16LENoBOM Encoding = "utf-16le without BOM"
	UTF16BENoBOM Encoding = "utf-16be without BOM"
	InvalidUTF8  Encoding = "invalid utf-8"
)

// Unusual reports whether the encoding is unexpected for agent context files
// and therefore worth reporting (FR-05).
func (e Encoding) Unusual() bool { return e != UTF8 && e != UTF8BOM }

// Decode converts raw file bytes to valid UTF-8 text. Invalid sequences become
// U+FFFD so that later stages never see broken input.
func Decode(b []byte) (string, Encoding) {
	switch {
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		rest := b[3:]
		if utf8.Valid(rest) {
			return string(rest), UTF8BOM
		}
		return toValidUTF8(rest), InvalidUTF8
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return decodeUTF16(b[2:], false), UTF16LE
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return decodeUTF16(b[2:], true), UTF16BE
	}
	if enc, ok := sniffUTF16(b); ok {
		return decodeUTF16(b, enc == UTF16BENoBOM), enc
	}
	if utf8.Valid(b) {
		return string(b), UTF8
	}
	return toValidUTF8(b), InvalidUTF8
}

// sniffUTF16 detects BOM-less UTF-16 text by its pattern of zero bytes. An
// attacker can save an instruction file this way so that byte-level pattern
// rules never see the ASCII text an agent may still decode.
func sniffUTF16(b []byte) (Encoding, bool) {
	if len(b) < 4 {
		return "", false
	}
	pairs := len(b) / 2
	var zeroEven, zeroOdd int
	for i := 0; i+1 < len(b); i += 2 {
		if b[i] == 0 {
			zeroEven++
		}
		if b[i+1] == 0 {
			zeroOdd++
		}
	}
	switch {
	case zeroOdd*10 >= pairs*4 && zeroEven*20 <= pairs:
		return UTF16LENoBOM, true
	case zeroEven*10 >= pairs*4 && zeroOdd*20 <= pairs:
		return UTF16BENoBOM, true
	}
	return "", false
}

func decodeUTF16(b []byte, bigEndian bool) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		if bigEndian {
			u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
		} else {
			u = append(u, uint16(b[i+1])<<8|uint16(b[i]))
		}
	}
	s := string(utf16.Decode(u))
	if len(b)%2 == 1 {
		s += string(utf8.RuneError)
	}
	return s
}

func toValidUTF8(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		sb.WriteRune(r) // RuneError for invalid bytes
		b = b[size:]
	}
	return sb.String()
}
