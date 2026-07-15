// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rangertaha/coinbase-mcp/internal/config"
)

// clearConfigEnv resets all config env vars for the test's duration.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{config.EnvAPIKey, config.EnvAPISecret, config.EnvBaseURL, config.EnvToolsets, config.EnvReadOnly} {
		t.Setenv(k, "")
	}
}

func TestCommandStructure(t *testing.T) {
	mcpCmd := mcpCommand()
	if mcpCmd.Name != "mcp" || mcpCmd.Action == nil {
		t.Errorf("mcp command malformed: %+v", mcpCmd)
	}
	testCmd := testCommand()
	if testCmd.Name != "test" || testCmd.Action == nil {
		t.Errorf("test command malformed: %+v", testCmd)
	}
}

func TestTestCommand_Success(t *testing.T) {
	clearConfigEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"num_products":42}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvBaseURL, srv.URL)

	// Capture stdout to keep test output clean and assert the summary.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	err := testCommand().Action(context.Background(), nil)

	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("test command: %v", err)
	}
	if !strings.Contains(string(out), "42 products") {
		t.Errorf("output = %q, want product count", out)
	}
	if !strings.Contains(string(out), "authenticated=false") {
		t.Errorf("output = %q, want authenticated=false", out)
	}
}

func TestTestCommand_ConnectionFailure(t *testing.T) {
	clearConfigEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // connection refused
	t.Setenv(config.EnvBaseURL, url)

	err := testCommand().Action(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "connecting to") {
		t.Fatalf("err = %v, want connection error", err)
	}
}

func TestTestCommand_ConfigError(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv(config.EnvAPIKey, "key-without-secret")

	err := testCommand().Action(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "configuration error") {
		t.Fatalf("err = %v, want configuration error", err)
	}
}

func TestTestCommand_InvalidBaseURL(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv(config.EnvBaseURL, "not-a-url")
	err := testCommand().Action(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}
}

func TestRun_VersionFlag(t *testing.T) {
	clearConfigEnv(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	var errOut strings.Builder
	code := run(context.Background(), []string{"coinbase", "--version"}, &errOut)

	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	if code != 0 {
		t.Errorf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if len(out) == 0 {
		t.Error("--version printed nothing")
	}
}

func TestRun_ErrorExitCode(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv(config.EnvAPIKey, "key-without-secret")

	var errOut strings.Builder
	code := run(context.Background(), []string{"coinbase", "test"}, &errOut)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "coinbase:") {
		t.Errorf("stderr = %q, want error prefixed with 'coinbase:'", errOut.String())
	}
}

func TestEnvFileReadErrorIsLoggedNotFatal(t *testing.T) {
	clearConfigEnv(t)
	// A .env that exists but cannot be scanned (it is a directory) must only
	// log a warning; the command still runs.
	dir := t.TempDir()
	if err := os.Mkdir(dir+"/.env", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"num_products":1}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvBaseURL, srv.URL)

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := testCommand().Action(context.Background(), nil)
	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("test command with unreadable .env: %v", err)
	}
}

func TestRunMCP_ConfigError(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv(config.EnvAPISecret, "secret-without-key")

	err := runMCP(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "configuration error") {
		t.Fatalf("err = %v, want configuration error", err)
	}
}

func TestRunMCP_ServesUntilStdinCloses(t *testing.T) {
	clearConfigEnv(t)
	// Run from a directory whose .env is unreadable to also cover runMCP's
	// env-file warning branch.
	dir := t.TempDir()
	if err := os.Mkdir(dir+"/.env", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(dir)

	// Feed the server an empty stdin so the stdio transport sees EOF and the
	// serve loop terminates.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldIn := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldIn; _ = r.Close() })
	_ = w.Close()

	done := make(chan error, 1)
	go func() { done <- runMCP(context.Background(), nil) }()

	select {
	case <-done:
		// Any return (nil or EOF-ish error) is fine; the point is it terminates.
	case <-time.After(10 * time.Second):
		t.Fatal("runMCP did not terminate after stdin EOF")
	}
}
