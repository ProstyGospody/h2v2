package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"h2v2/internal/config"
	"h2v2/internal/http/render"
	"h2v2/internal/repository"
	"h2v2/internal/security"
)

func RequireAuth(cfg config.Config, repo repository.Repository, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cfg.SessionCookieName)
			if err != nil || cookie.Value == "" {
				render.Error(w, http.StatusUnauthorized, "authentication required")
				return
			}

			tokenHash := security.HashToken(cookie.Value)
			authCtx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
			defer cancel()

			_, admin, err := repo.GetSessionWithAdminByTokenHash(authCtx, tokenHash)
			if err != nil {
				if repository.IsNotFound(err) {
					clearAuthCookies(w, cfg)
					render.Error(w, http.StatusUnauthorized, "invalid session")
					return
				}
				render.Error(w, http.StatusServiceUnavailable, "service unavailable")
				return
			}
			if !admin.IsActive {
				clearAuthCookies(w, cfg)
				render.Error(w, http.StatusUnauthorized, "invalid session")
				return
			}

			ctx := WithAdmin(r.Context(), admin)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

func clearAuthCookies(w http.ResponseWriter, cfg config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Secure:   cfg.SecureCookies,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.CSRFCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Secure:   cfg.SecureCookies,
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
	})
}

