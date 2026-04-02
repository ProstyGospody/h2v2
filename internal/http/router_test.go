package httpserver

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"h2v2/internal/config"
	"h2v2/internal/http/handlers"
)

func TestRouterExposesHysteriaRoutesAndDropsLegacyRoutes(t *testing.T) {
	cfg := config.Config{
		SessionCookieName: "pp_session",
		CSRFCookieName:    "pp_csrf",
		CSRFHeaderName:    "X-CSRF-Token",
	}
	router := NewRouter(cfg, slog.Default(), nil, &handlers.Handler{})

	for _, path := range []string{"/api/hysteria/users", "/api/hysteria/settings", "/api/storage/sqlite/backup", "/api/services", "/api/users", "/api/inbounds"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("expected %s to require auth and return 401, got %d", path, resp.Code)
		}
	}

	kickReq := httptest.NewRequest(http.MethodPost, "/api/users/kick", nil)
	kickResp := httptest.NewRecorder()
	router.ServeHTTP(kickResp, kickReq)
	if kickResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected /api/users/kick to require auth and return 401, got %d", kickResp.Code)
	}

	restoreReq := httptest.NewRequest(http.MethodPost, "/api/storage/sqlite/restore", nil)
	restoreResp := httptest.NewRecorder()
	router.ServeHTTP(restoreResp, restoreReq)
	if restoreResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected /api/storage/sqlite/restore to require auth and return 401, got %d", restoreResp.Code)
	}

	subReq := httptest.NewRequest(http.MethodGet, "/api/hysteria/subscription/demo-token", nil)
	subResp := httptest.NewRecorder()
	router.ServeHTTP(subResp, subReq)
	if subResp.Code == http.StatusNotFound || subResp.Code == http.StatusUnauthorized {
		t.Fatalf("expected subscription route to be exposed without auth middleware, got %d", subResp.Code)
	}
	legacySubReq := httptest.NewRequest(http.MethodGet, "/hysteria/subscription/demo-token", nil)
	legacySubResp := httptest.NewRecorder()
	router.ServeHTTP(legacySubResp, legacySubReq)
	if legacySubResp.Code == http.StatusNotFound || legacySubResp.Code == http.StatusUnauthorized {
		t.Fatalf("expected legacy subscription route alias to be exposed without auth middleware, got %d", legacySubResp.Code)
	}
	unifiedSubReq := httptest.NewRequest(http.MethodGet, "/api/subscriptions/demo-token", nil)
	unifiedSubResp := httptest.NewRecorder()
	router.ServeHTTP(unifiedSubResp, unifiedSubReq)
	if unifiedSubResp.Code == http.StatusNotFound || unifiedSubResp.Code == http.StatusUnauthorized {
		t.Fatalf("expected unified subscription route to be exposed without auth middleware, got %d", unifiedSubResp.Code)
	}

	for _, path := range []string{"/api/clients", "/api/hy2/accounts", "/api/legacy/access", "/api/legacy/settings"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("expected removed path %s to return 404, got %d", path, resp.Code)
		}
	}
}
