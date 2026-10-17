package engine

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

// gitCryptHeader starts every file git-crypt encrypts; a 12-byte nonce and
// the AES-CTR ciphertext follow.
var gitCryptHeader = []byte("\x00GITCRYPT\x00")

// minCiphertext is the shortest content after the header that is treated as
// encrypted, and maxTextRun the longest run of readable characters such
// content may hold. Random bytes almost never contain 16 readable characters
// in a row (about once in ten million positions); text an agent could follow,
// in any script, always does.
const (
	minCiphertext = 32
	maxTextRun    = 16
)

// encrypted reports whether raw is a git-crypt encrypted file. Its content
// cannot be analysed, but it is reported rather than matched as text: random
// bytes contain control characters and invalid sequences that rules would
// report. A forged header in front of readable text is not encrypted.
func encrypted(raw []byte) bool {
	payload, ok := bytes.CutPrefix(raw, gitCryptHeader)
	return ok && len(payload) >= minCiphertext && longestTextRun(payload) < maxTextRun
}

// longestTextRun is the longest run of valid, printable UTF-8 characters
// (letters, digits, punctuation, symbols and spaces of any script).
func longestTextRun(b []byte) int {
	longest, run := 0, 0
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		b = b[size:]
		if (r != utf8.RuneError || size > 1) && (unicode.IsPrint(r) || r == '\t' || r == '\n') {
			run++
			longest = max(longest, run)
			continue
		}
		run = 0
	}
	return longest
}
