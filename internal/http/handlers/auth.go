package handlers

import (
	"net/http"
	"strings"
	"time"

	"proxy-panel/internal/http/middleware"
	"proxy-panel/internal/http/render"
	"proxy-panel/internal/security"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	password := strings.TrimSpace(req.Password)
	if email == "" || password == "" {
		render.Error(w, http.StatusBadRequest, "email and password are required")
		return
	}

	ip := h.requestIP(r)
	if !h.rateLimiter.Allow(ip) {
		render.Error(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}

	admin, err := h.repo.GetAdminByEmail(r.Context(), email)
	if err != nil || !admin.IsActive {
		render.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := security.ComparePassword(admin.PasswordHash, password); err != nil {
		render.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	h.rateLimiter.Reset(ip)
	sessionToken, err := security.NewToken(32)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	csrfToken, err := security.NewToken(24)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "failed to create csrf token")
		return
	}

	expiresAt := time.Now().UTC().Add(h.cfg.SessionTTL)
	if _, err := h.repo.CreateSession(
		r.Context(),
		admin.ID,
		security.HashToken(sessionToken),
		expiresAt,
		ip,
		r.UserAgent(),
	); err != nil {
		render.Error(w, http.StatusInternalServerError, "failed to persist session")
		return
	}

	h.setAuthCookies(w, sessionToken, csrfToken, expiresAt)
	h.audit(r, "auth.login", "admin", &admin.ID, map[string]any{"email": admin.Email, "ip": ip})
	render.JSON(w, http.StatusOK, map[string]any{
		"admin": map[string]any{
			"id":    admin.ID,
			"email": admin.Email,
		},
		"csrf_token": csrfToken,
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.cfg.SessionCookieName)
	if err == nil && strings.TrimSpace(cookie.Value) != "" {
		_ = h.repo.DeleteSessionByHash(r.Context(), security.HashToken(cookie.Value))
	}
	h.clearAuthCookies(w)
	admin, ok := middleware.AdminFromContext(r.Context())
	if ok {
		h.audit(r, "auth.logout", "admin", &admin.ID, nil)
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	admin, ok := middleware.AdminFromContext(r.Context())
	if !ok {
		render.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{
		"id":         admin.ID,
		"email":      admin.Email,
		"is_active":  admin.IsActive,
		"created_at": admin.CreatedAt,
		"updated_at": admin.UpdatedAt,
	})
}

