package middleware

import (
	"net/http"
	"strings"

	"proxy-panel/internal/config"
	"proxy-panel/internal/http/render"
)

func RequireCSRF(cfg config.Config) func(http.Handler) http.Handler {
	safeMethods := map[string]struct{}{
		http.MethodGet:     {},
		http.MethodHead:    {},
		http.MethodOptions: {},
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := safeMethods[r.Method]; ok {
				next.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/api/auth/login") {
				next.ServeHTTP(w, r)
				return
			}

			csrfCookie, err := r.Cookie(cfg.CSRFCookieName)
			if err != nil || csrfCookie.Value == "" {
				render.Error(w, http.StatusForbidden, "missing csrf cookie")
				return
			}
			header := strings.TrimSpace(r.Header.Get(cfg.CSRFHeaderName))
			if header == "" || header != csrfCookie.Value {
				render.Error(w, http.StatusForbidden, "invalid csrf token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

