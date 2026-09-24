// Package auth implements cookie-based sessions and role-based access control.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const CookieName = "nc_session"

// MinPasswordLen is enforced on every password set through the API.
const MinPasswordLen = 8

type ctxKey struct{}

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("novelcheck-timing-guard"), bcrypt.DefaultCost)

type Manager struct {
	Store       *store.Store
	SessionDays int
}

func HashPassword(pw string) (string, error) {
	if len(pw) < MinPasswordLen {
		return "", errors.New("password must be at least 8 characters")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// RandomToken returns n random bytes, base64url-encoded.
func RandomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// hashToken stores only a digest of the session token so a leaked database
// backup cannot be replayed as live sessions.
func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

// Login verifies credentials and sets the session cookie.
func (m *Manager) Login(w http.ResponseWriter, r *http.Request, username, password string) (*store.User, error) {
	u, err := m.Store.UserByName(username)
	if err != nil {
		// Burn comparable time so usernames can't be enumerated by timing.
		bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return nil, errors.New("invalid username or password")
	}
	if !CheckPassword(u.PasswordHash, password) {
		return nil, errors.New("invalid username or password")
	}
	token := RandomToken(32)
	if err := m.Store.CreateSession(hashToken(token), u.ID, m.SessionDays); err != nil {
		return nil, err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   m.SessionDays * 86400,
		HttpOnly: true,
		Secure:   IsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	return u, nil
}

func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		_ = m.Store.DeleteSession(hashToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: IsHTTPS(r), SameSite: http.SameSiteLaxMode})
}

// IsHTTPS detects TLS directly or via a trusted proxy / Cloudflare Tunnel.
func IsHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// Current resolves the session cookie to a user, or nil.
func (m *Manager) Current(r *http.Request) *store.User {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	u, err := m.Store.SessionUser(hashToken(c.Value))
	if err != nil {
		return nil
	}
	return u
}

// RequireUser rejects unauthenticated requests with 401 and stores the user
// in the request context.
func (m *Manager) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := m.Current(r)
		if u == nil {
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

// RequireAdmin must be mounted after RequireUser.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u := UserFrom(r); u == nil || !u.IsAdmin() {
			http.Error(w, `{"error":"admin access required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func UserFrom(r *http.Request) *store.User {
	u, _ := r.Context().Value(ctxKey{}).(*store.User)
	return u
}
