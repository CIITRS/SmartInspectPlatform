package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryExpressProviderUsesBaiduMarketplaceParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json;charset=UTF-8" {
			t.Fatalf("content type = %q", got)
		}
		if got := r.Header.Get("X-Bce-Signature"); got != "AppCode/test-app-code" {
			t.Fatalf("signature = %q", got)
		}
		if got := r.URL.Query().Get("type"); got != "auto" {
			t.Fatalf("type = %q", got)
		}
		if got := r.URL.Query().Get("number"); got != "SF123456" {
			t.Fatalf("number = %q", got)
		}
		if got := r.URL.Query().Get("mobile"); got != "13800138000" {
			t.Fatalf("mobile = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": 0,
			"msg":    "ok",
			"result": map[string]interface{}{
				"number":         "SF123456",
				"type":           "sfexpress",
				"typename":       "顺丰速运",
				"deliverystatus": 1,
				"issign":         0,
				"list": []map[string]string{{
					"time": "2026-07-30 10:00:00", "status": "运输中",
				}},
			},
		})
	}))
	defer server.Close()

	result, err := queryExpressProvider(server.Client(), expressProviderConfig{
		Enabled: true,
		URL:     server.URL,
		AppKey:  "test-app-code",
	}, "sfexpress", "SF123456", "13800138000")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "in_transit" || len(result.Route) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestQueryExpressProviderDropsRouteAfterDelivery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": 0,
			"msg":    "ok",
			"result": map[string]interface{}{
				"number":         "4303200322000",
				"type":           "yunda",
				"typename":       "韵达快运",
				"deliverystatus": 3,
				"issign":         1,
				"list": []map[string]string{
					{"time": "2026-07-30 15:20:00", "status": "快件已签收"},
					{"time": "2026-07-30 10:00:00", "status": "运输中"},
				},
			},
		})
	}))
	defer server.Close()

	result, err := queryExpressProvider(server.Client(), expressProviderConfig{
		Enabled: true,
		URL:     server.URL,
		AppKey:  "test-app-code",
	}, "auto", "4303200322000", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "delivered" {
		t.Fatalf("status = %s", result.Status)
	}
	if result.DeliveredAt == nil {
		t.Fatal("delivered time was not captured")
	}
	if len(result.Route) != 0 {
		t.Fatalf("delivered route should be cleared, got %d events", len(result.Route))
	}
}
