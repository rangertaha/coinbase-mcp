// SPDX-License-Identifier: MIT

package orders

import (
	"strings"
	"testing"
)

// TestBuildConfig_AllVariants checks every order_configuration variant against
// the shapes in the official SDK's OrderConfiguration model.
func TestBuildConfig_AllVariants(t *testing.T) {
	tests := []struct {
		name    string
		p       ConfigParams
		check   func(t *testing.T, c *orderConfiguration)
		wantErr string
	}{
		{
			name: "limit_fok",
			p:    ConfigParams{OrderType: "limit_fok", BaseSize: "0.5", LimitPrice: "60000"},
			check: func(t *testing.T, c *orderConfiguration) {
				if c.LimitFOK == nil || c.LimitFOK.BaseSize != "0.5" || c.LimitFOK.LimitPrice != "60000" {
					t.Errorf("LimitFOK = %+v", c.LimitFOK)
				}
			},
		},
		{
			name: "sor_limit",
			p:    ConfigParams{OrderType: "sor_limit", BaseSize: "0.5", LimitPrice: "60000"},
			check: func(t *testing.T, c *orderConfiguration) {
				if c.SorLimitIOC == nil || c.SorLimitIOC.BaseSize != "0.5" || c.SorLimitIOC.LimitPrice != "60000" {
					t.Errorf("SorLimitIOC = %+v", c.SorLimitIOC)
				}
			},
		},
		{
			name: "stop_limit GTC",
			p: ConfigParams{OrderType: "stop_limit", BaseSize: "1", LimitPrice: "59000",
				StopPrice: "60000", StopDirection: "stop_direction_stop_down"},
			check: func(t *testing.T, c *orderConfiguration) {
				sl := c.StopLimitGTC
				if sl == nil || sl.StopPrice != "60000" || sl.StopDirection != "STOP_DIRECTION_STOP_DOWN" {
					t.Errorf("StopLimitGTC = %+v", sl)
				}
			},
		},
		{
			name: "stop_limit GTD",
			p: ConfigParams{OrderType: "stop_limit", BaseSize: "1", LimitPrice: "59000",
				StopPrice: "60000", StopDirection: "STOP_DIRECTION_STOP_UP", EndTime: "2026-12-31T00:00:00Z"},
			check: func(t *testing.T, c *orderConfiguration) {
				sl := c.StopLimitGTD
				if sl == nil || sl.EndTime != "2026-12-31T00:00:00Z" || sl.StopDirection != "STOP_DIRECTION_STOP_UP" {
					t.Errorf("StopLimitGTD = %+v", sl)
				}
			},
		},
		{
			name: "bracket GTC",
			p: ConfigParams{OrderType: "bracket", BaseSize: "1", LimitPrice: "61000",
				StopTriggerPrice: "58000"},
			check: func(t *testing.T, c *orderConfiguration) {
				b := c.TriggerGTC
				if b == nil || b.StopTriggerPrice != "58000" || b.LimitPrice != "61000" {
					t.Errorf("TriggerGTC = %+v", b)
				}
			},
		},
		{
			name: "bracket GTD",
			p: ConfigParams{OrderType: "bracket", BaseSize: "1", LimitPrice: "61000",
				StopTriggerPrice: "58000", EndTime: "2026-12-31T00:00:00Z"},
			check: func(t *testing.T, c *orderConfiguration) {
				b := c.TriggerGTD
				if b == nil || b.EndTime != "2026-12-31T00:00:00Z" {
					t.Errorf("TriggerGTD = %+v", b)
				}
			},
		},
		{
			name:    "stop_limit missing stopPrice",
			p:       ConfigParams{OrderType: "stop_limit", BaseSize: "1", LimitPrice: "59000", StopDirection: "STOP_DIRECTION_STOP_UP"},
			wantErr: "stopPrice is required",
		},
		{
			name:    "stop_limit bad direction",
			p:       ConfigParams{OrderType: "stop_limit", BaseSize: "1", LimitPrice: "59000", StopPrice: "60000", StopDirection: "UP"},
			wantErr: "stopDirection must be",
		},
		{
			name:    "bracket missing trigger",
			p:       ConfigParams{OrderType: "bracket", BaseSize: "1", LimitPrice: "61000"},
			wantErr: "stopTriggerPrice is required",
		},
		{
			name:    "stop_limit missing price",
			p:       ConfigParams{OrderType: "stop_limit", BaseSize: "1", StopPrice: "60000", StopDirection: "STOP_DIRECTION_STOP_UP"},
			wantErr: "limitPrice is required for stop_limit",
		},
		{
			name:    "bracket missing base",
			p:       ConfigParams{OrderType: "bracket", LimitPrice: "61000", StopTriggerPrice: "58000"},
			wantErr: "baseSize is required for bracket",
		},
		{
			name:    "limit_fok missing price",
			p:       ConfigParams{OrderType: "limit_fok", BaseSize: "1"},
			wantErr: "limitPrice is required for limit_fok",
		},
		{
			name:    "sor_limit missing base",
			p:       ConfigParams{OrderType: "sor_limit", LimitPrice: "60000"},
			wantErr: "baseSize is required for sor_limit",
		},
		{
			name:    "unknown type lists options",
			p:       ConfigParams{OrderType: "twap"},
			wantErr: "orderType must be one of",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := buildConfig(tt.p)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildConfig: %v", err)
			}
			// Exactly one variant must be set.
			count := 0
			for _, v := range []bool{
				c.MarketIOC != nil, c.SorLimitIOC != nil, c.LimitGTC != nil,
				c.LimitGTD != nil, c.LimitFOK != nil, c.StopLimitGTC != nil,
				c.StopLimitGTD != nil, c.TriggerGTC != nil, c.TriggerGTD != nil,
			} {
				if v {
					count++
				}
			}
			if count != 1 {
				t.Errorf("variants set = %d, want 1", count)
			}
			tt.check(t, c)
		})
	}
}

func TestBuildAttached(t *testing.T) {
	// Neither set: no attached configuration.
	got, err := buildAttached("", "  ")
	if err != nil || got != nil {
		t.Errorf("buildAttached(none) = %v, %v; want nil, nil", got, err)
	}
	// Only one set: error.
	if _, err := buildAttached("70000", ""); err == nil {
		t.Error("take-profit alone must error")
	}
	if _, err := buildAttached("", "58000"); err == nil {
		t.Error("stop-loss alone must error")
	}
	// Both set: trigger_bracket_gtc with both prices.
	got, err = buildAttached(" 70000 ", " 58000 ")
	if err != nil {
		t.Fatalf("buildAttached: %v", err)
	}
	if got.TriggerGTC == nil || got.TriggerGTC.LimitPrice != "70000" || got.TriggerGTC.StopTriggerPrice != "58000" {
		t.Errorf("attached = %+v", got.TriggerGTC)
	}
}
