package version

import (
	"runtime"
	"runtime/debug"
	"testing"
)

func TestGetHasNoEmptyFields(t *testing.T) {
	info := Get()
	for name, v := range map[string]string{
		"Version":   info.Version,
		"Commit":    info.Commit,
		"Date":      info.Date,
		"GoVersion": info.GoVersion,
		"Ruleset":   info.Ruleset,
	} {
		if v == "" {
			t.Errorf("%s is empty", name)
		}
	}
	if info.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want %q", info.GoVersion, runtime.Version())
	}
}

func TestFillFromBuildInfo(t *testing.T) {
	tests := []struct {
		name string
		in   Info
		bi   debug.BuildInfo
		want Info
	}{
		{
			name: "module version and vcs settings",
			bi: debug.BuildInfo{
				Main: debug.Module{Version: "v0.1.0"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abc123"},
					{Key: "vcs.time", Value: "2026-10-14T00:00:00Z"},
					{Key: "vcs.modified", Value: "false"},
				},
			},
			want: Info{Version: "v0.1.0", Commit: "abc123", Date: "2026-10-14T00:00:00Z"},
		},
		{
			name: "devel version is ignored and dirty tree is marked",
			bi: debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abc123"},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			want: Info{Commit: "abc123-dirty"},
		},
		{
			name: "ldflags values win over build info",
			in:   Info{Version: "v9.9.9", Commit: "fromldflags", Date: "d"},
			bi: debug.BuildInfo{
				Main:     debug.Module{Version: "v0.1.0"},
				Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}},
			},
			want: Info{Version: "v9.9.9", Commit: "fromldflags", Date: "d"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in
			fillFromBuildInfo(&got, &tt.bi)
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
