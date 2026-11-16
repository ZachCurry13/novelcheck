package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// View as: an admin sees the app exactly as another person does (their library, rules, pages) without their
// password. It is a separate session that only looks: every change is refused, it is never extended, it ends
// after two hours, and "Back to my account" signs the admin in again.

type viewerKey struct{}

const viewLife = 2 * 3600

// viewAllowed are the only changes a View as session may make: ending it, or signing out.
var viewAllowed = map[string]bool{"/api/me/view-as": true, "/api/auth/logout": true}

// ViewerFrom is the admin viewing the app as the request's user, or 0.
func ViewerFrom(r *http.Request) int64 {
	id, _ := r.Context().Value(viewerKey{}).(int64)
	return id
}

// ViewAs swaps the admin's session for a read-only one as userID.
func (m *Manager) ViewAs(w http.ResponseWriter, r *http.Request, userID, adminID int64) error {
	if c, err := r.Cookie(CookieName); err == nil {
		_ = m.Store.DeleteSession(hashToken(c.Value))
	}
	token := RandomToken(32)
	if err := m.Store.CreateViewSession(hashToken(token), userID, adminID, viewLife); err != nil {
		return err
	}
	m.setCookie(w, r, token, false)
	return nil
}

// StopViewing ends a View as session and signs its admin in again.
func (m *Manager) StopViewing(w http.ResponseWriter, r *http.Request) error {
	sess := m.session(w, r)
	if sess == nil || sess.ViewedBy == 0 {
		return errors.New("not viewing as anyone")
	}
	if c, err := r.Cookie(CookieName); err == nil {
		_ = m.Store.DeleteSession(hashToken(c.Value))
	}
	return m.startSession(w, r, sess.ViewedBy, true)
}

func jsonText(s string) string {
	b, _ := json.Marshal(s)
	return strings.Trim(string(b), `"`)
}
