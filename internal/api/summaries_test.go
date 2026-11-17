package api_test

import (
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/push"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The reading summaries: on a Monday at 9 or later the week that ended is summed up (hours and days), and on the
// 1st the month; each goes once, only to people with reading in that period and phone notifications on, and not
// to someone who turned that summary off.
func TestReadingSummaries(t *testing.T) {
	var server *api.Server
	srv, st := setupWith(t, func(s *api.Server) { server = s })
	admin := login(t, srv, "admin", "adminpass1")
	u, _ := st.UserByName("admin")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "sister", "password": "sisterpass1", "role": "editor"}, true)
	sis, _ := st.UserByName("sister")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "quiet", "password": "quietpass12", "role": "editor"}, true)
	quiet, _ := st.UserByName("quiet")

	hour := func(y int, m time.Month, d, h int) int64 { return time.Date(y, m, d, h, 0, 0, 0, time.UTC).Unix() }
	rows := []store.ReadingHour{
		{Title: "A", Hour: hour(2026, 10, 28, 20), Seconds: 2 * 3600, Pages: 40}, // last week and October
		{Title: "A", Hour: hour(2026, 10, 30, 21), Seconds: 3600, Pages: 20},     // last week and October
		{Title: "B", Hour: hour(2026, 10, 2, 8), Seconds: 3 * 3600, Pages: 70},   // October only
	}
	for _, id := range []int64{u.ID, sis.ID, quiet.ID} {
		_ = st.SaveReadingHours(id, rows)
	}
	for _, id := range []int64{u.ID, quiet.ID} { // sister has no phone notifications on
		_ = st.SavePushSub(store.PushSub{UserID: id, Endpoint: "https://push.example/" + itoa(id), P256dh: "k", Auth: "a", Scope: "all", Device: "Phone"})
	}
	_ = st.SetSummaryPrefs(quiet.ID, store.SummaryPrefs{Week: false, Month: true, Year: true})

	got := map[int64][]push.Message{}
	send := func(id int64, m push.Message) { got[id] = append(got[id], m) }
	srv0 := server
	monday := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC) // Monday 2 November, 10:00
	if n := srv0.SendSummaries(monday.Add(-2*time.Hour), send); n != 0 {
		t.Fatalf("not before nine: %d", n)
	}
	if n := srv0.SendSummaries(monday, send); n != 3 {
		t.Fatalf("the owner gets the week and the month, the quiet one only the month: %d %v", n, got)
	}
	var week, month push.Message
	for _, m := range got[u.ID] {
		if strings.Contains(m.Title, "week") {
			week = m
		} else {
			month = m
		}
	}
	if week.Body != "3 h over 2 days. Tap to see your week." || week.URL != "/#/stats" {
		t.Fatalf("the week: %+v", week)
	}
	if month.Title != "Your October Wrapped is ready" || month.Body != "6 h over 3 days. Tap to play." || month.URL != "/#/stats?play=month:-1" {
		t.Fatalf("the month: %+v", month)
	}
	if len(got[quiet.ID]) != 1 || !strings.Contains(got[quiet.ID][0].Title, "October") || len(got[sis.ID]) != 0 {
		t.Fatalf("the quiet one only the month, the sister (no phone) nothing yet: %v", got)
	}
	if n := srv0.SendSummaries(monday.Add(time.Hour), send); n != 0 {
		t.Fatalf("each once: %d", n)
	}
	// The sister turns notifications on a day later: she still gets them (the week is one day old).
	_ = st.SavePushSub(store.PushSub{UserID: sis.ID, Endpoint: "https://push.example/sis", P256dh: "k", Auth: "a", Scope: "all", Device: "Phone"})
	if n := srv0.SendSummaries(monday.AddDate(0, 0, 1), send); n != 2 {
		t.Fatalf("late but within two days: %d", n)
	}
	// On 1 January the year that ended is summed up (December had no reading, so no monthly notice).
	got = map[int64][]push.Message{}
	if n := srv0.SendSummaries(time.Date(2027, 1, 1, 9, 30, 0, 0, time.UTC), send); n != 3 {
		t.Fatalf("the year, to the three with phone notifications on: %d %v", n, got)
	}
	if m := got[u.ID][0]; m.Title != "Your 2026 Wrapped is ready" || m.Body != "6 h over 3 days. Tap to play." || m.URL != "/#/stats?play=year:-1" {
		t.Fatalf("the year: %+v", m)
	}
}
