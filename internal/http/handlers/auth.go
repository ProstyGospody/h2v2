package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"h2v2/internal/http/render"
	"h2v2/internal/repository"
	"h2v2/internal/security"
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

	authCtx, cancelAuth := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancelAuth()

	admin, err := h.repo.GetAdminByEmail(authCtx, email)
	if err != nil {
		if repository.IsNotFound(err) {
			render.Error(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		render.Error(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	if !admin.IsActive {
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
	sessionCtx, cancelSession := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancelSession()

	if _, err := h.repo.CreateSession(
		sessionCtx,
		admin.ID,
		security.HashToken(sessionToken),
		expiresAt,
		ip,
		r.UserAgent(),
	); err != nil {
		render.Error(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}

	h.setAuthCookies(w, sessionToken, csrfToken, expiresAt)
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

