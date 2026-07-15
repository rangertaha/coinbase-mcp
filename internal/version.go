// SPDX-License-Identifier: MIT

// Package internal holds build-wide values shared across coinbase-mcp, such as
// the server version reported to MCP clients.
package internal

import "runtime/debug"

// version is injected for release builds via:
//
//	-ldflags "-X github.com/rangertaha/coinbase-mcp/internal.version=v1.2.3"
//
// When empty (the common case for `go install` and source builds), Version
// derives a value from the build info instead.
var version string

// Version returns the server version, resolved in order of precedence:
//
//  1. the value injected at build time with -ldflags (release builds);
//  2. the module version from the build info, e.g. when installed with
//     `go install github.com/rangertaha/coinbase-mcp/cmd/coinbase@v1.2.3`;
//  3. a "dev" value annotated with the VCS revision when building from source.
func Version() string {
	bi, ok := debug.ReadBuildInfo()
	return resolveVersion(version, bi, ok)
}

// resolveVersion implements Version's precedence rules; split out so every
// branch is testable with synthetic build info.
func resolveVersion(injected string, bi *debug.BuildInfo, ok bool) string {
	if injected != "" {
		return injected
	}
	if !ok {
		return "dev"
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}

	// Building from a checkout: annotate "dev" with the VCS revision if the
	// toolchain stamped one (go build does so by default from a git tree).
	var rev, dirty string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if rev != "" {
		if len(rev) > 12 {
			rev = rev[:12]
		}
		return "dev-" + rev + dirty
	}
	return "dev"
}
