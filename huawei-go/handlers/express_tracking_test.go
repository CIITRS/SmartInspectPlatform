package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestSignExpressV1Request(t *testing.T) {
	request, err := http.NewRequest(http.MethodPost, "http://gwgp-65bmfhhrext.n.bdcloudapi.com/express/query?number=SF1&type=auto", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	if err := signExpressV1Request(request, "test-key", "test-secret", time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	wantPrefix := "bce-auth-v1/test-key/2026-08-01T08:00:00Z/1800/content-type;host/"
	if !strings.HasPrefix(request.Header.Get("X-Bce-Signature"), wantPrefix) {
		t.Fatalf("signature = %q, want prefix %q", request.Header.Get("X-Bce-Signature"), wantPrefix)
	}
}

func TestQueryExpressProviderIncludesProviderErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"msg":"number parameter is invalid"}`))
	}))
	defer server.Close()

	_, err := queryExpressProvider(server.Client(), expressProviderConfig{Enabled: true, URL: server.URL, AppKey: "test-app-code"}, "auto", "bad", "")
	if err == nil || !strings.Contains(err.Error(), "number parameter is invalid") {
		t.Fatalf("error = %v, want provider response", err)
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
