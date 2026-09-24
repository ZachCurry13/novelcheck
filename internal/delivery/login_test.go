package delivery

import (
	"errors"
	"strings"
	"testing"
)

func TestCleanGmailAppPassword(t *testing.T) {
	c := SMTPConfig{Host: " smtp.gmail.com ", Port: "587", Username: " me@gmail.com ", Password: "abcd efgh ijkl mnop"}.Clean()
	if c.Password != "abcdefghijklmnop" || c.Host != "smtp.gmail.com" || c.Username != "me@gmail.com" {
		t.Fatalf("clean: %+v", c)
	}
	other := SMTPConfig{Host: "mail.example.com", Password: "pass with spaces"}.Clean()
	if other.Password != "pass with spaces" {
		t.Fatal("non-Gmail passwords must be left alone")
	}
}

func TestAuthErrorExplainsGmail(t *testing.T) {
	gerr := errors.New("535 5.7.8 Username and Password not accepted. For more information, go to\n5.7.8 https://support.google.com/mail/?p=BadCredentials")
	msg := authError(SMTPConfig{Host: "smtp.gmail.com", Username: "me"}, gerr).Error()
	for _, want := range []string{"App Password", "apppasswords", "full Gmail address", "535 5.7.8"} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in %q", want, msg)
		}
	}
	if strings.Contains(msg, "\n") {
		t.Error("message should be one line")
	}
	other := authError(SMTPConfig{Host: "smtp.example.com"}, errors.New("535 bad auth")).Error()
	if !strings.Contains(other, "full email address") {
		t.Errorf("generic hint: %q", other)
	}
	if e := errors.New("connection reset"); authError(SMTPConfig{}, e) != e {
		t.Error("unrelated errors pass through")
	}
}
