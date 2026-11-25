package store_test

import "testing"

// A Kindle app's log after a sync that went fine replaces the last such log; one with trouble in it is kept.
func TestCleanSyncLogsReplaceEachOther(t *testing.T) {
	st := newStore(t)
	u, err := st.CreateUser("tester", "hash", "admin")
	if err != nil {
		t.Fatal(err)
	}
	ok := "KINDLE APP LOG (after a sync)\n\n2026-11-25 sync: Reported 83 books."
	bad := "KINDLE APP LOG (after a sync)\n\n2026-11-25 sync: Couldn't read the Kindle library."
	for _, body := range []string{ok, ok, bad, ok} {
		if err := st.SaveDeviceLog(u, "Kindle", "app 1.19.0", body); err != nil {
			t.Fatal(err)
		}
	}
	logs, _ := st.DeviceLogs()
	if len(logs) != 2 { // the last clean one, and the one with trouble
		t.Fatalf("logs kept: %d", len(logs))
	}
}
