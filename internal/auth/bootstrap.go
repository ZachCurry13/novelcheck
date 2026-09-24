package auth

import (
	"log"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Bootstrap creates the first admin account when the database has no users.
// If no password is configured a random one is generated and logged once.
func Bootstrap(s *store.Store, username, password string) error {
	n, err := s.CountUsers()
	if err != nil || n > 0 {
		return err
	}
	generated := password == ""
	if generated {
		password = RandomToken(12)
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	if _, err := s.CreateUser(username, hash, store.RoleAdmin); err != nil {
		return err
	}
	if generated {
		log.Printf("created admin user %q with generated password: %s (change it after first login)", username, password)
	} else {
		log.Printf("created admin user %q from NOVELCHECK_ADMIN_PASSWORD", username)
	}
	return nil
}
