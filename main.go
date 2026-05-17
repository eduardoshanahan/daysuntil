package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "modernc.org/sqlite"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "daysuntil.db"
	}

	githubOAuth := githubConfigFromEnv()
	cookieSecure, err := cookieSecureFromEnv(githubOAuth)
	if err != nil {
		log.Fatalf("cookie security config: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := initDB(db); err != nil {
		log.Fatalf("init db: %v", err)
	}

	h := &handler{
		db:           db,
		cookieSecure: cookieSecure,
		githubOAuth:  githubOAuth,
		httpClient:   http.DefaultClient,
		authLimiter:  newAuthRateLimiter(),
	}

	r := newRouter(h)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func newRouter(h *handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/api/version", h.appVersion)
	r.With(authRateLimitMiddleware(h.authLimiter, authActionRegister)).Post("/api/register", h.register)
	r.With(authRateLimitMiddleware(h.authLimiter, authActionLogin)).Post("/api/login", h.login)
	r.Post("/api/logout", h.logout)
	r.Get("/api/me", h.currentUser)
	r.Put("/api/me/profile", h.updateProfile)
	r.Get("/api/auth/providers", h.authProviders)
	r.Get("/api/public/users/{username}", h.publicProfile)
	r.Get("/api/oauth/github/start", h.githubOAuthStart)
	r.Get("/api/oauth/github/callback", h.githubOAuthCallback)
	r.Get("/u/{username}", servePublicProfileApp)

	r.Route("/api/intervals", func(r chi.Router) {
		r.Use(authMiddleware(h))
		r.Get("/", h.listIntervals)
		r.Post("/", h.createInterval)
		r.Put("/{id}", h.updateInterval)
		r.Delete("/{id}", h.deleteInterval)
	})

	r.Handle("/*", http.FileServer(http.Dir("static")))
	return r
}
