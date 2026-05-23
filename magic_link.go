package main

import (
	"fmt"
	"net/smtp"
	"net/url"
	"os"
	"strings"
)

type magicLinkConfig struct {
	BaseURL  string
	From     string
	SMTPHost string
	SMTPPort string
	Username string
	Password string
	send     func(email, rawToken string) error
}

func magicLinkConfigFromEnv() (magicLinkConfig, error) {
	cfg := magicLinkConfig{
		BaseURL:  strings.TrimRight(strings.TrimSpace(os.Getenv("BASE_URL")), "/"),
		From:     strings.TrimSpace(os.Getenv("SMTP_FROM")),
		SMTPHost: strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPort: strings.TrimSpace(os.Getenv("SMTP_PORT")),
		Username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		Password: strings.TrimSpace(os.Getenv("SMTP_PASSWORD")),
	}

	if !cfg.Enabled() {
		if cfg.BaseURL == "" && (cfg.From != "" || cfg.SMTPHost != "" || cfg.SMTPPort != "" || cfg.Username != "" || cfg.Password != "") {
			return magicLinkConfig{}, fmt.Errorf("BASE_URL is required when SMTP magic link settings are provided")
		}
		return cfg, nil
	}

	if !urlUsesHTTPS(cfg.BaseURL) && !strings.HasPrefix(cfg.BaseURL, "http://localhost") && !strings.HasPrefix(cfg.BaseURL, "http://127.0.0.1") {
		return magicLinkConfig{}, fmt.Errorf("BASE_URL must use https outside local development for magic links")
	}

	return cfg, nil
}

func (c magicLinkConfig) Enabled() bool {
	return c.BaseURL != "" && c.From != "" && c.SMTPHost != "" && c.SMTPPort != ""
}

func (c magicLinkConfig) loginURL(rawToken string) string {
	values := url.Values{}
	values.Set("login_token", rawToken)
	return c.BaseURL + "/?" + values.Encode()
}

func (c magicLinkConfig) sendLoginLink(email, rawToken string) error {
	if c.send != nil {
		return c.send(email, rawToken)
	}

	if !c.Enabled() {
		return fmt.Errorf("magic link sign-in is not configured")
	}

	loginURL := c.loginURL(rawToken)
	message := strings.Join([]string{
		"To: " + email,
		"From: " + c.From,
		"Subject: Your DaysUntil sign-in link",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		"Use this link to sign in to DaysUntil:",
		loginURL,
		"",
		"This link expires in 15 minutes and can only be used once.",
		"",
	}, "\r\n")

	addr := c.SMTPHost + ":" + c.SMTPPort
	var smtpAuth smtp.Auth
	if c.Username != "" || c.Password != "" {
		smtpAuth = smtp.PlainAuth("", c.Username, c.Password, c.SMTPHost)
	}

	return smtp.SendMail(addr, smtpAuth, c.From, []string{email}, []byte(message))
}
