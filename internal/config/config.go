// SPDX-License-Identifier: MIT

// Package config loads and validates runtime configuration for the coinbase-mcp
// server from environment variables.
//
// All configuration is supplied via the environment so the server can run as a
// stdio subprocess launched by an MCP client (Claude Desktop/Code, Cursor, …),
// where command-line flags are awkward to pass.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Environment variable names recognised by the server.
const (
	EnvAPIKey    = "COINBASE_API_KEY"    // CDP API key name (optional; enables authenticated tools)
	EnvAPISecret = "COINBASE_API_SECRET" // CDP API private key (optional)
	EnvBaseURL   = "COINBASE_BASE_URL"   // override the API base URL
	EnvToolsets  = "COINBASE_TOOLSETS"   // comma-separated toolset names, or "all"
	EnvReadOnly  = "COINBASE_READONLY"   // "true" disables all write tools
)

// DefaultBaseURL is the Coinbase API host used when COINBASE_BASE_URL is unset.
const DefaultBaseURL = "https://api.coinbase.com"

// Config holds validated server configuration.
type Config struct {
	// APIKey / APISecret authenticate to Coinbase. Optional: public market-data
	// tools work without them.
	APIKey    string
	APISecret string
	// BaseURL is the Coinbase REST base URL (never has a trailing slash).
	BaseURL string
	// Toolsets is the set of enabled toolset names. A nil/empty set means "all".
	Toolsets []string
	// ReadOnly, when true, suppresses mutating tools at registration time.
	ReadOnly bool
}

// AllToolsets reports whether every toolset should be enabled.
func (c *Config) AllToolsets() bool {
	if len(c.Toolsets) == 0 {
		return true
	}
	for _, t := range c.Toolsets {
		if t == "all" {
			return true
		}
	}
	return false
}

// ToolsetEnabled reports whether the named toolset should be registered.
func (c *Config) ToolsetEnabled(name string) bool {
	if c.AllToolsets() {
		return true
	}
	for _, t := range c.Toolsets {
		if strings.EqualFold(t, name) {
			return true
		}
	}
	return false
}

// Load reads configuration from the process environment and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		APIKey:    strings.TrimSpace(os.Getenv(EnvAPIKey)),
		APISecret: strings.TrimSpace(os.Getenv(EnvAPISecret)),
		BaseURL:   strings.TrimRight(strings.TrimSpace(os.Getenv(EnvBaseURL)), "/"),
		Toolsets:  splitList(os.Getenv(EnvToolsets)),
		ReadOnly:  isTruthy(os.Getenv(EnvReadOnly)),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}

	var errs []error
	if u, err := url.Parse(cfg.BaseURL); err != nil || u.Scheme == "" || u.Host == "" {
		errs = append(errs, fmt.Errorf("%s is not a valid URL: %q", EnvBaseURL, cfg.BaseURL))
	}
	// Credentials are optional, but a half-set pair is almost certainly a mistake.
	if (cfg.APIKey == "") != (cfg.APISecret == "") {
		errs = append(errs, fmt.Errorf("set both %s and %s, or neither", EnvAPIKey, EnvAPISecret))
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return cfg, nil
}

// splitList parses a comma-separated environment value into a trimmed,
// lower-cased slice, dropping empty entries.
func splitList(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// isTruthy reports whether an environment value represents boolean true.
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
