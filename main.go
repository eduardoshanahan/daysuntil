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
		cookieSecure: os.Getenv("COOKIE_SECURE") == "true",
		githubOAuth:  githubConfigFromEnv(),
		httpClient:   http.DefaultClient,
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

	r.Post("/api/register", h.register)
	r.Post("/api/login", h.login)
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
