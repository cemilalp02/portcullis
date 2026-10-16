package engine

import (
	"regexp"
	"strings"
)

var (
	// ${IFS}, ${IFS%?}, $IFS: the field separator used as a space.
	ifsRE = regexp.MustCompile(`\$\{ifs[^}]*\}|\$ifs\b`)
	// A backslash before an ordinary character is dropped by the shell.
	escRE = regexp.MustCompile(`\\([a-z0-9_./-])`)
	// Empty quotes split a word without changing it: c''url, ba""sh.
	emptyQuotes = strings.NewReplacer("''", "", `""`, "")
)

// shellView undoes common shell obfuscation in a normalised command line so
// exec rules see the words a shell would run. It is only used for matching;
// findings still show the command as written.
func shellView(s string) string {
	s = ifsRE.ReplaceAllString(s, " ")
	s = escRE.ReplaceAllString(s, "$1")
	return emptyQuotes.Replace(s)
}
