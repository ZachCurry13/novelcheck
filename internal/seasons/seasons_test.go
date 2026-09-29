package seasons

import (
	"testing"
	"time"
)

func TestEasterAndAdvent(t *testing.T) {
	for year, want := range map[int]string{2024: "2024-03-31", 2025: "2025-04-20", 2026: "2026-04-05", 2027: "2027-03-28"} {
		if got := Easter(year).Format("2006-01-02"); got != want {
			t.Fatalf("Easter %d: %s, want %s", year, got, want)
		}
	}
	for year, want := range map[int]string{2025: "2025-11-30", 2026: "2026-11-29", 2028: "2028-12-03"} {
		if got := AdventSunday(year).Format("2006-01-02"); got != want {
			t.Fatalf("Advent %d: %s, want %s", year, got, want)
		}
	}
}

func TestInSeason(t *testing.T) {
	at := func(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }
	keys := func(d string) string {
		var out string
		for _, s := range Current(at(d)) {
			out += s.Key + " "
		}
		return out
	}
	for d, want := range map[string]string{
		"2026-10-15": "halloween fall ",
		"2026-12-10": "advent christmas winter ",
		"2027-01-03": "christmas winter ",   // Christmas runs over New Year, to Epiphany
		"2026-02-20": "winter lent_easter ", // Ash Wednesday 2026 is February 18
		"2026-02-14": "winter valentines ",
		"2026-07-04": "summer ",
		"2026-09-05": "school ",
		"2026-11-01": "fall saints ",
	} {
		if got := keys(d); got != want {
			t.Fatalf("%s: %q, want %q", d, got, want)
		}
	}
}

func TestWordsCond(t *testing.T) {
	s, _ := Find("valentines")
	cond, args := s.WordsCond()
	if len(args) != len(s.Words) || args[0] != "% valentine%" || s.MaxSpice != 2 || cond == "" {
		t.Fatalf("%s %v", cond, args)
	}
}
