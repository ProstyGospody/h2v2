package httpserver

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"h2v2/internal/config"
	"h2v2/internal/http/handlers"
	"h2v2/internal/http/middleware"
	"h2v2/internal/repository"
)

func NewRouter(
	cfg config.Config,
	logger *slog.Logger,
	repo repository.Repository,
	h *handlers.Handler,
) *chi.Mux {
	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.RequestLogger(logger))

	r.Get("/healthz", h.Healthz)
	r.Get("/readyz", h.Readyz)
	r.Get("/hysteria/subscription/{token}", h.HysteriaUserSubscription)

	r.Route("/api", func(api chi.Router) {
		api.Get("/hysteria/subscription/{token}", h.HysteriaUserSubscription)

		api.Route("/auth", func(auth chi.Router) {
			auth.Post("/login", h.Login)
			auth.With(
				middleware.RequireAuth(cfg, repo, logger),
				middleware.RequireCSRF(cfg),
			).Post("/logout", h.Logout)
			auth.With(
				middleware.RequireAuth(cfg, repo, logger),
				middleware.RequireCSRF(cfg),
			).Get("/me", h.Me)
		})

		api.Group(func(secured chi.Router) {
			secured.Use(middleware.RequireAuth(cfg, repo, logger))
			secured.Use(middleware.RequireCSRF(cfg))

			secured.Get("/hysteria/client-defaults", h.HysteriaClientDefaults)
			secured.Get("/hysteria/users", h.ListHysteriaUsers)
			secured.Post("/hysteria/users", h.CreateHysteriaUser)
			secured.Post("/hysteria/users/state", h.SetHysteriaUsersState)
			secured.Post("/hysteria/users/delete", h.DeleteHysteriaUsers)
			secured.Get("/hysteria/users/{id}", h.GetHysteriaUser)
			secured.Patch("/hysteria/users/{id}", h.UpdateHysteriaUser)
			secured.Delete("/hysteria/users/{id}", h.DeleteHysteriaUser)
			secured.Post("/hysteria/users/{id}/revoke", h.RevokeHysteriaUser)
			secured.Post("/hysteria/users/{id}/enable", h.EnableHysteriaUser)
			secured.Post("/hysteria/users/{id}/disable", h.DisableHysteriaUser)
			secured.Get("/hysteria/users/{id}/artifacts", h.HysteriaUserArtifacts)
			secured.Get("/hysteria/users/{id}/qr", h.HysteriaUserQR)
			secured.Post("/hysteria/users/{id}/kick", h.KickHysteriaUser)
			secured.Get("/hysteria/stats/overview", h.HysteriaStatsOverview)
			secured.Get("/hysteria/stats/history", h.HysteriaStatsHistory)
			secured.Get("/hysteria/settings", h.GetHysteriaSettings)
			secured.Post("/hysteria/settings/validate", h.ValidateHysteriaSettings)
			secured.Put("/hysteria/settings", h.SaveHysteriaSettings)
			secured.Post("/hysteria/settings/apply", h.ApplyHysteriaSettings)
			secured.Get("/storage/sqlite/backup", h.DownloadSQLiteBackup)
			secured.Post("/storage/sqlite/restore", h.RestoreSQLiteBackup)

			secured.Get("/services", h.ListServices)
			secured.Get("/services/{name}", h.GetService)
			secured.Post("/services/{name}/restart", h.RestartService)
			secured.Post("/services/{name}/reload", h.ReloadService)

			secured.Get("/system/live", h.GetSystemLive)
			secured.Get("/system/history", h.GetSystemHistory)

			secured.Get("/audit", h.ListAudit)
		})
	})

	return r
}
