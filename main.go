package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "modernc.org/sqlite"
	"database/sql"
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

	h := &handler{db: db}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api/intervals", func(r chi.Router) {
		r.Get("/", h.listIntervals)
		r.Post("/", h.createInterval)
		r.Put("/{id}", h.updateInterval)
		r.Delete("/{id}", h.deleteInterval)
	})

	r.Handle("/*", http.FileServer(http.Dir("static")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
