package engine

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/cemilalp02/portcullis/internal/discover"
	"github.com/cemilalp02/portcullis/internal/extract"
	"github.com/cemilalp02/portcullis/internal/target"
)

// Hooks, tasks and lifecycle commands usually run a script that lives in the
// repository ("bash .claude/hooks/check.sh"), so the command alone does not
// show what runs. After the configuration files are analysed, Scan reads the
// scripts their commands name (through the same os.Root, so nothing outside
// the scan root is read) and analyses them as the run_by_config targets. Only
// one level is followed: scripts that those scripts run are not.

// maxRunScripts bounds the number of scripts read this way.
const maxRunScripts = 256

// projectDir matches a leading variable that names the project directory:
// Claude Code and Gemini CLI hooks, VS Code tasks, dev containers, npm.
var projectDir = regexp.MustCompile(`^\$(\{(CLAUDE_PROJECT_DIR|GEMINI_PROJECT_DIR|workspaceFolder|workspaceRoot|localWorkspaceFolder|containerWorkspaceFolder|PWD|INIT_CWD)\}|CLAUDE_PROJECT_DIR|GEMINI_PROJECT_DIR|PWD|INIT_CWD)/`)

// commandScripts returns the repository paths of the scripts that the
// commands of a config file run, '/'-separated and relative to the scan root.
// Relative paths are resolved against the project directory (the parent of a
// ".claude", ".vscode", ".devcontainer" or ".gemini" directory) and then
// against the file's own directory; Scan keeps the first that exists.
func commandScripts(ts *target.Set, configPath string, points []extract.ExecPoint) [][]string {
	if len(points) == 0 {
		return nil
	}
	dir := path.Dir(configPath)
	bases := []string{dir}
	if dir != "." && strings.HasPrefix(path.Base(dir), ".") {
		bases = []string{path.Dir(dir), dir}
	}
	var out [][]string
	for _, p := range points {
		for _, tok := range scriptTokens(p.Command, ts) {
			var alts []string
			for _, b := range bases {
				rel := path.Join(b, tok)
				if rel == ".." || strings.HasPrefix(rel, "../") {
					continue
				}
				alts = append(alts, rel)
			}
			if len(alts) > 0 {
				out = append(out, alts)
			}
		}
	}
	return out
}

// scriptTokens returns the relative script paths in a command line: words
// with a script extension, after quotes and a project-directory variable are
// removed. Absolute paths, URLs and words with other expansions are skipped.
func scriptTokens(cmd string, ts *target.Set) []string {
	cmd = strings.NewReplacer(`"`, "", `'`, "", `\`, "/").Replace(cmd)
	words := strings.FieldsFunc(cmd, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(";&|()<>`", r)
	})
	var out []string
	for _, w := range words {
		if i := strings.LastIndexByte(w, '='); i >= 0 { // --file=x.py
			w = w[i+1:]
		}
		w = projectDir.ReplaceAllString(w, "")
		if w == "" || strings.HasPrefix(w, "/") || strings.ContainsAny(w, "$*?[{~:%") || ts.RunByConfig(w) == nil {
			continue
		}
		out = append(out, w)
	}
	return out
}

// runScripts analyses the scripts named by config commands that were not
// already analysed as targets.
func (e *Engine) runScripts(r *os.Root, res *Result, refs [][]string, maxSize int64) {
	if len(refs) == 0 {
		return
	}
	known := make(map[string]bool, len(res.TargetFiles))
	for _, tf := range res.TargetFiles {
		known[tf.Path] = true
	}
	var cands []discover.Candidate
	for _, alts := range refs {
		for _, p := range alts {
			if known[p] {
				break
			}
			fi, err := r.Stat(filepath.FromSlash(p))
			if err != nil {
				continue
			}
			if fi.Mode().IsRegular() {
				known[p] = true
				cands = append(cands, discover.Candidate{Path: p, ReadPath: p, Target: e.Targets.RunByConfig(p), Size: fi.Size()})
			}
			break
		}
	}
	slices.SortFunc(cands, func(a, b discover.Candidate) int { return strings.Compare(a.Path, b.Path) })
	if len(cands) > maxRunScripts {
		cands = cands[:maxRunScripts]
	}
	for _, c := range cands {
		res.Targets++
		res.TargetFiles = append(res.TargetFiles, TargetFile{Path: c.Path, ReadPath: c.ReadPath, Kind: c.Target.Title})
		fr := e.scanCandidate(r, c, maxSize)
		res.Findings = append(res.Findings, fr.findings...)
		if fr.err != "" {
			res.fail(fr.path, fr.err)
		}
	}
}
