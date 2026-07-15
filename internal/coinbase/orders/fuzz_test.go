// SPDX-License-Identifier: MIT

package orders

import (
	"strings"
	"testing"
)

func FuzzBuildConfig(f *testing.F) {
	f.Add("market", "0.1", "", "", false, "")
	f.Add("market", "", "100", "", false, "")
	f.Add("limit", "0.1", "", "50000", true, "")
	f.Add("limit", "0.1", "", "50000", false, "2026-12-31T00:00:00Z")
	f.Add("LIMIT", " 0.1 ", " ", " 50000 ", false, " ")
	f.Add("", "", "", "", false, "")
	f.Add("stop", "1", "1", "1", true, "x")
	f.Fuzz(func(t *testing.T, orderType, baseSize, quoteSize, limitPrice string, postOnly bool, endTime string) {
		cfg, err := buildConfig(ConfigParams{
			OrderType: orderType, BaseSize: baseSize, QuoteSize: quoteSize,
			LimitPrice: limitPrice, PostOnly: postOnly, EndTime: endTime,
		}) // must never panic
		if err != nil {
			if cfg != nil {
				t.Fatal("error with non-nil config")
			}
			return
		}
		// Invariant: exactly one configuration variant populated.
		count := 0
		if cfg.MarketIOC != nil {
			count++
			// Invariant: market orders carry exactly one size.
			if (cfg.MarketIOC.BaseSize == "") == (cfg.MarketIOC.QuoteSize == "") {
				t.Errorf("market config sizes = %q/%q, want exactly one", cfg.MarketIOC.BaseSize, cfg.MarketIOC.QuoteSize)
			}
		}
		if cfg.LimitGTC != nil {
			count++
			if cfg.LimitGTC.BaseSize == "" || cfg.LimitGTC.LimitPrice == "" {
				t.Error("limit GTC missing base size or price")
			}
		}
		if cfg.LimitGTD != nil {
			count++
			if cfg.LimitGTD.EndTime == "" {
				t.Error("limit GTD without end time")
			}
		}
		if count != 1 {
			t.Errorf("config variants populated = %d, want exactly 1", count)
		}
	})
}

func FuzzNormalizeSide(f *testing.F) {
	f.Add("BUY")
	f.Add("sell")
	f.Add(" Buy ")
	f.Add("")
	f.Add("HOLD")
	f.Fuzz(func(t *testing.T, side string) {
		got, err := normalizeSide(side) // must never panic
		if err == nil && got != "BUY" && got != "SELL" {
			t.Errorf("normalizeSide(%q) = %q without error", side, got)
		}
		if err == nil && !strings.EqualFold(strings.TrimSpace(side), got) {
			t.Errorf("normalizeSide(%q) = %q changed the side", side, got)
		}
	})
}
