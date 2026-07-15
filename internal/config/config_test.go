// SPDX-License-Identifier: MIT

package config

import (
	"reflect"
	"strings"
	"testing"
)

// clearEnv unsets every config variable for the test's duration.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{EnvAPIKey, EnvAPISecret, EnvBaseURL, EnvToolsets, EnvReadOnly} {
		t.Setenv(k, "")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, DefaultBaseURL)
	}
	if cfg.APIKey != "" || cfg.APISecret != "" {
		t.Errorf("credentials should default empty, got %q/%q", cfg.APIKey, cfg.APISecret)
	}
	if len(cfg.Toolsets) != 0 {
		t.Errorf("Toolsets = %v, want empty", cfg.Toolsets)
	}
	if cfg.ReadOnly {
		t.Error("ReadOnly should default false")
	}
}

func TestLoad_FullConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvAPIKey, " key ")
	t.Setenv(EnvAPISecret, " secret ")
	t.Setenv(EnvBaseURL, "https://api-sandbox.coinbase.com/")
	t.Setenv(EnvToolsets, "Products, orders")
	t.Setenv(EnvReadOnly, "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey != "key" || cfg.APISecret != "secret" {
		t.Errorf("credentials not trimmed: %q/%q", cfg.APIKey, cfg.APISecret)
	}
	if cfg.BaseURL != "https://api-sandbox.coinbase.com" {
		t.Errorf("BaseURL = %q, want trailing slash stripped", cfg.BaseURL)
	}
	if !reflect.DeepEqual(cfg.Toolsets, []string{"products", "orders"}) {
		t.Errorf("Toolsets = %v, want [products orders]", cfg.Toolsets)
	}
	if !cfg.ReadOnly {
		t.Error("ReadOnly = false, want true")
	}
}

func TestLoad_InvalidBaseURL(t *testing.T) {
	clearEnv(t)
	for _, bad := range []string{"not a url", "api.coinbase.com", "https://"} {
		t.Setenv(EnvBaseURL, bad)
		if _, err := Load(); err == nil {
			t.Errorf("Load with base URL %q: expected error", bad)
		}
	}
}

func TestLoad_HalfSetCredentials(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvAPIKey, "key-only")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), EnvAPISecret) {
		t.Errorf("expected half-set credential error, got %v", err)
	}

	clearEnv(t)
	t.Setenv(EnvAPISecret, "secret-only")
	if _, err := Load(); err == nil {
		t.Error("expected half-set credential error for secret-only")
	}
}

func TestLoad_JoinsMultipleErrors(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvBaseURL, "bogus")
	t.Setenv(EnvAPIKey, "key-only")
	_, err := Load()
	if err == nil {
		t.Fatal("expected joined errors")
	}
	msg := err.Error()
	if !strings.Contains(msg, EnvBaseURL) || !strings.Contains(msg, EnvAPIKey) {
		t.Errorf("joined error missing parts: %v", msg)
	}
}

func TestAllToolsets(t *testing.T) {
	tests := []struct {
		toolsets []string
		want     bool
	}{
		{nil, true},
		{[]string{}, true},
		{[]string{"all"}, true},
		{[]string{"products", "all"}, true},
		{[]string{"products"}, false},
	}
	for _, tt := range tests {
		c := &Config{Toolsets: tt.toolsets}
		if got := c.AllToolsets(); got != tt.want {
			t.Errorf("AllToolsets(%v) = %v, want %v", tt.toolsets, got, tt.want)
		}
	}
}

func TestToolsetEnabled(t *testing.T) {
	c := &Config{Toolsets: []string{"products"}}
	if !c.ToolsetEnabled("products") {
		t.Error("products should be enabled")
	}
	if !c.ToolsetEnabled("PRODUCTS") {
		t.Error("matching should be case-insensitive")
	}
	if c.ToolsetEnabled("orders") {
		t.Error("orders should not be enabled")
	}

	all := &Config{}
	if !all.ToolsetEnabled("anything") {
		t.Error("empty toolsets should enable everything")
	}
}

func TestSplitList(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"a", []string{"a"}},
		{"A,B", []string{"a", "b"}},
		{" a , , b ,", []string{"a", "b"}},
	}
	for _, tt := range tests {
		if got := splitList(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitList(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestIsTruthy(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", " yes ", "on", "On"} {
		if !isTruthy(v) {
			t.Errorf("isTruthy(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"", "0", "false", "no", "off", "banana"} {
		if isTruthy(v) {
			t.Errorf("isTruthy(%q) = true, want false", v)
		}
	}
}
