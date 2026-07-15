// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIKeyHeaderAuthorizer(t *testing.T) {
	a := NewAPIKeyHeaderAuthorizer("X-Api-Key", "secret-key")
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err := a.Authorize(req); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if got := req.Header.Get("X-Api-Key"); got != "secret-key" {
		t.Errorf("X-Api-Key = %q, want secret-key", got)
	}
}

func TestBearerAuthorizer(t *testing.T) {
	a := NewBearerAuthorizer("tok")
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err := a.Authorize(req); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer tok" {
		t.Errorf("Authorization = %q, want 'Bearer tok'", got)
	}
}

type failingAuthorizer struct{}

func (failingAuthorizer) Authorize(*http.Request) error {
	return errors.New("signing failed")
}

func TestDo_AuthorizerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, failingAuthorizer{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/x"})
	if err == nil || !strings.Contains(err.Error(), "authorizing request") {
		t.Fatalf("err = %v, want authorizing request error", err)
	}
}
