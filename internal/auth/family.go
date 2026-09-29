package auth

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/zachcurry13/novelcheck/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// Family devices: a parent marks a shared device, which then keeps a
// private key in its own cookie. With it, the "Who's reading?" screen can
// switch profiles without passwords (kids), or with a PIN (anyone who set
// one) or the password (parents without a PIN).

// FamilyCookie holds a family device's key.
const FamilyCookie = "nc_family"

// familyLife is how long the key cookie lasts; it's renewed on use.
const familyLife = 400 * 86400

var pinRE = regexp.MustCompile(`^[0-9]{4}$`)

// HashPIN checks a PIN is 4 digits and returns its digest.
func HashPIN(pin string) (string, error) {
	if !pinRE.MatchString(pin) {
		return "", errors.New("a PIN is 4 digits")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPIN compares a PIN with its digest.
func CheckPIN(hash, pin string) bool {
	return hash != "" && bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin)) == nil
}

func (m *Manager) setFamilyCookie(w http.ResponseWriter, r *http.Request, token string, age int) {
	http.SetCookie(w, &http.Cookie{Name: FamilyCookie, Value: token, Path: "/", MaxAge: age,
		HttpOnly: true, Secure: IsHTTPS(r), SameSite: http.SameSiteLaxMode})
}

// FamilyDevice is the family device making the request, or nil.
func (m *Manager) FamilyDevice(w http.ResponseWriter, r *http.Request) *store.FamilyDevice {
	c, err := r.Cookie(FamilyCookie)
	if err != nil || c.Value == "" {
		return nil
	}
	d, err := m.Store.FamilyDeviceByToken(hashToken(c.Value))
	if err != nil {
		return nil
	}
	if w != nil {
		m.setFamilyCookie(w, r, c.Value, familyLife) // keep it from expiring while in use
	}
	return d
}

// MakeFamilyDevice marks the requesting device as a family device.
func (m *Manager) MakeFamilyDevice(w http.ResponseWriter, r *http.Request, name, by string) error {
	token := RandomToken(32)
	if _, err := m.Store.AddFamilyDevice(hashToken(token), name, by); err != nil {
		return err
	}
	m.setFamilyCookie(w, r, token, familyLife)
	return nil
}

// ForgetFamilyDevice stops the requesting device being a family device.
func (m *Manager) ForgetFamilyDevice(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(FamilyCookie); err == nil && c.Value != "" {
		_ = m.Store.RemoveFamilyDeviceByToken(hashToken(c.Value))
	}
	m.setFamilyCookie(w, r, "", -1)
}

// SwitchTo signs the device into someone's profile (a remembered session),
// ending the current one.
func (m *Manager) SwitchTo(w http.ResponseWriter, r *http.Request, userID int64) error {
	if c, err := r.Cookie(CookieName); err == nil {
		_ = m.Store.DeleteSession(hashToken(c.Value))
	}
	return m.startSession(w, r, userID, true)
}

// startSession creates a session and sets its cookie.
func (m *Manager) startSession(w http.ResponseWriter, r *http.Request, userID int64, remember bool) error {
	token := RandomToken(32)
	life := shortSession
	if remember {
		life = m.lifetime()
	}
	if err := m.Store.CreateSession(hashToken(token), userID, life, remember); err != nil {
		return err
	}
	m.setCookie(w, r, token, remember)
	return nil
}
