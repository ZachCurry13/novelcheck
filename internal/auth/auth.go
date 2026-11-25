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
	SessionDays int // fallback when the admin setting is unset
}

// shortSession is how long a "don't keep me signed in" login lasts without
// activity; its cookie also disappears when the browser is closed.
const shortSession = 12 * 3600

// renewAfter throttles rolling renewal to one database write per day.
const renewAfter = 86400

// lifetime returns the remembered-session length in seconds, from the
// admin's "session_days" setting (1-365), else the configured default.
func (m *Manager) lifetime() int {
	days := m.Store.SettingInt(store.KeySessionDays)
	if days < 1 || days > 365 {
		days = m.SessionDays
	}
	if days < 1 {
		days = 30
	}
	return days * 86400
}

func (m *Manager) setCookie(w http.ResponseWriter, r *http.Request, token string, remember bool) {
	c := &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   IsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	}
	if remember {
		c.MaxAge = m.lifetime()
	}
	http.SetCookie(w, c)
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

// Login verifies credentials and sets the session cookie. remember=true keeps
// the user signed in across browser restarts, renewed while they keep using
// the app; false signs them out when the browser closes.
func (m *Manager) Login(w http.ResponseWriter, r *http.Request, username, password string, remember bool) (*store.User, error) {
	u, err := m.Store.UserByName(username)
	if err != nil {
		// Burn comparable time so usernames can't be enumerated by timing.
		bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return nil, errors.New("invalid username or password")
	}
	if !CheckPassword(u.PasswordHash, password) {
		return nil, errors.New("invalid username or password")
	}
	if err := m.startSession(w, r, u.ID, remember); err != nil {
		return nil, err
	}
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

// current resolves the session cookie and, for active users, rolls the
// expiry forward (at most once a day) so regular users stay signed in.
func (m *Manager) current(w http.ResponseWriter, r *http.Request) *store.User {
	sess := m.session(w, r)
	if sess == nil {
		return nil
	}
	return sess.User
}

// session is current with the whole session (who, and who is viewing as them).
func (m *Manager) session(w http.ResponseWriter, r *http.Request) *store.Session {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	sess, err := m.Store.SessionByToken(hashToken(c.Value))
	if err != nil {
		return nil
	}
	if sess.ViewedBy != 0 {
		return sess // a View as session is never extended
	}
	life := shortSession
	if sess.Remember {
		life = m.lifetime()
	}
	if life-sess.SecondsLeft >= min(renewAfter, life/2) {
		if m.Store.ExtendSession(hashToken(c.Value), life) == nil && sess.Remember {
			m.setCookie(w, r, c.Value, true)
		}
	}
	return sess
}

// RequireUser rejects unauthenticated requests with 401 and stores the user
// in the request context.
func (m *Manager) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := m.session(w, r)
		if sess == nil {
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, sess.User)
		if sess.ViewedBy != 0 {
			// View as: an admin looking at the app as this person. Looking only: nothing is changed in their name.
			if r.Method != http.MethodGet && r.Method != http.MethodHead && !viewAllowed[r.URL.Path] {
				http.Error(w, `{"error":"You're viewing the app as `+jsonText(sess.User.Username)+`, so changes are off. Go back to your own account to change things."}`, http.StatusForbidden)
				return
			}
			ctx = context.WithValue(ctx, viewerKey{}, sess.ViewedBy)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
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

// RequireManager admits admins and editors; mount after RequireUser.
func RequireManager(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u := UserFrom(r); u == nil || !u.CanManage() {
			http.Error(w, `{"error":"editor or admin access required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// WithUser is ctx as if u had signed in (background work done for a person, like keeping admin pages ready).
func WithUser(ctx context.Context, u *store.User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

func UserFrom(r *http.Request) *store.User {
	u, _ := r.Context().Value(ctxKey{}).(*store.User)
	return u
}

// BurnTime spends about as long as a password check, for an unknown name, so the answer's time doesn't tell which names exist.
func BurnTime(password string) { _ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password)) }
