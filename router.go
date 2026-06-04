package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func newRouter(h *handler) http.Handler {
	r := chi.NewRouter()
	r.Use(pathOnlyLogger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeadersMiddleware(h.cookieSecure))

	r.Route("/api", func(r chi.Router) {
		r.Use(noStoreMiddleware)

		r.Get("/version", h.appVersion)
		r.With(authRateLimitMiddleware(h.authLimiter, authActionRegister)).Post("/register", h.register)
		r.With(authRateLimitMiddleware(h.authLimiter, authActionLogin)).Post("/login", h.login)
		r.With(authRateLimitMiddleware(h.authLimiter, authActionMagicLink)).Post("/login/link", h.requestMagicLink)
		r.Post("/login/link/consume", h.consumeMagicLink)
		r.Post("/verify-email", h.verifyEmail)
		r.With(authRateLimitMiddleware(h.authLimiter, authActionMagicLink)).Post("/resend-verification", h.resendVerification)
		r.Post("/logout", h.logout)
		r.Get("/me", h.currentUser)
		r.Delete("/me", h.deleteAccount)
		r.Put("/me/profile", h.updateProfile)
		r.Get("/auth/providers", h.authProviders)
		r.With(authRateLimitMiddleware(h.authLimiter, authActionPublicLookup)).Get("/public/groups/{groupSlug}", h.publicShareGroup)
	})
	r.Get("/api/oauth/github/start", h.githubOAuthStart)
	r.Get("/api/oauth/github/callback", h.githubOAuthCallback)
	r.Get("/g/{groupSlug}", servePublicGroupApp)

	r.Route("/api/intervals", func(r chi.Router) {
		r.Use(authMiddleware(h))
		r.Get("/", h.listIntervals)
		r.Post("/", h.createInterval)
		r.Post("/{id}/move", h.moveInterval)
		r.Put("/{id}", h.updateInterval)
		r.Delete("/{id}", h.deleteInterval)
	})

	r.Route("/api/share-groups", func(r chi.Router) {
		r.Use(authMiddleware(h))
		r.Get("/", h.listShareGroups)
		r.Post("/", h.createShareGroup)
		r.Put("/{id}", h.updateShareGroup)
		r.Delete("/{id}", h.deleteShareGroup)
		r.Post("/{id}/rotate", h.rotateShareGroup)
	})

	r.Handle("/*", http.FileServer(http.Dir("static")))
	return r
}
