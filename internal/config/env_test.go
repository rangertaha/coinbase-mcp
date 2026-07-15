// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeEnvFile writes content to a temp file and returns its path.
func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	return path
}

func TestLoadEnvFile_MissingFileIsNotAnError(t *testing.T) {
	if err := LoadEnvFile(filepath.Join(t.TempDir(), "does-not-exist")); err != nil {
		t.Errorf("missing file: %v, want nil", err)
	}
}

func TestLoadEnvFile_UnreadableFile(t *testing.T) {
	// A directory opens fine but fails on read... actually os.Open succeeds on
	// dirs and Scan returns an error. Use that to hit the sc.Err() path.
	if err := LoadEnvFile(t.TempDir()); err == nil {
		t.Error("expected error reading a directory")
	}
}

func TestLoadEnvFile_ParsesEntries(t *testing.T) {
	content := `
# comment line
TEST_ENVFILE_A=plain
export TEST_ENVFILE_B=exported
TEST_ENVFILE_C="double quoted"
TEST_ENVFILE_D='single quoted'
TEST_ENVFILE_E=has=equals
TEST_ENVFILE_F=  spaced
malformed-line-no-equals
=no-key
TEST_ENVFILE_G="mismatched'
`
	for _, k := range []string{"TEST_ENVFILE_A", "TEST_ENVFILE_B", "TEST_ENVFILE_C", "TEST_ENVFILE_D", "TEST_ENVFILE_E", "TEST_ENVFILE_F", "TEST_ENVFILE_G"} {
		t.Setenv(k, "") // register cleanup, start unset-ish
		_ = os.Unsetenv(k)
	}

	if err := LoadEnvFile(writeEnvFile(t, content)); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}

	want := map[string]string{
		"TEST_ENVFILE_A": "plain",
		"TEST_ENVFILE_B": "exported",
		"TEST_ENVFILE_C": "double quoted",
		"TEST_ENVFILE_D": "single quoted",
		"TEST_ENVFILE_E": "has=equals",
		"TEST_ENVFILE_F": "spaced",
		"TEST_ENVFILE_G": `"mismatched'`,
	}
	for k, w := range want {
		if got := os.Getenv(k); got != w {
			t.Errorf("%s = %q, want %q", k, got, w)
		}
	}
}

func TestLoadEnvFile_ExistingValueWins(t *testing.T) {
	t.Setenv("TEST_ENVFILE_KEEP", "from-shell")
	if err := LoadEnvFile(writeEnvFile(t, "TEST_ENVFILE_KEEP=from-file")); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if got := os.Getenv("TEST_ENVFILE_KEEP"); got != "from-shell" {
		t.Errorf("existing value overwritten: %q", got)
	}
}

func TestLoadEnvFile_EmptyExistingValueIsReplaced(t *testing.T) {
	t.Setenv("TEST_ENVFILE_EMPTY", "")
	if err := LoadEnvFile(writeEnvFile(t, "TEST_ENVFILE_EMPTY=filled")); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if got := os.Getenv("TEST_ENVFILE_EMPTY"); got != "filled" {
		t.Errorf("empty value not replaced: %q", got)
	}
}

func TestLoadEnvFile_StripsUTF8BOM(t *testing.T) {
	t.Setenv("TEST_ENVFILE_BOM", "")
	_ = os.Unsetenv("TEST_ENVFILE_BOM")
	if err := LoadEnvFile(writeEnvFile(t, "\ufeffTEST_ENVFILE_BOM=value")); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if got := os.Getenv("TEST_ENVFILE_BOM"); got != "value" {
		t.Errorf("TEST_ENVFILE_BOM = %q, want value (BOM not stripped)", got)
	}
}

func TestLoadEnvFile_OpenError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	path := writeEnvFile(t, "KEY=value")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if err := LoadEnvFile(path); err == nil {
		t.Error("expected permission error")
	}
}

func TestLoadEnvFile_SetenvError(t *testing.T) {
	// A key containing a NUL byte is rejected by the OS; LoadEnvFile must
	// propagate the failure.
	if err := LoadEnvFile(writeEnvFile(t, "BAD\x00KEY=value")); err == nil {
		t.Error("expected Setenv error for NUL in key")
	}
}

func TestUnquote(t *testing.T) {
	tests := []struct{ in, want string }{
		{`"quoted"`, "quoted"},
		{`'quoted'`, "quoted"},
		{`unquoted`, "unquoted"},
		{`"mismatched'`, `"mismatched'`},
		{`"`, `"`},
		{`""`, ""},
		{``, ``},
	}
	for _, tt := range tests {
		if got := unquote(tt.in); got != tt.want {
			t.Errorf("unquote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
