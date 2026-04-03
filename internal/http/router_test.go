package httpserver

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"h2v2/internal/config"
	"h2v2/internal/http/handlers"
)

func TestRouterExposesCoreRoutesAndDropsLegacyRoutes(t *testing.T) {
	cfg := config.Config{
		SessionCookieName: "pp_session",
		CSRFCookieName:    "pp_csrf",
		CSRFHeaderName:    "X-CSRF-Token",
	}
	router := NewRouter(cfg, slog.Default(), nil, &handlers.Handler{})

	for _, path := range []string{"/api/storage/sqlite/backup", "/api/services", "/api/v1/users", "/api/v1/inbounds"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("expected %s to require auth and return 401, got %d", path, resp.Code)
		}
	}

	restoreReq := httptest.NewRequest(http.MethodPost, "/api/storage/sqlite/restore", nil)
	restoreResp := httptest.NewRecorder()
	router.ServeHTTP(restoreResp, restoreReq)
	if restoreResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected /api/storage/sqlite/restore to require auth and return 401, got %d", restoreResp.Code)
	}

	for _, path := range []string{
		"/sub/demo-token/profile.singbox.json",
		"/sub/demo-token/uris.txt",
		"/sub/demo-token/qr.png",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code == http.StatusNotFound || resp.Code == http.StatusUnauthorized {
			t.Fatalf("expected %s to be exposed without auth middleware, got %d", path, resp.Code)
		}
	}

	for _, path := range []string{
		"/api/users",
		"/api/inbounds",
		"/api/subscriptions/demo-token",
		"/subscriptions/demo-token",
		"/api/clients",
		"/api/hy2/accounts",
		"/api/legacy/access",
		"/api/legacy/settings",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("expected removed path %s to return 404, got %d", path, resp.Code)
		}
	}
}
