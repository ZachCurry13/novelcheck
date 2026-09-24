package delivery

import (
	"errors"
	"strings"
)

// isGoogle reports whether the account is Gmail / Google Workspace.
func (c SMTPConfig) isGoogle() bool {
	h := strings.ToLower(c.Host)
	u := strings.ToLower(c.Username)
	return strings.HasSuffix(h, "gmail.com") || strings.HasSuffix(h, "googlemail.com") ||
		strings.HasSuffix(h, "smtp-relay.google.com") || strings.HasSuffix(u, "@gmail.com")
}

// Clean tidies hand-typed settings: trims spaces and, for Gmail, removes the
// spaces Google shows inside App Passwords ("abcd efgh ijkl mnop").
func (c SMTPConfig) Clean() SMTPConfig {
	c.Host = strings.TrimSpace(c.Host)
	c.Port = strings.TrimSpace(c.Port)
	c.Username = strings.TrimSpace(c.Username)
	c.From = strings.TrimSpace(c.From)
	if c.isGoogle() {
		c.Password = strings.Join(strings.Fields(c.Password), "")
	}
	return c
}

// authError explains a rejected login in plain words.
func authError(c SMTPConfig, err error) error {
	msg := err.Error()
	rejected := strings.Contains(msg, "535") || strings.Contains(msg, "534") ||
		strings.Contains(msg, "BadCredentials") || strings.Contains(strings.ToLower(msg), "password")
	if !rejected {
		return err
	}
	if c.isGoogle() {
		hint := "Gmail rejected the login. Gmail doesn't accept your normal Google password here: " +
			"turn on 2-Step Verification, create an App Password at myaccount.google.com/apppasswords, " +
			"and paste its 16 letters as the SMTP password."
		if !strings.Contains(c.Username, "@") {
			hint += " Also use your full Gmail address as the SMTP username."
		}
		return errors.New(hint + " (Google said: " + firstLine(msg) + ")")
	}
	return errors.New("the email server rejected the username or password. The username is usually your full email address. (" + firstLine(msg) + ")")
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
