package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func newRouter(h *handler, corsOrigins []string) http.Handler {
	r := chi.NewRouter()
	r.Use(pathOnlyLogger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeadersMiddleware(h.cookieSecure))
	if len(corsOrigins) > 0 {
		r.Use(corsMiddleware(corsOrigins))
	}

	r.Route("/api", func(r chi.Router) {
		r.Use(noStoreMiddleware)

		r.Get("/version", h.appVersion)
		r.Post("/logout", h.logout)
		r.Get("/me", h.currentUser)
		r.Delete("/me", h.deleteAccount)
		r.Put("/me/profile", h.updateProfile)
		r.Put("/me/username", h.setUsername)
		r.With(authRateLimitMiddleware(h.authLimiter, authActionPublicLookup)).Get("/public/groups/{groupSlug}", h.publicShareGroup)
	})

	r.Get("/api/oidc/start", h.oidcStart)
	r.Get("/api/oidc/callback", h.oidcCallback)

	r.Route("/api/intervals", func(r chi.Router) {
		r.Use(authMiddleware(h))
		r.Get("/", h.listIntervals)
		r.Post("/", h.createInterval)
		r.Post("/{id}/move", h.moveInterval)
		r.Put("/{id}", h.updateInterval)
		r.Delete("/{id}", h.deleteInterval)
	})

	r.Route("/api/intervals/{intervalID}/reminders", func(r chi.Router) {
		r.Use(authMiddleware(h))
		r.Get("/", h.listReminders)
		r.Post("/", h.createReminder)
	})

	r.Route("/api/reminders/{id}", func(r chi.Router) {
		r.Use(authMiddleware(h))
		r.Put("/", h.updateReminder)
		r.Delete("/", h.deleteReminder)
	})

	r.Route("/api/tokens", func(r chi.Router) {
		r.Use(authMiddleware(h))
		r.Get("/", h.listTokens)
		r.Post("/", h.createToken)
		r.Delete("/{id}", h.deleteToken)
	})

	r.Route("/api/share-groups", func(r chi.Router) {
		r.Use(authMiddleware(h))
		r.Get("/", h.listShareGroups)
		r.Post("/", h.createShareGroup)
		r.Put("/{id}", h.updateShareGroup)
		r.Delete("/{id}", h.deleteShareGroup)
		r.Post("/{id}/rotate", h.rotateShareGroup)
	})

	return r
}
