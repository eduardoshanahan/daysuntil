package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "daysuntil.db"
	}

	githubOAuth := githubConfigFromEnv()
	magicLinks, err := magicLinkConfigFromEnv()
	if err != nil {
		log.Fatalf("magic link config: %v", err)
	}
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
		magicLinks:   magicLinks,
		httpClient:   http.DefaultClient,
		authLimiter:  newAuthRateLimiter(db),
	}

	r := newRouter(h)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(newHTTPServer(":"+port, r).ListenAndServe())
}
