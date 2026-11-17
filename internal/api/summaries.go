package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/push"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Reading summaries: on each person's own phones, at 9 in the morning, a notice for the week that ended (Mondays),
// the month that ended (on the 1st) and the year that ended (1 January), with the hours and days read. A weekly
// one opens that week's report on Stats; a monthly or yearly one plays Wrapped. Nothing is sent for a period with
// no timed reading (from KOReader's statistics), to someone with no phone notifications on, or for a notice they
// turned off in Settings. The time zone is the one the admin's automatic-rating hours use (else UTC). A notice
// that was missed (the server was off) is still sent for two days after, then dropped.

const summaryHour = 9

// summaryLocation is the time zone the summaries are cut and sent in.
func (s *Server) summaryLocation() *time.Location {
	if name := s.Store.Setting(store.KeyAutoRateTZ); name != "" {
		if loc, err := time.LoadLocation(name); err == nil {
			return loc
		}
	}
	return time.UTC
}

// summaryPeriod is one notice that may be due: its key, the period it covers and the pushed words' name for it.
type summaryPeriod struct {
	kind     string // week, month or year
	key      string
	from, to time.Time
}

// summariesDue lists the notices due at now (in loc): the week that ended on Monday and the day after, the month
// that ended in the first three days of the next, the year that ended in the first week of January.
func summariesDue(now time.Time, loc *time.Location) []summaryPeriod {
	t := now.In(loc)
	if t.Hour() < summaryHour {
		return nil
	}
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	var out []summaryPeriod
	if wd := t.Weekday(); wd == time.Monday || wd == time.Tuesday {
		monday := day.AddDate(0, 0, -((int(wd) + 6) % 7)) // this week's Monday
		from := monday.AddDate(0, 0, -7)
		out = append(out, summaryPeriod{"week", "w:" + from.Format("2006-01-02"), from, monday})
	}
	if t.Day() <= 3 {
		first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
		from := first.AddDate(0, -1, 0)
		out = append(out, summaryPeriod{"month", "m:" + from.Format("2006-01"), from, first})
	}
	if t.Month() == time.January && t.Day() <= 7 {
		first := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, loc)
		from := first.AddDate(-1, 0, 0)
		out = append(out, summaryPeriod{"year", fmt.Sprintf("y:%d", from.Year()), from, first})
	}
	return out
}

func hoursAndMinutes(sec int64) string {
	m := (sec + 30) / 60
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	if m%60 == 0 || m >= 600 {
		return fmt.Sprintf("%d h", m/60)
	}
	return fmt.Sprintf("%d h %02d m", m/60, m%60)
}

func plural(n int, one string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %ss", n, one)
}

// summaryMessage is the notice for a period of reading.
func summaryMessage(p summaryPeriod, seconds int64, days int) push.Message {
	read := fmt.Sprintf("%s over %s", hoursAndMinutes(seconds), plural(days, "day"))
	switch p.kind {
	case "week":
		return push.Message{Title: "Your week in reading", Body: read + ". Tap to see your week.", URL: "/#/stats", Tag: "summary"}
	case "month":
		return push.Message{Title: fmt.Sprintf("Your %s Wrapped is ready", p.from.Format("January")), Body: read + ". Tap to play.", URL: "/#/stats?play=month:-1", Tag: "summary"}
	}
	return push.Message{Title: fmt.Sprintf("Your %d Wrapped is ready", p.from.Year()), Body: read + ". Tap to play.", URL: "/#/stats?play=year:-1", Tag: "summary"}
}

// SendSummaries sends the notices due at now. send delivers one (the server's push service; a test's stand-in).
func (s *Server) SendSummaries(now time.Time, send func(userID int64, m push.Message)) int {
	loc := s.summaryLocation()
	due := summariesDue(now, loc)
	if len(due) == 0 {
		return 0
	}
	sent := 0
	for _, uid := range s.Store.SummaryUsers() {
		prefs := s.Store.SummaryPrefsOf(uid)
		for _, p := range due {
			if s.Store.SummarySent(uid, p.key) {
				continue
			}
			if (p.kind == "week" && !prefs.Week) || (p.kind == "month" && !prefs.Month) || (p.kind == "year" && !prefs.Year) {
				continue
			}
			seconds, _, days := s.Store.ReadingBetween(uid, p.from, p.to, loc)
			if seconds < 60 { // nothing to sum up for that period
				s.Store.MarkSummarySent(uid, p.key)
				continue
			}
			if subs, _ := s.Store.UserPushSubs(uid); len(subs) == 0 {
				continue // no phone notifications on yet: asked again at the next look, until the period is two days old
			}
			send(uid, summaryMessage(p, seconds, days))
			s.Store.MarkSummarySent(uid, p.key)
			sent++
		}
	}
	return sent
}

// SummaryLoop looks every fifteen minutes whether a summary is due.
func (s *Server) SummaryLoop(ctx context.Context) {
	t := time.NewTicker(15 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s.Push != nil {
				s.SendSummaries(time.Now(), s.Push.ToUser)
			}
		}
	}
}

// handleSummaryPrefs: GET and PUT /api/me/summaries {week, month, year}.
func (s *Server) handleSummaryPrefs(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	if r.Method == http.MethodPut {
		var p store.SummaryPrefs
		if !readJSON(w, r, &p, 1<<10) {
			return
		}
		if err := s.Store.SetSummaryPrefs(u.ID, p); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, s.Store.SummaryPrefsOf(u.ID))
}
