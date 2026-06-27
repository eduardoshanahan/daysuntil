package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

func (h *handler) oidcStart(w http.ResponseWriter, r *http.Request) {
	if h.oidcRT == nil {
		http.Error(w, "sign-in is not configured", http.StatusServiceUnavailable)
		return
	}

	state, err := randomToken(32)
	if err != nil {
		http.Error(w, "failed to start sign-in", http.StatusInternalServerError)
		return
	}

	verifier := oauth2.GenerateVerifier()
	setOIDCStateCookie(w, state, h.cookieSecure)
	setOIDCPKCECookie(w, verifier, h.cookieSecure)
	http.Redirect(w, r, h.oidcRT.oauth2Cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

func (h *handler) oidcCallback(w http.ResponseWriter, r *http.Request) {
	if h.oidcRT == nil {
		http.Error(w, "sign-in is not configured", http.StatusServiceUnavailable)
		return
	}

	home := h.homeURL()

	if oauthError := strings.TrimSpace(r.URL.Query().Get("error")); oauthError != "" {
		clearOIDCStateCookie(w, h.cookieSecure)
		http.Redirect(w, r, home+"?auth_error="+url.QueryEscape("Sign-in was cancelled or denied."), http.StatusFound)
		return
	}

	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	cookie, err := r.Cookie(oidcStateCookie)
	if err != nil || code == "" || state == "" || cookie.Value != state {
		clearOIDCStateCookie(w, h.cookieSecure)
		http.Redirect(w, r, home+"?auth_error="+url.QueryEscape("Sign-in could not be verified. Please try again."), http.StatusFound)
		return
	}
	clearOIDCStateCookie(w, h.cookieSecure)

	pkceCookie, err := r.Cookie(oidcPKCECookie)
	clearOIDCPKCECookie(w, h.cookieSecure)
	if err != nil || pkceCookie.Value == "" {
		http.Redirect(w, r, home+"?auth_error="+url.QueryEscape("Sign-in could not be verified. Please try again."), http.StatusFound)
		return
	}

	ctx := context.Background()
	token, err := h.oidcRT.oauth2Cfg.Exchange(ctx, code, oauth2.VerifierOption(pkceCookie.Value))
	if err != nil {
		log.Printf("oidc: code exchange failed: %v", err)
		http.Redirect(w, r, home+"?auth_error="+url.QueryEscape("Sign-in failed. Please try again."), http.StatusFound)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		log.Printf("oidc: id_token missing from token response")
		http.Redirect(w, r, home+"?auth_error="+url.QueryEscape("Sign-in failed. Please try again."), http.StatusFound)
		return
	}

	idToken, err := h.oidcRT.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		log.Printf("oidc: id_token verification failed: %v", err)
		http.Redirect(w, r, home+"?auth_error="+url.QueryEscape("Sign-in failed. Please try again."), http.StatusFound)
		return
	}

	var claims struct {
		Sub               string `json:"sub"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		log.Printf("oidc: claims extraction failed: %v", err)
		http.Redirect(w, r, home+"?auth_error="+url.QueryEscape("Sign-in failed. Please try again."), http.StatusFound)
		return
	}

	displayName := claims.Name
	if displayName == "" {
		displayName = claims.PreferredUsername
	}

	user, err := findOrCreateOIDCUser(h.db, claims.Sub, displayName)
	if err != nil {
		log.Printf("oidc: find/create user failed: %v", err)
		http.Redirect(w, r, home+"?auth_error="+url.QueryEscape("Sign-in failed. Please try again."), http.StatusFound)
		return
	}

	sessionToken, expiresAt, err := createSession(h.db, user.ID)
	if err != nil {
		log.Printf("oidc: session creation failed: %v", err)
		http.Redirect(w, r, home+"?auth_error="+url.QueryEscape("Sign-in failed. Please try again."), http.StatusFound)
		return
	}

	setSessionCookie(w, sessionToken, expiresAt, h.cookieSecure, h.crossOrigin())
	http.Redirect(w, r, home, http.StatusFound)
}
