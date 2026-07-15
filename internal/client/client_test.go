// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNew_InvalidBaseURL(t *testing.T) {
	if _, err := New("://not-a-url", nil); err == nil {
		t.Fatal("expected error for invalid base URL")
	}
}

func TestNew_Defaults(t *testing.T) {
	c, err := New("https://api.example.com", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.userAgent != "mcp-client" {
		t.Errorf("default user agent = %q, want mcp-client", c.userAgent)
	}
	if c.http == nil || c.http.Timeout != defaultTimeout {
		t.Errorf("default http client timeout = %v, want %v", c.http.Timeout, defaultTimeout)
	}
}

func TestOptions(t *testing.T) {
	hc := &http.Client{}
	c, err := New("https://api.example.com", nil,
		WithHTTPClient(hc),
		WithUserAgent("custom-agent"),
		WithHeader("X-Pin", "2026-01-01"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.http != hc {
		t.Error("WithHTTPClient not applied")
	}
	if c.userAgent != "custom-agent" {
		t.Errorf("user agent = %q, want custom-agent", c.userAgent)
	}
	if got := c.header.Get("X-Pin"); got != "2026-01-01" {
		t.Errorf("X-Pin header = %q, want 2026-01-01", got)
	}
}

// newTestClient returns a client pointed at a httptest server running handler.
func newTestClient(t *testing.T, handler http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, nil, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestDo_GetDecodesJSON(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"name":"BTC-USD"}`)
	})

	var out struct {
		Name string `json:"name"`
	}
	resp, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/v1/thing", Out: &out})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if out.Name != "BTC-USD" {
		t.Errorf("decoded name = %q, want BTC-USD", out.Name)
	}
}

func TestDo_QueryParams(t *testing.T) {
	var gotQuery url.Values
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = io.WriteString(w, `{}`)
	})

	q := url.Values{}
	q.Set("limit", "5")
	q.Set("product_type", "SPOT")
	if _, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/x", Query: q}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotQuery.Get("limit") != "5" || gotQuery.Get("product_type") != "SPOT" {
		t.Errorf("query = %v, want limit=5 product_type=SPOT", gotQuery)
	}
}

func TestDo_PostEncodesJSONBody(t *testing.T) {
	var gotBody []byte
	var gotCT string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"7"}`)
	})

	body := map[string]string{"side": "BUY"}
	var out struct {
		ID string `json:"id"`
	}
	resp, err := c.Do(context.Background(), Request{Method: http.MethodPost, Path: "/orders", Body: body, Out: &out})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	var sent map[string]string
	if err := json.Unmarshal(gotBody, &sent); err != nil || sent["side"] != "BUY" {
		t.Errorf("sent body = %s, want {\"side\":\"BUY\"}", gotBody)
	}
	if out.ID != "7" {
		t.Errorf("decoded id = %q, want 7", out.ID)
	}
}

func TestDo_ReaderBodySentVerbatim(t *testing.T) {
	var gotBody []byte
	var gotCT string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotCT = r.Header.Get("Content-Type")
		_, _ = io.WriteString(w, `{}`)
	})

	if _, err := c.Do(context.Background(), Request{
		Method: http.MethodPost, Path: "/raw", Body: strings.NewReader("raw-bytes"),
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if string(gotBody) != "raw-bytes" {
		t.Errorf("body = %q, want raw-bytes", gotBody)
	}
	if gotCT != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", gotCT)
	}
}

func TestDo_ContentTypeOverride(t *testing.T) {
	var gotCT string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		_, _ = io.WriteString(w, `{}`)
	})

	if _, err := c.Do(context.Background(), Request{
		Method: http.MethodPost, Path: "/x", Body: strings.NewReader("a=b"), ContentType: "application/x-www-form-urlencoded",
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotCT != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want form-urlencoded", gotCT)
	}

	// JSON body with explicit content type.
	if _, err := c.Do(context.Background(), Request{
		Method: http.MethodPost, Path: "/x", Body: map[string]int{"a": 1}, ContentType: "application/vnd.api+json",
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotCT != "application/vnd.api+json" {
		t.Errorf("Content-Type = %q, want vnd.api+json", gotCT)
	}
}

func TestDo_UnencodableBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := c.Do(context.Background(), Request{Method: http.MethodPost, Path: "/x", Body: func() {}})
	if err == nil || !strings.Contains(err.Error(), "encoding request body") {
		t.Fatalf("err = %v, want encoding request body error", err)
	}
}

func TestDo_APIErrorOnNon2xx(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","error_details":"product not found","message":"product not found"}`)
	})

	resp, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/missing"})
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Errorf("resp = %+v, want status 404 alongside error", resp)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T (%v), want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.Message != "product not found" {
		t.Errorf("Message = %q, want 'product not found'", apiErr.Message)
	}
	if apiErr.Method != http.MethodGet {
		t.Errorf("Method = %q, want GET", apiErr.Method)
	}
}

func TestDo_DecodeError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `not-json`)
	})
	var out map[string]any
	_, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/x", Out: &out})
	if err == nil || !strings.Contains(err.Error(), "decoding") {
		t.Fatalf("err = %v, want decoding error", err)
	}
}

func TestDo_EmptyBodyWithOutIsNotAnError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	var out map[string]any
	resp, err := c.Do(context.Background(), Request{Method: http.MethodDelete, Path: "/x", Out: &out})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
}

func TestDo_NilOutIgnoresBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ignored":true}`)
	})
	if _, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/x"}); err != nil {
		t.Fatalf("Do: %v", err)
	}
}

func TestDo_RawBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "plain text")
	})
	var raw RawBody
	if _, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/x", Out: &raw}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if raw.String() != "plain text" {
		t.Errorf("raw = %q, want 'plain text'", raw.String())
	}
	if raw.ContentType != "text/plain" {
		t.Errorf("content type = %q, want text/plain", raw.ContentType)
	}
}

func TestDo_TransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	c, err := New(srv.URL, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv.Close() // force connection refused
	if _, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/x"}); err == nil {
		t.Fatal("expected transport error after server close")
	}
}

func TestDo_BodyReadError(t *testing.T) {
	// Advertise a longer body than is sent, then drop the connection so
	// io.ReadAll fails mid-body.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("response writer does not support hijacking")
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			t.Fatalf("hijack: %v", err)
		}
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\nshort")
		_ = buf.Flush()
		_ = conn.Close()
	})
	_, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/x"})
	if err == nil || !strings.Contains(err.Error(), "reading response body") {
		t.Fatalf("err = %v, want body read error", err)
	}
}

func TestDo_ContextCancelled(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Do(ctx, Request{Method: http.MethodGet, Path: "/x"}); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestDo_InvalidMethod(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	if _, err := c.Do(context.Background(), Request{Method: "BAD METHOD", Path: "/x"}); err == nil {
		t.Fatal("expected error for invalid method")
	}
}

func TestBuildRequest_EscapedPathNotDoubleEncoded(t *testing.T) {
	var gotPath, gotRawPath string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawPath = r.URL.RawPath
		_, _ = io.WriteString(w, `{}`)
	})

	// A path-escaped ID containing a slash must arrive encoded exactly once.
	path := "/api/v3/brokerage/market/products/" + url.PathEscape("BTC/USD")
	if _, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: path}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotPath != "/api/v3/brokerage/market/products/BTC/USD" {
		t.Errorf("decoded path = %q", gotPath)
	}
	if gotRawPath != "/api/v3/brokerage/market/products/BTC%2FUSD" {
		t.Errorf("raw path = %q, want single-encoded %%2F", gotRawPath)
	}
}

func TestBuildRequest_QueryEmbeddedInPath(t *testing.T) {
	var gotQuery url.Values
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = io.WriteString(w, `{}`)
	})

	q := url.Values{}
	q.Set("limit", "5")
	if _, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/x?embedded=1", Query: q}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotQuery.Get("embedded") != "1" || gotQuery.Get("limit") != "5" {
		t.Errorf("query = %v, want both embedded=1 and limit=5", gotQuery)
	}
}

func TestBuildRequest_InvalidPath(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	if _, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/bad%zz"}); err == nil {
		t.Fatal("expected error for invalid percent-escape in path")
	}
}

func TestBuildRequest_HeadersAndAuth(t *testing.T) {
	var got http.Header
	handler := func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = io.WriteString(w, `{}`)
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	c, err := New(srv.URL, NewBearerAuthorizer("tok-123"),
		WithUserAgent("ua-test"),
		WithHeader("X-Extra", "base-value"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	reqHeader := http.Header{}
	reqHeader.Set("X-Extra", "per-request") // must win over client-level header
	if _, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/x", Header: reqHeader}); err != nil {
		t.Fatalf("Do: %v", err)
	}

	if got.Get("Authorization") != "Bearer tok-123" {
		t.Errorf("Authorization = %q, want Bearer tok-123", got.Get("Authorization"))
	}
	if got.Get("User-Agent") != "ua-test" {
		t.Errorf("User-Agent = %q, want ua-test", got.Get("User-Agent"))
	}
	if vals := got.Values("X-Extra"); len(vals) != 1 || vals[0] != "per-request" {
		t.Errorf("X-Extra = %v, want [per-request]", vals)
	}
}

func TestBuildRequest_MultiValueClientHeader(t *testing.T) {
	var got http.Header
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = io.WriteString(w, `{}`)
	})
	// Simulate a multi-valued client-level header.
	c.header = http.Header{"X-Multi": {"one", "two"}}

	if _, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/x"}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if vals := got.Values("X-Multi"); len(vals) != 2 || vals[0] != "one" || vals[1] != "two" {
		t.Errorf("X-Multi = %v, want [one two]", vals)
	}
}

func TestBuildRequest_BasePathPreserved(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)

	c, err := New(srv.URL+"/base/prefix/", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "sub/resource"}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotPath != "/base/prefix/sub/resource" {
		t.Errorf("path = %q, want /base/prefix/sub/resource", gotPath)
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		base, rel, want string
	}{
		{"", "", "/"},
		{"", "/x", "/x"},
		{"", "x", "/x"},
		{"/base", "", "/base"},
		{"/base/", "", "/base"},
		{"/base", "/x", "/base/x"},
		{"/base/", "/x/", "/base/x/"},
		{"/base", "x/y", "/base/x/y"},
	}
	for _, tt := range tests {
		if got := joinPath(tt.base, tt.rel); got != tt.want {
			t.Errorf("joinPath(%q, %q) = %q, want %q", tt.base, tt.rel, got, tt.want)
		}
	}
}

func TestEncodeBody_Nil(t *testing.T) {
	r, ct, err := encodeBody(Request{})
	if err != nil || r != nil || ct != "" {
		t.Errorf("encodeBody(nil) = %v, %q, %v; want nil, \"\", nil", r, ct, err)
	}
}

func TestHelpers(t *testing.T) {
	var lastMethod string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	ctx := context.Background()
	var out map[string]any

	tests := []struct {
		name string
		call func() error
		want string
	}{
		{"GetJSON", func() error { return c.GetJSON(ctx, "/x", nil, &out) }, http.MethodGet},
		{"PostJSON", func() error { return c.PostJSON(ctx, "/x", nil, map[string]int{"a": 1}, &out) }, http.MethodPost},
		{"PutJSON", func() error { return c.PutJSON(ctx, "/x", nil, map[string]int{"a": 1}, &out) }, http.MethodPut},
		{"PatchJSON", func() error { return c.PatchJSON(ctx, "/x", nil, map[string]int{"a": 1}, &out) }, http.MethodPatch},
		{"Delete", func() error { return c.Delete(ctx, "/x", nil, &out) }, http.MethodDelete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if lastMethod != tt.want {
				t.Errorf("method = %s, want %s", lastMethod, tt.want)
			}
			if ok, _ := out["ok"].(bool); !ok {
				t.Errorf("out = %v, want ok=true", out)
			}
		})
	}
}
