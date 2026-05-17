package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "modernc.org/sqlite"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
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
	log.Fatal(newHTTPServer(":"+port, r).ListenAndServe())
}

func newRouter(h *handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeadersMiddleware)

	r.Route("/api", func(r chi.Router) {
		r.Use(noStoreMiddleware)

		r.Get("/version", h.appVersion)
		r.With(authRateLimitMiddleware(h.authLimiter, authActionRegister)).Post("/register", h.register)
		r.With(authRateLimitMiddleware(h.authLimiter, authActionLogin)).Post("/login", h.login)
		r.Post("/logout", h.logout)
		r.Get("/me", h.currentUser)
		r.Delete("/me", h.deleteAccount)
		r.Post("/me/make-private", h.makeAllIntervalsPrivate)
		r.Post("/me/public-link/rotate", h.rotatePublicLink)
		r.Put("/me/profile", h.updateProfile)
		r.Get("/auth/providers", h.authProviders)
		r.Get("/public/profiles/{publicSlug}", h.publicProfile)
	})
	r.Get("/api/oauth/github/start", h.githubOAuthStart)
	r.Get("/api/oauth/github/callback", h.githubOAuthCallback)
	r.Get("/p/{publicSlug}", servePublicProfileApp)

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

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func noStoreMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		next.ServeHTTP(w, r)
	})
}
