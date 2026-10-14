package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	t.Parallel()

	buildInfo := func(version string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: version}}, true
		}
	}
	none := func() (*debug.BuildInfo, bool) { return nil, false }
	tests := []struct {
		name   string
		linked string
		info   func() (*debug.BuildInfo, bool)
		want   string
	}{
		{name: "linked by ldflags", linked: "0.1.0", info: buildInfo("v9.9.9"), want: "0.1.0"},
		{name: "linked with a leading v", linked: "v0.1.0", info: buildInfo("v9.9.9"), want: "0.1.0"},
		{name: "linked from git describe", linked: "0.1.0-3-gabc1234-dirty", info: none, want: "0.1.0-3-gabc1234-dirty"},
		{name: "test version", linked: "test", info: none, want: "test"},
		{name: "go install of a tag", linked: "dev", info: buildInfo("v0.1.0"), want: "0.1.0"},
		{name: "go install of a release candidate", linked: "dev", info: buildInfo("v0.1.0-rc.1"), want: "0.1.0-rc.1"},
		{name: "go install of a pseudo-version", linked: "dev", info: buildInfo("v0.0.0-20261014120000-abcdef123456"), want: "0.0.0-20261014120000-abcdef123456"},
		{name: "a development build", linked: "dev", info: buildInfo("(devel)"), want: "dev"},
		{name: "build info without a version", linked: "dev", info: buildInfo(""), want: "dev"},
		{name: "no build info", linked: "dev", info: none, want: "dev"},
		{name: "build info that says ok but is nil", linked: "dev", info: func() (*debug.BuildInfo, bool) { return nil, true }, want: "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveVersion(tt.linked, tt.info); got != tt.want {
				t.Fatalf("resolveVersion(%q) = %q, want %q", tt.linked, got, tt.want)
			}
		})
	}
}
