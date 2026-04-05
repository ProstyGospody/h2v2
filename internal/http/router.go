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
	apiLimiter := middleware.NewAPIRateLimiter()

	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.RequestLogger(logger))

	r.Get("/healthz", h.Healthz)
	r.Get("/readyz", h.Readyz)
	r.Get("/sub/{token}", h.CoreSubscriptionAuto)
	r.Get("/sub/{token}/profile.singbox.json", h.CoreSubscriptionProfile)
	r.Get("/sub/{token}/uris.txt", h.CoreSubscriptionURIs)
	r.Get("/sub/{token}/qr.png", h.CoreSubscriptionQR)
	r.Get("/sub/{token}/profile.clash.yaml", h.CoreSubscriptionClash)
	r.Get("/sub/{token}/profile.base64.txt", h.CoreSubscriptionBase64)

	r.Route("/api", func(api chi.Router) {
		api.Use(middleware.RateLimit(apiLimiter))
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

			secured.Route("/v1", func(v1 chi.Router) {
				v1.Get("/defaults", h.CoreDefaults)
				v1.Get("/servers", h.ListCoreServers)
				v1.Post("/servers", h.CreateCoreServer)
				v1.Get("/servers/{id}", h.GetCoreServer)
				v1.Patch("/servers/{id}", h.UpdateCoreServer)
				v1.Delete("/servers/{id}", h.DeleteCoreServer)
				v1.Get("/servers/{id}/config/preview", h.GetCoreServerConfigPreview)
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
				v1.Post("/users/provision", h.ProvisionCoreUser)
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

				// Domain validation
				v1.Get("/servers/{id}/validate/domain", h.ValidateCoreServerDomain)

				// Outbounds
				v1.Get("/outbounds", h.ListCoreOutbounds)
				v1.Post("/outbounds", h.UpsertCoreOutbound)
				v1.Get("/outbounds/{id}", h.GetCoreOutbound)
				v1.Patch("/outbounds/{id}", h.UpsertCoreOutbound)
				v1.Delete("/outbounds/{id}", h.DeleteCoreOutbound)

				// Route rules
				v1.Get("/route-rules", h.ListCoreRouteRules)
				v1.Post("/route-rules", h.UpsertCoreRouteRule)
				v1.Get("/route-rules/{id}", h.GetCoreRouteRule)
				v1.Patch("/route-rules/{id}", h.UpsertCoreRouteRule)
				v1.Delete("/route-rules/{id}", h.DeleteCoreRouteRule)

				// DNS profiles
				v1.Get("/dns-profiles", h.ListCoreDNSProfiles)
				v1.Post("/dns-profiles", h.UpsertCoreDNSProfile)
				v1.Get("/dns-profiles/{id}", h.GetCoreDNSProfile)
				v1.Patch("/dns-profiles/{id}", h.UpsertCoreDNSProfile)
				v1.Delete("/dns-profiles/{id}", h.DeleteCoreDNSProfile)

				// Log profiles
				v1.Get("/log-profiles", h.ListCoreLogProfiles)
				v1.Post("/log-profiles", h.UpsertCoreLogProfile)
				v1.Get("/log-profiles/{id}", h.GetCoreLogProfile)
				v1.Patch("/log-profiles/{id}", h.UpsertCoreLogProfile)
				v1.Delete("/log-profiles/{id}", h.DeleteCoreLogProfile)

				// Reality profiles
				v1.Get("/reality-profiles", h.ListCoreRealityProfiles)
				v1.Post("/reality-profiles", h.UpsertCoreRealityProfile)
				v1.Get("/reality-profiles/{id}", h.GetCoreRealityProfile)
				v1.Patch("/reality-profiles/{id}", h.UpsertCoreRealityProfile)
				v1.Delete("/reality-profiles/{id}", h.DeleteCoreRealityProfile)

				// Transport profiles
				v1.Get("/transport-profiles", h.ListCoreTransportProfiles)
				v1.Post("/transport-profiles", h.UpsertCoreTransportProfile)
				v1.Get("/transport-profiles/{id}", h.GetCoreTransportProfile)
				v1.Patch("/transport-profiles/{id}", h.UpsertCoreTransportProfile)
				v1.Delete("/transport-profiles/{id}", h.DeleteCoreTransportProfile)

				// Multiplex profiles
				v1.Get("/multiplex-profiles", h.ListCoreMultiplexProfiles)
				v1.Post("/multiplex-profiles", h.UpsertCoreMultiplexProfile)
				v1.Get("/multiplex-profiles/{id}", h.GetCoreMultiplexProfile)
				v1.Patch("/multiplex-profiles/{id}", h.UpsertCoreMultiplexProfile)
				v1.Delete("/multiplex-profiles/{id}", h.DeleteCoreMultiplexProfile)

				// HY2 masquerade profiles
				v1.Get("/hy2-masquerade-profiles", h.ListCoreHY2MasqueradeProfiles)
				v1.Post("/hy2-masquerade-profiles", h.UpsertCoreHY2MasqueradeProfile)
				v1.Get("/hy2-masquerade-profiles/{id}", h.GetCoreHY2MasqueradeProfile)
				v1.Patch("/hy2-masquerade-profiles/{id}", h.UpsertCoreHY2MasqueradeProfile)
				v1.Delete("/hy2-masquerade-profiles/{id}", h.DeleteCoreHY2MasqueradeProfile)

				// Client profiles (user-facing connection modes)
				v1.Get("/client-profiles", h.ListCoreClientProfiles)
				v1.Post("/client-profiles", h.UpsertCoreClientProfile)
				v1.Get("/client-profiles/{id}", h.GetCoreClientProfile)
				v1.Patch("/client-profiles/{id}", h.UpsertCoreClientProfile)
				v1.Delete("/client-profiles/{id}", h.DeleteCoreClientProfile)
			})
		})
	})

	return r
}
