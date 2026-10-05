package store_test

import (
	"fmt"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The first start of a version with a main admin: the earliest admin
// becomes it, and the other admins keep every area until it's changed.
func TestFirstOwnerOnUpgrade(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=foreign_keys(1)", t.Name())
	before, err := db.OpenDSN(dsn) // an existing install: admins, nobody the main admin yet
	if err != nil {
		t.Fatal(err)
	}
	defer before.Close()
	for _, name := range []string{"first", "second"} {
		if _, err := before.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?, 'x', 'admin')`, name); err != nil {
			t.Fatal(err)
		}
	}
	after, err := db.OpenDSN(dsn) // the upgrade's start runs the migration
	if err != nil {
		t.Fatal(err)
	}
	defer after.Close()
	st := store.New(after)
	first, _ := st.UserByName("first")
	second, _ := st.UserByName("second")
	if !first.Owner || second.Owner || len(second.Areas()) != len(store.AllAreas) {
		t.Fatalf("first owner %v; second owner %v areas %v", first.Owner, second.Owner, second.Areas())
	}

	// Later starts change nothing: the main admin's choices stay.
	_ = st.SetAdminAreas(second.ID, []string{store.AreaDeep})
	again, err := db.OpenDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	second, _ = store.New(again).UserByName("second")
	if got := second.Areas(); len(got) != 1 || got[0] != store.AreaDeep {
		t.Fatalf("areas after a restart: %v", got)
	}
}
