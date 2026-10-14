package main

import (
	"runtime/debug"
	"strings"
)

// resolveVersion returns the version caged reports. A build with a linked
// version (-ldflags "-X main.version=...", used by Makefile and GoReleaser)
// uses it. A plain `go install ...@v0.1.0` build has none, so it falls back to
// the module version in the build info, when that is a real version rather than
// "(devel)". A leading "v" is stripped, so every install channel prints the same
// text. Anything else is "dev".
func resolveVersion(linked string, info func() (*debug.BuildInfo, bool)) string {
	if linked != "dev" {
		return strings.TrimPrefix(linked, "v")
	}
	if build, ok := info(); ok && build != nil {
		if version := build.Main.Version; version != "" && version != "(devel)" {
			return strings.TrimPrefix(version, "v")
		}
	}
	return "dev"
}
