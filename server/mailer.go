package main

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

// mailer sends reminder emails via the homelab SMTP relay. Uses stdlib
// net/smtp only — no new dependency, matching this repo's minimal-deps
// style (see go.mod).
type mailer struct {
	host     string
	port     string
	from     string
	username string
	password string
}

func mailerFromEnv() *mailer {
	return &mailer{
		host:     strings.TrimSpace(os.Getenv("SMTP_HOST")),
		port:     strings.TrimSpace(os.Getenv("SMTP_PORT")),
		from:     strings.TrimSpace(os.Getenv("SMTP_FROM")),
		username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		password: os.Getenv("SMTP_PASSWORD"),
	}
}

// configured mirrors the enabled-if-env-set pattern already used for OIDC
// (oidcConfig.Enabled) — reminders are dispatched only when SMTP is set up.
func (m *mailer) configured() bool {
	return m.host != "" && m.port != "" && m.from != ""
}

func (m *mailer) send(to, subject, body string) error {
	if !m.configured() {
		return fmt.Errorf("mailer not configured")
	}

	addr := m.host + ":" + m.port
	msg := "From: " + m.from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" + body + "\r\n"

	var auth smtp.Auth
	if m.username != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}
	return smtp.SendMail(addr, auth, m.from, []string{to}, []byte(msg))
}
