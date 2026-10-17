package archtest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// siteSource maps a path of the published website to the repository file
// that scripts/site/build.sh copies there.
func siteSource(root, ref string) string {
	switch {
	case strings.HasPrefix(ref, "fonts/"):
		return filepath.Join(root, "internal", "report", "html", "fonts", strings.TrimPrefix(ref, "fonts/"))
	case ref == "assets/favicon.svg":
		return filepath.Join(root, "docs", "assets", "brand", "favicon.svg")
	case strings.HasPrefix(ref, "assets/"):
		return filepath.Join(root, "docs", "assets", strings.TrimPrefix(ref, "assets/"))
	}
	return filepath.Join(root, "site", filepath.FromSlash(ref))
}

var (
	htmlRef = regexp.MustCompile(`(?:href|src|srcset)="([^"]+)"`)
	cssRef  = regexp.MustCompile(`url\("([^"]+)"\)`)
)

// TestSiteReferences checks that every local file the website refers to is
// published by scripts/site/build.sh, and that the page loads nothing from
// other hosts (its Content-Security-Policy would block it anyway).
func TestSiteReferences(t *testing.T) {
	root := filepath.Join("..", "..")
	build, err := os.ReadFile(filepath.Join(root, "scripts", "site", "build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct {
		name string
		re   *regexp.Regexp
	}{{"index.html", htmlRef}, {"style.css", cssRef}} {
		data, err := os.ReadFile(filepath.Join(root, "site", f.name))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range f.re.FindAllStringSubmatch(string(data), -1) {
			ref := m[1]
			switch {
			case strings.HasPrefix(ref, "https://github.com/cemilalp02/portcullis"), strings.HasPrefix(ref, "#"), ref == "./":
				continue
			case strings.Contains(ref, "://"), strings.HasPrefix(ref, "//"):
				if f.name == "style.css" || !strings.HasPrefix(m[0], "href") {
					t.Errorf("%s loads %s from another host", f.name, ref)
				}
				continue
			}
			src := siteSource(root, ref)
			if _, err := os.Stat(src); err != nil {
				t.Errorf("%s refers to %s, but %s does not exist", f.name, ref, src)
			}
			copied := filepath.Base(src)
			if strings.HasSuffix(copied, ".woff2") {
				copied = "*.woff2"
			}
			if !strings.Contains(string(build), copied) {
				t.Errorf("%s refers to %s, which scripts/site/build.sh does not copy", f.name, ref)
			}
		}
	}
}
