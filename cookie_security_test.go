package main

import "testing"

func TestCookieSecureFromEnvDefaultsToFalseWithoutHTTPS(t *testing.T) {
	t.Setenv("BASE_URL", "")
	t.Setenv("GITHUB_CALLBACK_URL", "")
	t.Setenv("COOKIE_SECURE", "")

	secure, err := cookieSecureFromEnv(githubOAuthConfig{})
	if err != nil {
		t.Fatalf("cookieSecureFromEnv returned error: %v", err)
	}
	if secure {
		t.Fatal("expected insecure cookies by default for non-HTTPS local configuration")
	}
}

func TestCookieSecureFromEnvDefaultsToTrueForHTTPSBaseURL(t *testing.T) {
	t.Setenv("BASE_URL", "https://daysuntil.example.com")
	t.Setenv("COOKIE_SECURE", "")

	secure, err := cookieSecureFromEnv(githubOAuthConfig{})
	if err != nil {
		t.Fatalf("cookieSecureFromEnv returned error: %v", err)
	}
	if !secure {
		t.Fatal("expected secure cookies when BASE_URL uses https")
	}
}

func TestCookieSecureFromEnvDefaultsToTrueForHTTPSCallbackURL(t *testing.T) {
	t.Setenv("BASE_URL", "")
	t.Setenv("COOKIE_SECURE", "")

	secure, err := cookieSecureFromEnv(githubOAuthConfig{
		CallbackURL: "https://daysuntil.example.com/api/oauth/github/callback",
	})
	if err != nil {
		t.Fatalf("cookieSecureFromEnv returned error: %v", err)
	}
	if !secure {
		t.Fatal("expected secure cookies when OAuth callback uses https")
	}
}

func TestCookieSecureFromEnvRejectsFalseForHTTPSDeployment(t *testing.T) {
	t.Setenv("BASE_URL", "https://daysuntil.example.com")
	t.Setenv("COOKIE_SECURE", "false")

	_, err := cookieSecureFromEnv(githubOAuthConfig{})
	if err == nil {
		t.Fatal("expected config error for COOKIE_SECURE=false on HTTPS deployment")
	}
}

func TestCookieSecureFromEnvAllowsExplicitFalseForLocalHTTP(t *testing.T) {
	t.Setenv("BASE_URL", "http://localhost:8080")
	t.Setenv("COOKIE_SECURE", "false")

	secure, err := cookieSecureFromEnv(githubOAuthConfig{})
	if err != nil {
		t.Fatalf("cookieSecureFromEnv returned error: %v", err)
	}
	if secure {
		t.Fatal("expected insecure cookies for explicit local HTTP configuration")
	}
}

func TestCookieSecureFromEnvRejectsInvalidValue(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "maybe")

	_, err := cookieSecureFromEnv(githubOAuthConfig{})
	if err == nil {
		t.Fatal("expected config error for invalid COOKIE_SECURE value")
	}
}
