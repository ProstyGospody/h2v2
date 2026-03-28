package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"proxy-panel/internal/repository"
)

func TestHysteriaUserArtifactsRejectsDisabledUser(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	repo, err := repository.New(filepath.Join(tmpDir, "storage"), filepath.Join(tmpDir, "audit"), filepath.Join(tmpDir, "run"))
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	created, err := repo.CreateHysteriaUser(ctx, "demo-user", "supersecret88", nil, nil)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := repo.SetHysteriaUserEnabled(ctx, created.ID, false); err != nil {
		t.Fatalf("disable user: %v", err)
	}

	handler := &Handler{repo: repo}
	req := httptest.NewRequest(http.MethodGet, "/api/hysteria/users/"+created.ID+"/artifacts", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", created.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	resp := httptest.NewRecorder()
	handler.HysteriaUserArtifacts(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("expected disabled user artifacts request to fail with 409, got %d", resp.Code)
	}
}

