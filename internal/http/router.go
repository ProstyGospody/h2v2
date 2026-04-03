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
	r.Get("/subscriptions/{token}", h.UserSubscription)
	r.Get("/sub/{token}/profile.singbox.json", h.CoreSubscriptionProfile)
	r.Get("/sub/{token}/uris.txt", h.CoreSubscriptionURIs)
	r.Get("/sub/{token}/qr.png", h.CoreSubscriptionQR)

	r.Route("/api", func(api chi.Router) {
		api.Get("/subscriptions/{token}", h.UserSubscription)

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

			secured.Get("/storage/sqlite/backup", h.DownloadSQLiteBackup)
			secured.Post("/storage/sqlite/restore", h.RestoreSQLiteBackup)

			secured.Get("/services", h.ListServices)
			secured.Get("/services/{name}", h.GetService)
			secured.Post("/services/{name}/restart", h.RestartService)
			secured.Post("/services/{name}/reload", h.ReloadService)

			secured.Get("/system/live", h.GetSystemLive)
			secured.Get("/system/history", h.GetSystemHistory)

			secured.Get("/audit", h.ListAudit)

			secured.Get("/users", h.ListUsers)
			secured.Post("/users", h.CreateUser)
			secured.Post("/users/state", h.SetUsersState)
			secured.Post("/users/delete", h.DeleteUsers)
			secured.Get("/users/{id}", h.GetUser)
			secured.Patch("/users/{id}", h.UpdateUser)
			secured.Delete("/users/{id}", h.DeleteUser)
			secured.Get("/users/{id}/qr", h.UserQR)
			secured.Post("/users/{id}/kick", h.KickUser)
			secured.Post("/users/kick", h.KickUsers)
			secured.Get("/users/{id}/subscription-token", h.GetUserSubscriptionToken)
			secured.Post("/users/{id}/subscription-token/rotate", h.RotateUserSubscriptionToken)
			secured.Post("/users/{id}/subscription-token/revoke", h.RevokeUserSubscriptionToken)
			secured.Post("/users/{id}/subscription-token/restore", h.RestoreUserSubscriptionToken)

			secured.Get("/inbounds", h.ListInbounds)
			secured.Post("/inbounds", h.UpsertInbound)
			secured.Get("/inbounds/{id}", h.GetInbound)
			secured.Patch("/inbounds/{id}", h.UpsertInbound)
			secured.Delete("/inbounds/{id}", h.DeleteInbound)

			secured.Route("/v1", func(v1 chi.Router) {
				v1.Get("/servers", h.ListCoreServers)
				v1.Post("/servers", h.CreateCoreServer)
				v1.Get("/servers/{id}", h.GetCoreServer)
				v1.Patch("/servers/{id}", h.UpdateCoreServer)
				v1.Delete("/servers/{id}", h.DeleteCoreServer)
				v1.Post("/servers/{id}/config/render", h.RenderCoreServerConfig)
				v1.Post("/servers/{id}/config/validate", h.ValidateCoreServerConfig)
				v1.Post("/servers/{id}/config/apply", h.ApplyCoreServerConfig)
				v1.Get("/servers/{id}/config/revisions", h.ListCoreServerRevisions)
				v1.Post("/servers/{id}/config/rollback/{revisionID}", h.RollbackCoreServerConfig)

				v1.Get("/inbounds", h.ListCoreInbounds)
				v1.Post("/inbounds", h.CreateCoreInbound)
				v1.Get("/inbounds/{id}", h.GetCoreInbound)
				v1.Patch("/inbounds/{id}", h.UpdateCoreInbound)
				v1.Delete("/inbounds/{id}", h.DeleteCoreInbound)

				v1.Get("/users", h.ListCoreUsers)
				v1.Post("/users", h.CreateCoreUser)
				v1.Get("/users/{id}", h.GetCoreUser)
				v1.Patch("/users/{id}", h.UpdateCoreUser)
				v1.Delete("/users/{id}", h.DeleteCoreUser)

				v1.Get("/users/{id}/access", h.ListCoreUserAccess)
				v1.Post("/access", h.UpsertCoreAccess)
				v1.Delete("/access/{id}", h.DeleteCoreAccess)

				v1.Get("/users/{id}/artifacts", h.CoreUserArtifacts)
				v1.Get("/users/{id}/artifacts/profile.json", h.CoreUserProfileJSON)
				v1.Get("/users/{id}/artifacts/profile.raw", h.CoreUserProfileRaw)
				v1.Get("/users/{id}/artifacts/uris.txt", h.CoreUserURIsRaw)
				v1.Get("/users/{id}/artifacts/qr.png", h.CoreUserQR)

				v1.Get("/users/{id}/subscription/tokens", h.ListCoreUserTokens)
				v1.Post("/users/{id}/subscription/tokens", h.IssueCoreUserToken)
				v1.Post("/users/{id}/subscription/tokens/rotate", h.RotateCoreUserToken)
				v1.Post("/users/{id}/subscription/tokens/revoke", h.RevokeCoreUserTokens)
			})
		})
	})

	return r
}
