package main

import (
	"context"
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

	oidcCfg := oidcConfigFromEnv()
	cookieSecure, err := cookieSecureFromEnv()
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
		oidc:         oidcCfg,
		httpClient:   http.DefaultClient,
		authLimiter:  newAuthRateLimiter(db),
	}

	if oidcCfg.Enabled() {
		rt, err := newOIDCRuntime(context.Background(), oidcCfg)
		if err != nil {
			log.Fatalf("oidc init: %v", err)
		}
		h.oidcRT = rt
		log.Printf("oidc: configured with issuer %s", oidcCfg.Issuer)
	} else {
		log.Printf("oidc: not configured (set OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET)")
	}

	r := newRouter(h)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(newHTTPServer(":"+port, r).ListenAndServe())
}
