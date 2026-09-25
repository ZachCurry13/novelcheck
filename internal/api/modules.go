package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// meJSON is the signed-in user plus which features are on, so the app can
// hide what the admin turned off.
type meJSON struct {
	*store.User
	Modules      map[string]bool `json:"modules"`
	DeliveryFrom string          `json:"delivery_from"` // the Send-to-Kindle sender, for Amazon's approved list
}

func (s *Server) me(u *store.User) meJSON {
	return meJSON{User: u, Modules: s.Store.Modules(), DeliveryFrom: s.Store.Setting(store.KeySMTPFrom)}
}

// requireModule refuses API calls for a feature the admin turned off.
func (s *Server) requireModule(key, what string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !s.Store.SettingBool(key) {
				writeErr(w, http.StatusForbidden, what+" is turned off (Admin → Features)")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
