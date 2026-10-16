package engine

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/cemilalp02/portcullis/internal/ruleset"
)

// decodeTags turns a run of Unicode Tag characters back into the ASCII text
// it shadows (U+E0041 -> "A"), which is what an LLM reads. Begin/cancel tags
// are dropped; anything else becomes '?'.
func decodeTags(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 0xE0020 && r <= 0xE007E:
			b.WriteByte(byte(r - 0xE0000))
		case r == 0xE0001 || r == 0xE007F:
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

// emojiTagSequence reports whether text[start:end] is the tag sequence of an
// emoji subdivision flag: after U+1F3F4, three to six lowercase tag letters or
// digits and a CANCEL TAG, as in the flags of England, Scotland and Wales
// (Unicode TS #51). Six lowercase characters cannot carry an instruction.
func emojiTagSequence(text string, start, end int) bool {
	if prev, _ := utf8.DecodeLastRuneInString(text[:start]); prev != 0x1F3F4 {
		return false
	}
	runes := []rune(text[start:end])
	if len(runes) < 4 || len(runes) > 7 || runes[len(runes)-1] != 0xE007F {
		return false
	}
	for _, r := range runes[:len(runes)-1] {
		if (r < 0xE0030 || r > 0xE0039) && (r < 0xE0061 || r > 0xE007A) {
			return false
		}
	}
	return true
}

// frontMatter splits a Markdown file with YAML front-matter ("---" lines)
// into a few scalar fields and the byte offset where the body starts. It is
// deliberately not a YAML parser: scanned content never reaches one.
func frontMatter(text string) (fields map[string]string, bodyStart int) {
	fields = map[string]string{}
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return fields, 0
	}
	lines := strings.SplitAfter(text, "\n")
	off := len(lines[0])
	key := ""
	for _, line := range lines[1:] {
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "---" {
			return fields, off + len(line)
		}
		off += len(line)
		switch {
		case key != "" && (strings.HasPrefix(trimmed, " ") || strings.HasPrefix(trimmed, "\t")):
			fields[key] = strings.TrimSpace(fields[key] + " " + strings.TrimSpace(trimmed))
		default:
			k, v, ok := strings.Cut(trimmed, ":")
			if !ok {
				key = ""
				continue
			}
			key = strings.ToLower(strings.TrimSpace(k))
			v = strings.TrimSpace(v)
			if v == ">" || v == "|" || v == ">-" || v == "|-" {
				v = ""
			}
			fields[key] = strings.Trim(v, `"'`)
		}
	}
	return map[string]string{}, 0 // no closing marker: not front-matter
}

var (
	// Network commands and client libraries. Plain URLs are not counted: skills
	// link to documentation all the time.
	networkUse = regexp.MustCompile(`(?i)\b(curl|wget|invoke-webrequest|invoke-restmethod|iwr|irm|scp|rsync|ncat|ftp)\b|\brequests\.(get|post|put)\b|\burllib\b|\bhttp\.client\b|\bfetch\(|\baxios\b`)
	networkDoc = regexp.MustCompile(`(?i)\b(download|upload|fetch|https?|url|urls|api|apis|web|website|internet|network|online|remote|cloud|server|sync|deploy|publish|install|github|git|search|browse|request|endpoint|webhook|scrape|crawl)`)
)

// skillCapabilityMismatch reports a SKILL.md whose body uses the network
// while its name and description say nothing about it: the user installed a
// "markdown formatter", not something that talks to the internet
// (REQUIREMENTS D4, THREAT_MODEL T7).
func skillCapabilityMismatch(c *collector, r *ruleset.Compiled) {
	fields, bodyStart := frontMatter(c.doc.Text)
	declared := fields["name"] + " " + fields["description"]
	if networkDoc.MatchString(declared) {
		return
	}
	body := c.doc.Text[bodyStart:]
	loc := networkUse.FindStringIndex(body)
	if loc == nil {
		return
	}
	start, end := bodyStart+loc[0], bodyStart+loc[1]
	name := fields["name"]
	if name == "" {
		name = c.doc.Path
	}
	c.add(r, start, end, c.doc.Text[start:end], map[string]string{"name": name, "match": c.doc.Text[start:end]})
}
