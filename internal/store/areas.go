package store

import (
	"errors"
	"strings"
)

// The main admin and the other admins' areas. The main admin (Owner) reaches
// everything and alone decides who is an admin and which areas each other
// admin may reach. Every admin manages kids; the areas cover the rest.
const (
	AreaAI       = "ai"       // AI engines, keys, models, prices, limits, AI machines, custom AI filters
	AreaDeep     = "deep"     // Deep Scans: start, review, settings
	AreaCalibre  = "calibre"  // the Calibre library, removing books, formats, duplicates, delete requests
	AreaServices = "services" // Send-to-Kindle email, Discover lists
	AreaSystem   = "system"   // features, remote access, safe mode, backups, updates, checks, usage
	AreaUsers    = "users"    // other parents' accounts
)

// AllAreas lists the areas in the order they're shown.
var AllAreas = []string{AreaAI, AreaDeep, AreaCalibre, AreaServices, AreaSystem, AreaUsers}

// DefaultAdminAreas is what a new admin may reach until the main admin
// changes it.
const DefaultAdminAreas = AreaDeep

// Can reports whether u may reach an admin area.
func (u *User) Can(area string) bool {
	if u == nil || u.Role != RoleAdmin {
		return false
	}
	if u.Owner {
		return true
	}
	for _, a := range strings.Split(u.AdminAreas, ",") {
		if a == area {
			return true
		}
	}
	return false
}

// Areas lists the areas u may reach (all of them for the main admin).
func (u *User) Areas() []string {
	out := []string{}
	for _, a := range AllAreas {
		if u.Can(a) {
			out = append(out, a)
		}
	}
	return out
}

// cleanAreas keeps known areas, in order, without repeats.
func cleanAreas(areas []string) string {
	want := map[string]bool{}
	for _, a := range areas {
		want[strings.TrimSpace(a)] = true
	}
	var out []string
	for _, a := range AllAreas {
		if want[a] {
			out = append(out, a)
		}
	}
	return strings.Join(out, ",")
}

// SetAdminAreas sets what an admin who isn't the main admin may reach.
func (s *Store) SetAdminAreas(userID int64, areas []string) error {
	_, err := s.DB.Exec(`UPDATE users SET admin_areas = ? WHERE id = ? AND owner = 0`, cleanAreas(areas), userID)
	return err
}

// MakeOwner hands the main admin over to another admin; the old one keeps
// every area as an ordinary admin.
func (s *Store) MakeOwner(userID int64) error {
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE users SET owner = 1, admin_areas = '' WHERE id = ? AND role = 'admin'`, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("only an admin can be the main admin")
	}
	if _, err := tx.Exec(`UPDATE users SET owner = 0, admin_areas = ? WHERE owner = 1 AND id != ?`, cleanAreas(AllAreas), userID); err != nil {
		return err
	}
	return tx.Commit()
}

// EnsureOwner makes the earliest admin the main admin when there is none
// (a new install's first admin).
func (s *Store) EnsureOwner() {
	_, _ = s.DB.Exec(`UPDATE users SET owner = 1, admin_areas = '' WHERE id = (SELECT MIN(id) FROM users WHERE role = 'admin')
		AND NOT EXISTS (SELECT 1 FROM users WHERE owner = 1)`)
}
