package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestHysteriaFetchTrafficUsesRawAuthorizationSecret(t *testing.T) {
	const expectedSecret = "secret-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/traffic" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != expectedSecret {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user-1": map[string]any{"tx": 100, "rx": 50},
		})
	}))
	defer server.Close()

	client := NewHysteriaClient(server.URL, expectedSecret)
	traffic, err := client.FetchTraffic(context.Background())
	if err != nil {
		t.Fatalf("FetchTraffic returned error: %v", err)
	}

	expected := map[string]Hy2Traffic{"user-1": {TxBytes: 100, RxBytes: 50}}
	if !reflect.DeepEqual(traffic, expected) {
		t.Fatalf("unexpected traffic payload: %+v", traffic)
	}
}

func TestHysteriaKickUsesArrayPayload(t *testing.T) {
	const expectedSecret = "secret-token"
	const identity = "hy2-user-1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/kick" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != expectedSecret {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("unexpected content-type: %q", ct)
		}
		var body []string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		expected := []string{identity}
		if !reflect.DeepEqual(body, expected) {
			t.Fatalf("unexpected kick payload: %+v", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHysteriaClient(server.URL, expectedSecret)
	if err := client.Kick(context.Background(), identity); err != nil {
		t.Fatalf("Kick returned error: %v", err)
	}
}

func TestHysteriaFetchOnlineStatsWithProtocolBreakdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/online" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"users": map[string]any{
				"alice": map[string]any{
					"online":          4,
					"connections_tcp": 1,
					"connections_udp": 3,
				},
				"bob": []any{
					map[string]any{"protocol": "tcp"},
					map[string]any{"network": "udp"},
					map[string]any{"transport": "quic"},
				},
			},
		})
	}))
	defer server.Close()

	client := NewHysteriaClient(server.URL, "")
	online, summary, err := client.FetchOnlineStats(context.Background())
	if err != nil {
		t.Fatalf("FetchOnlineStats returned error: %v", err)
	}

	expectedOnline := map[string]int{
		"alice": 4,
		"bob":   3,
	}
	if !reflect.DeepEqual(online, expectedOnline) {
		t.Fatalf("unexpected online map: %#v", online)
	}

	if summary.TotalConnections != 7 {
		t.Fatalf("unexpected total connections: %d", summary.TotalConnections)
	}
	if summary.TCPConnections != 2 {
		t.Fatalf("unexpected tcp connections: %d", summary.TCPConnections)
	}
	if summary.UDPConnections != 5 {
		t.Fatalf("unexpected udp connections: %d", summary.UDPConnections)
	}
	if !summary.BreakdownAvailable {
		t.Fatalf("expected breakdown availability to be true")
	}
}

func TestHysteriaFetchOnlineStatsWithoutProtocolBreakdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/online" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"users": map[string]any{
				"alice": 2,
				"bob": map[string]any{
					"count": 1,
				},
			},
		})
	}))
	defer server.Close()

	client := NewHysteriaClient(server.URL, "")
	online, summary, err := client.FetchOnlineStats(context.Background())
	if err != nil {
		t.Fatalf("FetchOnlineStats returned error: %v", err)
	}

	expectedOnline := map[string]int{
		"alice": 2,
		"bob":   1,
	}
	if !reflect.DeepEqual(online, expectedOnline) {
		t.Fatalf("unexpected online map: %#v", online)
	}
	if summary.TotalConnections != 3 {
		t.Fatalf("unexpected total connections: %d", summary.TotalConnections)
	}
	if summary.TCPConnections != 0 || summary.UDPConnections != 0 {
		t.Fatalf("expected zero tcp/udp breakdown, got tcp=%d udp=%d", summary.TCPConnections, summary.UDPConnections)
	}
	if summary.BreakdownAvailable {
		t.Fatalf("expected breakdown availability to be false")
	}
}
