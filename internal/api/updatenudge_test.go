package api

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestVersionsBehind(t *testing.T) {
	if versionsBehind("1.2.0", "1.26.0") != 24 || versionsBehind("1.26.0", "1.26.0") != 0 || versionsBehind("1.30.0", "1.26.0") != 0 || versionsBehind("1.2.0", "2.0.0") != 0 {
		t.Fatal("versions behind")
	}
}

// One notice per person, Kindle and version; a new version is new again; a Kindle that is up to date or long silent gets none.
func TestNudgeOncePerVersion(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	s := &Server{Store: st}
	u, err := st.CreateUser("jenna", "hash", store.RoleEditor)
	if err != nil {
		t.Fatal(err)
	}
	id := u.ID
	st.SaveDeviceInfo(id, "Her Kindle", "app 1.2.0")
	if n := s.nudgeOnce(); n != 1 {
		t.Fatalf("first look: %d", n)
	}
	if n := s.nudgeOnce(); n != 0 {
		t.Fatalf("told once: %d", n)
	}
	_ = st.SetSetting(nudgeKey(id, "Her Kindle", "app"), "1.1.0") // as if told about an older version
	if n := s.nudgeOnce(); n != 1 {
		t.Fatalf("a new version is told again: %d", n)
	}
	_, _ = st.DB.Exec(`UPDATE device_info SET reported_at = 1`) // silent for ages
	_ = st.SetSetting(nudgeKey(id, "Her Kindle", "app"), "")
	if n := s.nudgeOnce(); n != 0 {
		t.Fatalf("not nagged when silent: %d", n)
	}
}
