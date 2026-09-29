package events

import (
	"regexp"
	"strings"
	"time"
)

var (
	titleTagRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	h1Re       = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	dayRe      = regexp.MustCompile(`(?i)\b(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*\.?\s+(\d{1,2})(?:st|nd|rd|th)?,?\s+(\d{4})\b`)
	titleSepRe = regexp.MustCompile(`\s+[|—–-]\s+`)
)

// Info guesses an event's name and day from its page's title or top
// heading: "Romance Day — October 2, 2026" gives "Romance Day" and
// "2026-10-02". Either may be "".
func Info(page string) (name, day string) {
	var heads []string
	for _, re := range []*regexp.Regexp{titleTagRe, h1Re} {
		if m := re.FindStringSubmatch(page); m != nil {
			if t := text(m[1]); t != "" {
				heads = append(heads, t)
			}
		}
	}
	for _, h := range heads {
		if d := dayRe.FindStringSubmatch(h); d != nil && day == "" {
			if t, err := time.Parse("Jan 2 2006", d[1]+" "+d[2]+" "+d[3]); err == nil {
				day = t.Format("2006-01-02")
			}
		}
		for _, part := range titleSepRe.Split(h, -1) {
			if part = strings.TrimSpace(dayRe.ReplaceAllString(part, "")); name == "" && len([]rune(part)) >= 3 {
				name = part
			}
		}
	}
	if r := []rune(name); len(r) > 100 {
		name = string(r[:100])
	}
	return name, day
}
