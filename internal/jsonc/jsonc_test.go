package jsonc

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// toAny converts a node to the value encoding/json would produce.
func toAny(n *Node) any {
	switch n.Kind {
	case Null:
		return nil
	case Bool:
		return n.Bool
	case Number:
		f, _ := strconv.ParseFloat(n.Str, 64)
		return f
	case String:
		return n.Str
	case Array:
		out := make([]any, 0, len(n.Elems))
		for _, e := range n.Elems {
			out = append(out, toAny(e))
		}
		return out
	default:
		out := make(map[string]any, len(n.Members))
		for _, m := range n.Members {
			out[m.Key] = toAny(m.Value) // last duplicate wins, like encoding/json
		}
		return out
	}
}

func TestParseJSONC(t *testing.T) {
	src := `// tasks for this folder
{
  /* block
     comment */
  "version": "2.0.0",
  "tasks": [
    { "label": "setup", "command": "node scripts/setup.js", "runOptions": { "runOn": "folderOpen" }, },
  ],
  "n": -12.5e3, "t": true, "f": false, "z": null,
  "esc": "a\"b\\c\/d\n\u00e7\ud83d\ude00",
}
`
	n, dups, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(dups) != 0 {
		t.Errorf("dups = %v", dups)
	}
	tasks := n.Get("tasks")
	if tasks == nil || tasks.Kind != Array || len(tasks.Elems) != 1 {
		t.Fatalf("tasks = %+v", tasks)
	}
	cmd, ok := tasks.Elems[0].Get("command").StringValue()
	if !ok || cmd != "node scripts/setup.js" {
		t.Errorf("command = %q, %v", cmd, ok)
	}
	cmdNode := tasks.Elems[0].Get("command")
	if got := src[cmdNode.Start:cmdNode.End]; got != `"node scripts/setup.js"` {
		t.Errorf("command span = %q", got)
	}
	if v := n.Get("esc").Str; v != "a\"b\\c/d\nç😀" {
		t.Errorf("esc = %q", v)
	}
	if v := n.Get("n"); v.Kind != Number || v.Str != "-12.5e3" {
		t.Errorf("n = %+v", v)
	}
	if !n.Get("t").Bool || n.Get("f").Bool || n.Get("z").Kind != Null {
		t.Error("literals parsed wrongly")
	}
	if n.Get("missing") != nil || tasks.Get("x") != nil {
		t.Error("Get on missing key / non-object returned a node")
	}
	if _, ok := n.Get("n").StringValue(); ok {
		t.Error("StringValue on number returned ok")
	}
}

func TestDuplicateKeys(t *testing.T) {
	src := `{"hooks": {}, "a": {"x": 1, "x": 2}, "hooks": {"evil": true}}`
	n, dups, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(dups) != 2 {
		t.Fatalf("dups = %+v, want 2", dups)
	}
	keys := map[string]DuplicateKey{}
	for _, d := range dups {
		keys[d.Key] = d
	}
	h := keys["hooks"]
	if src[h.Offset:h.Offset+7] != `"hooks"` || h.FirstOffset != 1 || h.Offset <= h.FirstOffset {
		t.Errorf("hooks dup = %+v", h)
	}
	if n.Get("hooks").Get("evil") == nil {
		t.Error("Get should return the last duplicate (what consumers use)")
	}
}

func TestSyntaxErrors(t *testing.T) {
	tests := []struct {
		src     string
		wantMsg string
	}{
		{"", "empty document"},
		{"   // only a comment", "empty document"},
		{"{", "unterminated object"},
		{"[1,", "unterminated array"},
		{`{"a" 1}`, "expected ':'"},
		{`{"a":}`, "unexpected character '}'"},
		{`{a: 1}`, "expected object key string"},
		{`[1 2]`, "expected ','"},
		{`"abc`, "unterminated string"},
		{"\"a\x01b\"", "control character"},
		{`"\x"`, "invalid escape"},
		{`"\u12G4"`, "invalid unicode escape"},
		{`01`, "unexpected character '1' after top-level value"},
		{`-`, "invalid number"},
		{`1.`, "missing fraction digits"},
		{`1e+`, "missing exponent digits"},
		{`/* never closed`, "unterminated block comment"},
		{`tru`, "unexpected character 't'"},
		{`{} {}`, "after top-level value"},
		{`{"a":1,}x`, "after top-level value"},
		{`"\`, "unterminated escape"},
		{`{"a":`, "missing value"},
		{`[1,]`, ""},
	}
	for _, tt := range tests {
		_, _, err := Parse(tt.src)
		if tt.wantMsg == "" {
			if err != nil {
				t.Errorf("Parse(%q) = %v, want success", tt.src, err)
			}
			continue
		}
		var se *SyntaxError
		if !errors.As(err, &se) || !strings.Contains(se.Error(), tt.wantMsg) {
			t.Errorf("Parse(%q) error = %v, want containing %q", tt.src, err, tt.wantMsg)
		}
	}
}

func TestDepthLimit(t *testing.T) {
	ok := strings.Repeat("[", MaxDepth) + strings.Repeat("]", MaxDepth)
	if _, _, err := Parse(ok); err != nil {
		t.Errorf("depth %d rejected: %v", MaxDepth, err)
	}
	deep := strings.Repeat("[", MaxDepth+1) + strings.Repeat("]", MaxDepth+1)
	if _, _, err := Parse(deep); err == nil || !strings.Contains(err.Error(), "nesting deeper") {
		t.Errorf("depth %d accepted or wrong error: %v", MaxDepth+1, err)
	}
}

func TestLoneSurrogate(t *testing.T) {
	n, _, err := Parse(`"\ud800x"`)
	if err != nil || n.Str != string(utf8.RuneError)+"x" {
		t.Errorf("got %q, %v", n.Str, err)
	}
}

func TestKindString(t *testing.T) {
	if Object.String() != "object" || Null.String() != "null" {
		t.Error("Kind.String")
	}
}

// FuzzParse checks that Parse never panics and that, for every input that
// encoding/json accepts, Parse accepts it too and yields the same value.
func FuzzParse(f *testing.F) {
	seeds := []string{
		`{}`, `[]`, `{"a":[1,2,{"b":null}],"c":"\u00e7"}`, `// c` + "\n" + `{"a":1,}`,
		`{"a":1,"a":2}`, `"\ud83d\ude00"`, `-0.5E-10`, `[[[[]]]]`, `{"a":"\""}`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		n, _, err := Parse(src)
		if !utf8.ValidString(src) || !json.Valid([]byte(src)) {
			return
		}
		var want any
		if json.Unmarshal([]byte(src), &want) != nil {
			return // e.g. numbers out of float64 range
		}
		if err != nil {
			t.Fatalf("encoding/json accepts %q but Parse fails: %v", src, err)
		}
		if got := toAny(n); !reflect.DeepEqual(got, want) {
			t.Fatalf("value mismatch for %q:\n got %#v\nwant %#v", src, got, want)
		}
	})
}
