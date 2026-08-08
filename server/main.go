package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/oauth2"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	profileServiceURL := strings.TrimSpace(os.Getenv("PROFILE_SERVICE_URL"))
	profileServiceToken := strings.TrimSpace(os.Getenv("PROFILE_SERVICE_TOKEN"))
	if profileServiceURL == "" || profileServiceToken == "" {
		log.Fatal("PROFILE_SERVICE_URL and PROFILE_SERVICE_TOKEN are required")
	}

	oidcCfg := oidcConfigFromEnv()
	cookieSecure, err := cookieSecureFromEnv()
	if err != nil {
		log.Fatalf("cookie security config: %v", err)
	}

	webOrigin := strings.TrimRight(strings.TrimSpace(os.Getenv("WEB_ORIGIN")), "/")
	corsOrigins := parseCORSOrigins(os.Getenv("CORS_ORIGINS"))

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := initDB(db); err != nil {
		log.Fatalf("init db: %v", err)
	}

	outboundClient := &http.Client{Timeout: outboundHTTPTimeout}

	h := &handler{
		db:            db,
		cookieSecure:  cookieSecure,
		webOrigin:     webOrigin,
		oidc:          oidcCfg,
		httpClient:    outboundClient,
		authLimiter:   newAuthRateLimiter(db),
		profileClient: newHTTPProfileClient(profileServiceURL, profileServiceToken, outboundClient),
	}

	if oidcCfg.Enabled() {
		oidcCtx := context.WithValue(context.Background(), oauth2.HTTPClient, outboundClient)
		rt, err := newOIDCRuntime(oidcCtx, oidcCfg)
		if err != nil {
			log.Fatalf("oidc init: %v", err)
		}
		h.oidcRT = rt
		log.Printf("oidc: configured with issuer %s", oidcCfg.Issuer)
	} else {
		log.Printf("oidc: not configured (set OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET)")
	}

	m := mailerFromEnv()
	if m.configured() {
		pollInterval := 5 * time.Minute
		if raw := strings.TrimSpace(os.Getenv("REMINDER_POLL_INTERVAL")); raw != "" {
			parsed, err := time.ParseDuration(raw)
			if err != nil {
				log.Fatalf("invalid REMINDER_POLL_INTERVAL: %v", err)
			}
			pollInterval = parsed
		}
		startReminderDispatcher(context.Background(), db, m, h.profileClient, pollInterval)
		log.Printf("reminders: dispatcher started, polling every %s", pollInterval)
	} else {
		log.Printf("reminders: not configured (set SMTP_HOST, SMTP_PORT, SMTP_FROM)")
	}

	r := newRouter(h, corsOrigins)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(newHTTPServer(":"+port, r).ListenAndServe())
}

func parseCORSOrigins(val string) []string {
	var origins []string
	for _, o := range strings.Split(val, ",") {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}
