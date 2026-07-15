// SPDX-License-Identifier: MIT

package internal

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestVersion_Injected(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	version = "v9.9.9"
	if got := Version(); got != "v9.9.9" {
		t.Errorf("Version() = %q, want injected v9.9.9", got)
	}
}

func TestVersion_Fallback(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	version = ""
	got := Version()
	if got == "" {
		t.Fatal("Version() must never be empty")
	}
	// In a test binary the module version is "" or "(devel)" and no VCS info is
	// stamped, so the fallback is "dev" (possibly revision-annotated).
	if got != "dev" && !strings.HasPrefix(got, "dev-") && !strings.HasPrefix(got, "v") {
		t.Errorf("Version() = %q, want dev, dev-<rev>, or a module version", got)
	}
}

func TestResolveVersion(t *testing.T) {
	bi := func(modVersion string, settings ...debug.BuildSetting) *debug.BuildInfo {
		return &debug.BuildInfo{
			Main:     debug.Module{Version: modVersion},
			Settings: settings,
		}
	}
	set := func(k, v string) debug.BuildSetting { return debug.BuildSetting{Key: k, Value: v} }

	tests := []struct {
		name     string
		injected string
		bi       *debug.BuildInfo
		ok       bool
		want     string
	}{
		{"injected wins", "v1.2.3", bi("v9.9.9"), true, "v1.2.3"},
		{"no build info", "", nil, false, "dev"},
		{"module version", "", bi("v2.0.1"), true, "v2.0.1"},
		{"devel module version ignored", "", bi("(devel)"), true, "dev"},
		{"empty module version", "", bi(""), true, "dev"},
		{
			"vcs revision annotated",
			"", bi("(devel)", set("vcs.revision", "abcdef1234567890"), set("vcs.modified", "false")), true,
			"dev-abcdef123456",
		},
		{
			"short revision kept whole",
			"", bi("", set("vcs.revision", "abc123")), true,
			"dev-abc123",
		},
		{
			"dirty tree annotated",
			"", bi("", set("vcs.revision", "abcdef1234567890"), set("vcs.modified", "true")), true,
			"dev-abcdef123456-dirty",
		},
		{
			"unrelated settings ignored",
			"", bi("", set("GOOS", "linux")), true,
			"dev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.injected, tt.bi, tt.ok); got != tt.want {
				t.Errorf("resolveVersion = %q, want %q", got, tt.want)
			}
		})
	}
}
