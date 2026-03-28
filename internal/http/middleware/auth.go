package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"proxy-panel/internal/config"
	"proxy-panel/internal/http/render"
	"proxy-panel/internal/repository"
	"proxy-panel/internal/security"
)

func RequireAuth(cfg config.Config, repo *repository.Repository, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cfg.SessionCookieName)
			if err != nil || cookie.Value == "" {
				render.Error(w, http.StatusUnauthorized, "authentication required")
				return
			}

			tokenHash := security.HashToken(cookie.Value)
			session, admin, err := repo.GetSessionWithAdminByTokenHash(r.Context(), tokenHash)
			if err != nil || !admin.IsActive {
				clearAuthCookies(w, cfg)
				render.Error(w, http.StatusUnauthorized, "invalid session")
				return
			}

			ctx := WithAdmin(r.Context(), admin)
			r = r.WithContext(ctx)

			go touchSessionAsync(context.Background(), repo, logger, session.ID)

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

func touchSessionAsync(ctx context.Context, repo *repository.Repository, logger *slog.Logger, sessionID string) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := repo.TouchSession(ctx, sessionID); err != nil {
		logger.Debug("failed to touch session", "session_id", sessionID, "error", err)
	}
}

