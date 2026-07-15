// SPDX-License-Identifier: MIT

package client

import (
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzJoinPath(f *testing.F) {
	f.Add("", "")
	f.Add("/base", "rel")
	f.Add("/base/", "/rel/")
	f.Add("//", "//")
	f.Fuzz(func(t *testing.T, base, rel string) {
		got := joinPath(base, rel)
		if got == "" {
			t.Fatalf("joinPath(%q, %q) = empty", base, rel)
		}
		if strings.Contains(got, "//") && !strings.Contains(base+rel, "//") {
			t.Errorf("joinPath(%q, %q) = %q introduced a double slash", base, rel, got)
		}
	})
}

func FuzzTruncate(f *testing.F) {
	f.Add("hello", 3)
	f.Add("aé漢🎉", 5)
	f.Add("", 0)
	f.Add("x", -1)
	f.Fuzz(func(t *testing.T, s string, n int) {
		if n < 0 || n > 1<<20 {
			t.Skip()
		}
		got := truncate(s, n)
		if len(got) > n+len("…") {
			t.Errorf("truncate(%q, %d) = %d bytes, exceeds limit", s, n, len(got))
		}
		if utf8.ValidString(s) && !utf8.ValidString(got) {
			t.Errorf("truncate(%q, %d) = %q produced invalid UTF-8 from valid input", s, n, got)
		}
	})
}

func FuzzParseAPIError(f *testing.F) {
	f.Add(`{"message":"m"}`, 400)
	f.Add(`{"error":"s"}`, 404)
	f.Add(`{"error":{"message":"m"}}`, 500)
	f.Add(`{"error":123}`, 500)
	f.Add(`not json at all`, 502)
	f.Add(``, 503)
	f.Add(`{"error":null,"message":null}`, 500)
	f.Fuzz(func(t *testing.T, body string, status int) {
		u, err := url.Parse("https://api.coinbase.com/x?a=1")
		if err != nil {
			t.Fatal(err)
		}
		e := parseAPIError("GET", u, status, []byte(body)) // must never panic
		if e == nil {
			t.Fatal("parseAPIError returned nil")
		}
		if e.StatusCode != status {
			t.Errorf("StatusCode = %d, want %d", e.StatusCode, status)
		}
		if e.Error() == "" {
			t.Error("Error() must never be empty")
		}
		if len(e.Body) > 2000+len("…") {
			t.Errorf("Body not truncated: %d bytes", len(e.Body))
		}
	})
}
