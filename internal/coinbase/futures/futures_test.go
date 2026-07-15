// SPDX-License-Identifier: MIT

package futures

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rangertaha/coinbase-mcp/internal/client"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Fixtures shaped per the Advanced Trade CFM endpoints, including fields the
// trimmed structs intentionally drop (decoding must ignore them).
const (
	balanceFixture = `{
  "balance_summary": {
    "futures_buying_power": {"value": "5000.00", "currency": "USD"},
    "total_usd_balance": {"value": "10000.00", "currency": "USD"},
    "cbi_usd_balance": {"value": "4000.00", "currency": "USD"},
    "cfm_usd_balance": {"value": "6000.00", "currency": "USD"},
    "total_open_orders_hold_amount": {"value": "250.00", "currency": "USD"},
    "unrealized_pnl": {"value": "-12.34", "currency": "USD"},
    "daily_realized_pnl": {"value": "45.67", "currency": "USD"},
    "initial_margin": {"value": "1500.00", "currency": "USD"},
    "available_margin": {"value": "3500.00", "currency": "USD"},
    "liquidation_threshold": {"value": "900.00", "currency": "USD"},
    "liquidation_buffer_amount": {"value": "2600.00", "currency": "USD"},
    "liquidation_buffer_percentage": "288.9",
    "intraday_margin_window_measure": {"margin_window_type": "MARGIN_WINDOW_TYPE_INTRADAY"}
  }
}`
	positionsFixture = `{
  "positions": [
    {
      "product_id": "BIT-31OCT25-CDE",
      "expiration_time": "2025-10-31T16:00:00Z",
      "side": "LONG",
      "number_of_contracts": "3",
      "current_price": "64500.00",
      "avg_entry_price": "63000.00",
      "unrealized_pnl": "450.00",
      "daily_realized_pnl": "0.00"
    },
    {
      "product_id": "ET-31OCT25-CDE",
      "side": "SHORT",
      "number_of_contracts": "1"
    }
  ]
}`
	positionFixture = `{
  "position": {
    "product_id": "BIT-31OCT25-CDE",
    "expiration_time": "2025-10-31T16:00:00Z",
    "side": "LONG",
    "number_of_contracts": "3",
    "current_price": "64500.00",
    "avg_entry_price": "63000.00",
    "unrealized_pnl": "450.00",
    "daily_realized_pnl": "0.00"
  }
}`
	sweepsFixture = `{
  "sweeps": [
    {
      "id": "sweep-1",
      "requested_amount": {"value": "100.00", "currency": "USD"},
      "should_sweep_all": false,
      "status": "PENDING",
      "scheduled_time": "2026-07-14T22:00:00Z"
    },
    {
      "id": "sweep-2",
      "requested_amount": {"value": "0", "currency": "USD"},
      "should_sweep_all": true,
      "status": "PROCESSING"
    }
  ]
}`
	marginSettingFixture = `{"setting": "INTRADAY_MARGIN_SETTING_INTRADAY"}`
	marginWindowFixture  = `{
  "margin_window": {"margin_window_type": "MARGIN_WINDOW_TYPE_INTRADAY", "end_time": "2026-07-14T20:00:00Z"},
  "is_intraday_margin_killswitch_enabled": true,
  "is_intraday_margin_enrollment_killswitch_enabled": false
}`
	errorFixture = `{"error": "INVALID_ARGUMENT", "message": "bad request"}`
)

// newTestService returns a futures service backed by a httptest server.
func newTestService(t *testing.T, handler http.HandlerFunc) *service {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := coinbase.NewClients(srv.URL, "", "")
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	return &service{c: c}
}

// errorService returns a service whose API always answers with the given
// status and the standard error envelope.
func errorService(t *testing.T, status int) *service {
	t.Helper()
	return newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, errorFixture)
	})
}

// wantAPIError asserts err is a *client.APIError with the given status.
func wantAPIError(t *testing.T, err error, status int) {
	t.Helper()
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != status {
		t.Fatalf("err = %v, want *APIError with %d", err, status)
	}
	if apiErr.Message != "bad request" {
		t.Errorf("Message = %q, want API message", apiErr.Message)
	}
}

func TestGetBalanceSummary(t *testing.T) {
	var gotPath, gotMethod string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_, _ = io.WriteString(w, balanceFixture)
	})

	out, err := svc.GetBalanceSummary(context.Background())
	if err != nil {
		t.Fatalf("GetBalanceSummary: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/v3/brokerage/cfm/balance_summary" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	usd := func(v string) Amount { return Amount{Value: v, Currency: "USD"} }
	if out.FuturesBuyingPower != usd("5000.00") ||
		out.TotalUSDBalance != usd("10000.00") ||
		out.CBIUSDBalance != usd("4000.00") ||
		out.CFMUSDBalance != usd("6000.00") ||
		out.TotalOpenOrdersHoldAmount != usd("250.00") ||
		out.UnrealizedPnL != usd("-12.34") ||
		out.DailyRealizedPnL != usd("45.67") ||
		out.InitialMargin != usd("1500.00") ||
		out.AvailableMargin != usd("3500.00") ||
		out.LiquidationThreshold != usd("900.00") ||
		out.LiquidationBufferAmount != usd("2600.00") ||
		out.LiquidationBufferPct != "288.9" {
		t.Errorf("balance summary decoded wrong: %+v", out)
	}
}

func TestGetBalanceSummary_APIError(t *testing.T) {
	svc := errorService(t, http.StatusUnauthorized)
	_, err := svc.GetBalanceSummary(context.Background())
	wantAPIError(t, err, http.StatusUnauthorized)
}

func TestListPositions(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, positionsFixture)
	})

	out, err := svc.ListPositions(context.Background())
	if err != nil {
		t.Fatalf("ListPositions: %v", err)
	}
	if gotPath != "/api/v3/brokerage/cfm/positions" {
		t.Errorf("path = %q", gotPath)
	}
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	p := out[0]
	if p.ProductID != "BIT-31OCT25-CDE" || p.ExpirationTime != "2025-10-31T16:00:00Z" ||
		p.Side != "LONG" || p.NumberOfContracts != "3" || p.CurrentPrice != "64500.00" ||
		p.AvgEntryPrice != "63000.00" || p.UnrealizedPnL != "450.00" || p.DailyRealizedPnL != "0.00" {
		t.Errorf("position decoded wrong: %+v", p)
	}
	if out[1].ProductID != "ET-31OCT25-CDE" || out[1].Side != "SHORT" {
		t.Errorf("second position decoded wrong: %+v", out[1])
	}
}

func TestListPositions_APIError(t *testing.T) {
	svc := errorService(t, http.StatusForbidden)
	_, err := svc.ListPositions(context.Background())
	wantAPIError(t, err, http.StatusForbidden)
}

func TestGetPosition(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, positionFixture)
	})

	p, err := svc.GetPosition(context.Background(), "  BIT-31OCT25-CDE  ")
	if err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	if gotPath != "/api/v3/brokerage/cfm/positions/BIT-31OCT25-CDE" {
		t.Errorf("path = %q, want trimmed ID", gotPath)
	}
	if p.ProductID != "BIT-31OCT25-CDE" || p.Side != "LONG" || p.NumberOfContracts != "3" ||
		p.CurrentPrice != "64500.00" || p.AvgEntryPrice != "63000.00" {
		t.Errorf("position decoded wrong: %+v", p)
	}
}

func TestGetPosition_IDIsPathEscaped(t *testing.T) {
	var gotRawPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawPath = r.URL.RawPath
		_, _ = io.WriteString(w, positionFixture)
	})
	if _, err := svc.GetPosition(context.Background(), "BIT/CDE"); err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	if gotRawPath != "/api/v3/brokerage/cfm/positions/BIT%2FCDE" {
		t.Errorf("raw path = %q, want single-encoded", gotRawPath)
	}
}

func TestGetPosition_EmptyID(t *testing.T) {
	called := false
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	for _, id := range []string{"", "   "} {
		if _, err := svc.GetPosition(context.Background(), id); err == nil {
			t.Errorf("GetPosition(%q): expected error", id)
		}
	}
	if called {
		t.Error("empty ID must not reach the API")
	}
}

func TestGetPosition_APIError(t *testing.T) {
	svc := errorService(t, http.StatusNotFound)
	_, err := svc.GetPosition(context.Background(), "NOPE-CDE")
	wantAPIError(t, err, http.StatusNotFound)
}

func TestListSweeps(t *testing.T) {
	var gotPath, gotMethod string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_, _ = io.WriteString(w, sweepsFixture)
	})

	out, err := svc.ListSweeps(context.Background())
	if err != nil {
		t.Fatalf("ListSweeps: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/v3/brokerage/cfm/sweeps" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	s := out[0]
	if s.ID != "sweep-1" || s.RequestedAmount != (Amount{Value: "100.00", Currency: "USD"}) ||
		s.ShouldSweepAll || s.Status != "PENDING" || s.ScheduledTime != "2026-07-14T22:00:00Z" {
		t.Errorf("sweep decoded wrong: %+v", s)
	}
	if !out[1].ShouldSweepAll || out[1].Status != "PROCESSING" {
		t.Errorf("second sweep decoded wrong: %+v", out[1])
	}
}

func TestListSweeps_APIError(t *testing.T) {
	svc := errorService(t, http.StatusUnauthorized)
	_, err := svc.ListSweeps(context.Background())
	wantAPIError(t, err, http.StatusUnauthorized)
}

func TestScheduleSweep(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, `{"success": true}`)
	})

	out, err := svc.ScheduleSweep(context.Background(), " 100.00 ")
	if err != nil {
		t.Fatalf("ScheduleSweep: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v3/brokerage/cfm/sweeps/schedule" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if len(gotBody) != 1 || gotBody["usd_amount"] != "100.00" {
		t.Errorf("body = %v, want {usd_amount: 100.00}", gotBody)
	}
	if !out.Scheduled || out.USDAmount != "100.00" {
		t.Errorf("result = %+v, want scheduled confirmation", out)
	}
}

func TestScheduleSweep_EmptyAmount(t *testing.T) {
	called := false
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	for _, amt := range []string{"", "  "} {
		if _, err := svc.ScheduleSweep(context.Background(), amt); err == nil {
			t.Errorf("ScheduleSweep(%q): expected error", amt)
		}
	}
	if called {
		t.Error("empty amount must not reach the API")
	}
}

func TestScheduleSweep_APIError(t *testing.T) {
	svc := errorService(t, http.StatusBadRequest)
	_, err := svc.ScheduleSweep(context.Background(), "100.00")
	wantAPIError(t, err, http.StatusBadRequest)
}

func TestCancelSweep(t *testing.T) {
	var gotPath, gotMethod string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_, _ = io.WriteString(w, `{"success": true}`)
	})

	out, err := svc.CancelSweep(context.Background())
	if err != nil {
		t.Fatalf("CancelSweep: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/v3/brokerage/cfm/sweeps" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if !out.Cancelled {
		t.Errorf("result = %+v, want cancelled confirmation", out)
	}
}

func TestCancelSweep_APIError(t *testing.T) {
	svc := errorService(t, http.StatusBadRequest)
	_, err := svc.CancelSweep(context.Background())
	wantAPIError(t, err, http.StatusBadRequest)
}

func TestGetMarginSetting(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, marginSettingFixture)
	})

	out, err := svc.GetMarginSetting(context.Background())
	if err != nil {
		t.Fatalf("GetMarginSetting: %v", err)
	}
	if gotPath != "/api/v3/brokerage/cfm/intraday/margin_setting" {
		t.Errorf("path = %q", gotPath)
	}
	if out.Setting != "INTRADAY_MARGIN_SETTING_INTRADAY" {
		t.Errorf("setting = %q", out.Setting)
	}
}

func TestGetMarginSetting_APIError(t *testing.T) {
	svc := errorService(t, http.StatusUnauthorized)
	_, err := svc.GetMarginSetting(context.Background())
	wantAPIError(t, err, http.StatusUnauthorized)
}

func TestSetMarginSetting(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, `{}`)
	})

	out, err := svc.SetMarginSetting(context.Background(), " INTRADAY_MARGIN_SETTING_STANDARD ")
	if err != nil {
		t.Fatalf("SetMarginSetting: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v3/brokerage/cfm/intraday/margin_setting" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if len(gotBody) != 1 || gotBody["setting"] != "INTRADAY_MARGIN_SETTING_STANDARD" {
		t.Errorf("body = %v", gotBody)
	}
	if !out.Updated || out.Setting != "INTRADAY_MARGIN_SETTING_STANDARD" {
		t.Errorf("result = %+v, want updated confirmation", out)
	}
}

func TestSetMarginSetting_EmptySetting(t *testing.T) {
	called := false
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	for _, setting := range []string{"", "  "} {
		if _, err := svc.SetMarginSetting(context.Background(), setting); err == nil {
			t.Errorf("SetMarginSetting(%q): expected error", setting)
		}
	}
	if called {
		t.Error("empty setting must not reach the API")
	}
}

func TestSetMarginSetting_APIError(t *testing.T) {
	svc := errorService(t, http.StatusBadRequest)
	_, err := svc.SetMarginSetting(context.Background(), "INTRADAY_MARGIN_SETTING_INTRADAY")
	wantAPIError(t, err, http.StatusBadRequest)
}

func TestGetMarginWindow(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, marginWindowFixture)
	})

	out, err := svc.GetMarginWindow(context.Background(), "")
	if err != nil {
		t.Fatalf("GetMarginWindow: %v", err)
	}
	if gotPath != "/api/v3/brokerage/cfm/intraday/current_margin_window" {
		t.Errorf("path = %q", gotPath)
	}
	if out.MarginWindow.MarginWindowType != "MARGIN_WINDOW_TYPE_INTRADAY" ||
		out.MarginWindow.EndTime != "2026-07-14T20:00:00Z" {
		t.Errorf("margin window decoded wrong: %+v", out.MarginWindow)
	}
	if !out.IsKillswitchEnabled || out.IsEnrollKillswitchEnabled {
		t.Errorf("killswitch flags decoded wrong: %+v", out)
	}
}

func TestGetMarginWindow_APIError(t *testing.T) {
	svc := errorService(t, http.StatusInternalServerError)
	_, err := svc.GetMarginWindow(context.Background(), "")
	wantAPIError(t, err, http.StatusInternalServerError)
}

func TestScheduleSweep_SuccessFalse(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"success": false}`)
	})
	if _, err := svc.ScheduleSweep(context.Background(), "100.00"); err == nil {
		t.Fatal("expected error when the API reports success=false")
	}
}

func TestCancelSweep_SuccessFalse(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"success": false}`)
	})
	if _, err := svc.CancelSweep(context.Background()); err == nil {
		t.Fatal("expected error when the API reports success=false")
	}
}

func TestGetMarginWindow_ProfileTypeParam(t *testing.T) {
	var gotQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, marginWindowFixture)
	})
	if _, err := svc.GetMarginWindow(context.Background(), "MARGIN_PROFILE_TYPE_RETAIL_INTRADAY_MARGIN_1"); err != nil {
		t.Fatalf("GetMarginWindow: %v", err)
	}
	if gotQuery != "margin_profile_type=MARGIN_PROFILE_TYPE_RETAIL_INTRADAY_MARGIN_1" {
		t.Errorf("query = %q", gotQuery)
	}
}
