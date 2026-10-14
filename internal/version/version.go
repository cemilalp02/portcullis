// Package version reports build metadata for the bekci binary.
package version

import (
	"runtime"
	"runtime/debug"
)

// Release builds inject these with:
//
//	-ldflags "-X github.com/cemilalp02/bekci/internal/version.version=v0.1.0
//	          -X github.com/cemilalp02/bekci/internal/version.commit=<sha>
//	          -X github.com/cemilalp02/bekci/internal/version.date=<RFC 3339>"
//
// When they are empty, Get falls back to the Go build information, which is
// populated for `go install` and for builds inside a VCS checkout.
var (
	version string
	commit  string
	date    string
)

// Ruleset is the version of the embedded default rule set. It moves to the
// rules package once rules are embedded (Phase 3).
const Ruleset = "unreleased"

// Info describes the running binary.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"goVersion"`
	Ruleset   string `json:"ruleset"`
}

// Get returns the build metadata of the running binary.
func Get() Info {
	info := Info{
		Version:   version,
		Commit:    commit,
		Date:      date,
		GoVersion: runtime.Version(),
		Ruleset:   Ruleset,
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		fillFromBuildInfo(&info, bi)
	}
	if info.Version == "" {
		info.Version = "dev"
	}
	if info.Commit == "" {
		info.Commit = "unknown"
	}
	if info.Date == "" {
		info.Date = "unknown"
	}
	return info
}

func fillFromBuildInfo(info *Info, bi *debug.BuildInfo) {
	if info.Version == "" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		info.Version = bi.Main.Version
	}
	dirty, fromVCS := false, false
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if info.Commit == "" {
				info.Commit = s.Value
				fromVCS = true
			}
		case "vcs.time":
			if info.Date == "" {
				info.Date = s.Value
			}
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if dirty && fromVCS {
		info.Commit += "-dirty"
	}
}
