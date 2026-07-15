// SPDX-License-Identifier: MIT

package client

import (
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func TestParseAPIError_MessageField(t *testing.T) {
	u := mustURL(t, "https://api.coinbase.com/api/v3/brokerage/market/products?limit=1")
	e := parseAPIError("GET", u, 400, []byte(`{"message":"invalid limit"}`))
	if e.Message != "invalid limit" {
		t.Errorf("Message = %q, want 'invalid limit'", e.Message)
	}
	if e.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", e.StatusCode)
	}
	if e.URL != "/api/v3/brokerage/market/products?limit=1" {
		t.Errorf("URL = %q, want path+query", e.URL)
	}
}

func TestParseAPIError_ErrorString(t *testing.T) {
	// Coinbase error envelope: {"error": "...", "error_details": "...", "message": "..."}
	// with message empty and error a bare string.
	u := mustURL(t, "https://api.coinbase.com/x")
	e := parseAPIError("GET", u, 404, []byte(`{"error":"NOT_FOUND"}`))
	if e.Message != "NOT_FOUND" {
		t.Errorf("Message = %q, want NOT_FOUND", e.Message)
	}
}

func TestParseAPIError_ErrorObjectWithMessage(t *testing.T) {
	u := mustURL(t, "https://api.coinbase.com/x")
	e := parseAPIError("POST", u, 401, []byte(`{"error":{"message":"unauthorized"}}`))
	if e.Message != "unauthorized" {
		t.Errorf("Message = %q, want unauthorized", e.Message)
	}
}

func TestParseAPIError_ErrorObjectWithoutMessage(t *testing.T) {
	u := mustURL(t, "https://api.coinbase.com/x")
	e := parseAPIError("GET", u, 500, []byte(`{"error":{"code":13}}`))
	if e.Message != "" {
		t.Errorf("Message = %q, want empty", e.Message)
	}
	if !strings.Contains(e.Body, `"code":13`) {
		t.Errorf("Body = %q, want raw body preserved", e.Body)
	}
}

func TestParseAPIError_NonJSONBody(t *testing.T) {
	u := mustURL(t, "https://api.coinbase.com/x")
	e := parseAPIError("GET", u, 502, []byte("  Bad Gateway\n"))
	if e.Message != "" {
		t.Errorf("Message = %q, want empty for non-JSON", e.Message)
	}
	if e.Body != "Bad Gateway" {
		t.Errorf("Body = %q, want trimmed 'Bad Gateway'", e.Body)
	}
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name string
		e    APIError
		want string
	}{
		{
			"with message",
			APIError{Method: "GET", URL: "/x", StatusCode: 404, Message: "not found", Body: "{...}"},
			"GET /x -> HTTP 404: not found",
		},
		{
			"falls back to body",
			APIError{Method: "GET", URL: "/x", StatusCode: 500, Body: "raw body"},
			"GET /x -> HTTP 500: raw body",
		},
		{
			"no body at all",
			APIError{Method: "DELETE", URL: "/y", StatusCode: 503},
			"DELETE /y -> HTTP 503: (no response body)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 100); got != "short" {
		t.Errorf("truncate(short) = %q", got)
	}
	if got := truncate("abcdef", 3); got != "abc…" {
		t.Errorf("truncate = %q, want abc…", got)
	}
	// Never split a multi-byte rune.
	s := "aé" // 'é' is 2 bytes starting at index 1
	got := truncate(s, 2)
	if !utf8.ValidString(got) {
		t.Errorf("truncate produced invalid UTF-8: %q", got)
	}
	if got != "a…" {
		t.Errorf("truncate(aé, 2) = %q, want a…", got)
	}
	// Long bodies are capped.
	long := strings.Repeat("x", 3000)
	if got := truncate(long, 2000); len(got) != 2000+len("…") {
		t.Errorf("truncated length = %d", len(got))
	}
}
