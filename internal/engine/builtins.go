package engine

import (
	"regexp"
	"strings"
	"unicode"
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

// wordSeparator reports whether text[start:end] is made of zero-width spaces
// next to a letter of a script that writes no spaces between words (Khmer,
// Thai, Lao, Myanmar, Tibetan), where U+200B is the normal word separator.
// The other side is such a letter too, or a space, punctuation or the end of
// the text: never a letter of another script.
func wordSeparator(text string, start, end int) bool {
	for _, r := range text[start:end] {
		if r != 0x200B {
			return false
		}
	}
	prev, _ := utf8.DecodeLastRuneInString(text[:start])
	next, _ := utf8.DecodeRuneInString(text[end:])
	return noSpaceScript(prev) && (noSpaceScript(next) || boundary(next)) ||
		noSpaceScript(next) && boundary(prev)
}

func boundary(r rune) bool {
	return r == utf8.RuneError || unicode.IsSpace(r) || unicode.IsPunct(r)
}

func noSpaceScript(r rune) bool {
	return unicode.In(r, unicode.Khmer, unicode.Thai, unicode.Lao, unicode.Myanmar, unicode.Tibetan)
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

var (
	urlHost     = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://([^/\s"'` + "`" + `)<>]+)`)
	localHost   = regexp.MustCompile(`(?i)^([^@]*@)?(localhost|127\.\d+\.\d+\.\d+|0\.0\.0\.0|\[::1\]|([a-z0-9-]+\.)*example\.(com|org|net)|[a-z0-9.-]+\.localhost)(:\d+)?$`)
	localMarker = regexp.MustCompile(`(?i)\b(localhost|127\.0\.0\.1|0\.0\.0\.0)\b|\[::1\]`)
	remoteSpec  = regexp.MustCompile(`(?i)rsync://|\s[\w.@-]+:[~/\w$]`)
	relFetch    = regexp.MustCompile(`^\s*[` + "`" + `'"](/|\.|\$\{)`)
	axiosCall   = regexp.MustCompile(`^\.(get|post|put|patch|delete|head|request)\(`)
)

// localNetwork reports whether the network use at body[loc[0]:loc[1]] stays
// on this machine or is an example: every URL on its line points to
// localhost or a documentation domain (example.com, RFC 2606; the defanged
// attack samples use .invalid, which stays remote), or the line names
// localhost and no URL,
// rsync copies between local directories, or fetch() takes a relative URL
// (application code shown in the skill, not something the skill does).
func localNetwork(body string, loc []int) bool {
	ls := strings.LastIndexByte(body[:loc[0]], '\n') + 1
	le := strings.IndexByte(body[loc[1]:], '\n')
	if le < 0 {
		le = len(body)
	} else {
		le += loc[1]
	}
	line, word := body[ls:le], strings.ToLower(body[loc[0]:loc[1]])
	switch word {
	case "rsync":
		return !remoteSpec.MatchString(line)
	case "fetch(":
		if relFetch.MatchString(body[loc[1]:le]) {
			return true
		}
	case "axios":
		if m := axiosCall.FindStringIndex(body[loc[1]:le]); m != nil && relFetch.MatchString(body[loc[1]+m[1]:le]) {
			return true
		}
	}
	hosts := urlHost.FindAllStringSubmatch(line, -1)
	if len(hosts) == 0 {
		return localMarker.MatchString(line)
	}
	for _, h := range hosts {
		if !localHost.MatchString(h[1]) {
			return false
		}
	}
	return true
}

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
	var loc []int
	for _, l := range networkUse.FindAllStringIndex(body, -1) {
		if !localNetwork(body, l) {
			loc = l
			break
		}
	}
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
