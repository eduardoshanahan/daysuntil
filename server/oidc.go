package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type oidcConfig struct {
	Issuer      string
	ClientID    string
	CallbackURL string
}

type oidcRuntime struct {
	verifier  *gooidc.IDTokenVerifier
	oauth2Cfg *oauth2.Config
}

func oidcConfigFromEnv() oidcConfig {
	issuer := strings.TrimSpace(os.Getenv("OIDC_ISSUER"))
	clientID := strings.TrimSpace(os.Getenv("OIDC_CLIENT_ID"))
	callbackURL := strings.TrimSpace(os.Getenv("OIDC_CALLBACK_URL"))
	if callbackURL == "" {
		baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("BASE_URL")), "/")
		if baseURL != "" {
			callbackURL = baseURL + "/api/oidc/callback"
		}
	}
	return oidcConfig{
		Issuer:      issuer,
		ClientID:    clientID,
		CallbackURL: callbackURL,
	}
}

func (c oidcConfig) Enabled() bool {
	return c.Issuer != "" && c.ClientID != "" && c.CallbackURL != ""
}

func newOIDCRuntime(ctx context.Context, cfg oidcConfig) (*oidcRuntime, error) {
	provider, err := gooidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc provider discovery failed for %q: %w", cfg.Issuer, err)
	}

	oauth2Cfg := &oauth2.Config{
		ClientID:    cfg.ClientID,
		RedirectURL: cfg.CallbackURL,
		Endpoint:    provider.Endpoint(),
		Scopes:      []string{gooidc.ScopeOpenID, "profile", "email"},
	}

	return &oidcRuntime{
		verifier:  provider.Verifier(&gooidc.Config{ClientID: cfg.ClientID}),
		oauth2Cfg: oauth2Cfg,
	}, nil
}
