// Package seasons defines the seasonal shelves (Halloween, Advent, summer…):
// when each is in season, and the words that find its books in a title,
// Calibre tags or description. The church seasons follow Easter and Advent
// each year.
package seasons

import (
	"strings"
	"time"
)

// Season is one seasonal shelf.
type Season struct {
	Key   string `json:"key"`
	Icon  string `json:"icon"`
	Name  string `json:"name"`
	Theme string `json:"theme"` // what "Build with AI" asks for
	// MaxSpice caps peppers for the shelf (-1 = no cap): Valentine's is for
	// clean romance and friendship.
	MaxSpice int                                 `json:"-"`
	Words    []string                            `json:"-"`
	span     func(year int) (from, to time.Time) // inclusive days
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// fixed is a span of calendar days; one ending before it starts runs into
// the next year.
func fixed(m1 time.Month, d1 int, m2 time.Month, d2 int) func(int) (time.Time, time.Time) {
	return func(y int) (time.Time, time.Time) {
		from, to := day(y, m1, d1), day(y, m2, d2)
		if to.Before(from) {
			to = day(y+1, m2, d2)
		}
		return from, to
	}
}

// Easter is Easter Sunday of year (the Gregorian computus).
func Easter(year int) time.Time {
	a, b, c := year%19, year/100, year%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	return day(year, time.Month(month), (h+l-7*m+114)%31+1)
}

// AdventSunday is the first Sunday of Advent: the Sunday from November 27
// to December 3.
func AdventSunday(year int) time.Time {
	d := day(year, time.November, 27)
	for d.Weekday() != time.Sunday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

// All seasonal shelves, in the order they're offered.
var All = []Season{
	{Key: "halloween", Icon: "🎃", Name: "Halloween & spooky", Theme: "not-too-scary spooky and Halloween stories: ghosts, haunted houses, monsters, costumes and trick-or-treating",
		MaxSpice: -1, span: fixed(time.September, 20, time.October, 31),
		Words: []string{"halloween", "haunted", "haunting", "ghost", "spooky", "monster", "vampire", "zombie", "pumpkin", "skeleton", "graveyard", "trick-or-treat", "trick or treat", "goosebumps", "werewolf", "all hallows"}},
	{Key: "fall", Icon: "🍂", Name: "Fall & Thanksgiving", Theme: "cozy autumn and Thanksgiving books: harvest, falling leaves, gratitude and family gatherings",
		MaxSpice: -1, span: fixed(time.September, 22, time.November, 30),
		Words: []string{"autumn", "thanksgiving", "harvest", "scarecrow", "apple orchard", "fall leaves", "hayride", "corn maze", "pilgrim"}},
	{Key: "advent", Icon: "🕯️", Name: "Advent", Theme: "books for Advent: waiting for Christmas, the Nativity story, St. Nicholas and preparing our hearts",
		MaxSpice: -1, span: func(y int) (time.Time, time.Time) { return AdventSunday(y), day(y, time.December, 24) },
		Words: []string{"advent", "nativity", "jesse tree", "st. nicholas", "saint nicholas", "bethlehem", "manger"}},
	{Key: "christmas", Icon: "🎄", Name: "Christmas", Theme: "Christmas stories: the Nativity, Santa and St. Nicholas, family traditions, snow and giving",
		MaxSpice: -1, span: fixed(time.November, 20, time.January, 6),
		Words: []string{"christmas", "santa", "reindeer", "north pole", "nativity", "grinch", "yuletide", "mistletoe", "noel", "nutcracker", "gingerbread", "polar express", "elves"}},
	{Key: "winter", Icon: "❄️", Name: "Winter", Theme: "winter books: snow days, ice, cold adventures and cozy stories by the fire",
		MaxSpice: -1, span: fixed(time.December, 1, time.February, 28),
		Words: []string{"winter", "snowman", "snowflake", "snowy", "blizzard", "ice skating", "sled", "frozen", "arctic", "hot cocoa", "mittens"}},
	{Key: "valentines", Icon: "💝", Name: "Valentine's & friendship", Theme: "clean Valentine's books: friendship, kindness, family love and sweet (not spicy) romance",
		MaxSpice: 2, span: fixed(time.January, 25, time.February, 14),
		Words: []string{"valentine", "friendship", "best friend", "cupid", "love letter", "kindness"}},
	{Key: "lent_easter", Icon: "✝️", Name: "Lent & Easter", Theme: "books for Lent and Easter: the Passion and Resurrection of Jesus, sacrifice, forgiveness and new life",
		MaxSpice: -1, span: func(y int) (time.Time, time.Time) { e := Easter(y); return e.AddDate(0, 0, -46), e.AddDate(0, 0, 49) },
		Words: []string{"easter", "lent", "lenten", "resurrection", "holy week", "good friday", "stations of the cross", "palm sunday", "empty tomb", "crucifixion"}},
	{Key: "spring", Icon: "🌷", Name: "Spring", Theme: "spring books: gardens, flowers, baby animals, rain and new beginnings",
		MaxSpice: -1, span: fixed(time.March, 20, time.June, 20),
		Words: []string{"springtime", "garden", "blossom", "bloom", "butterfly", "butterflies", "baby animals", "tulip"}},
	{Key: "summer", Icon: "☀️", Name: "Summer", Theme: "summer reads: beaches, camp, vacations, road trips and outdoor adventures",
		MaxSpice: -1, span: fixed(time.June, 1, time.August, 31),
		Words: []string{"summer", "beach", "camping", "summer camp", "vacation", "ocean", "island", "road trip", "lemonade", "swimming"}},
	{Key: "school", Icon: "🎒", Name: "Back to school", Theme: "back-to-school books: first days, new friends, teachers and classrooms",
		MaxSpice: -1, span: fixed(time.August, 1, time.September, 15),
		Words: []string{"back to school", "first day of school", "new school", "classroom", "teacher", "kindergarten", "school year", "school bus"}},
	{Key: "saints", Icon: "👼", Name: "Saints & feast days", Theme: "lives of the saints, feast days, and stories of faith for Catholic families",
		MaxSpice: -1, span: fixed(time.October, 25, time.November, 8),
		Words: []string{"saint", "martyr", "our lady", "feast day", "patron saint", "canonized", "lives of the"}},
}

// Find returns the season with key.
func Find(key string) (Season, bool) {
	for _, s := range All {
		if s.Key == key {
			return s, true
		}
	}
	return Season{}, false
}

// In reports whether now falls in the season (in any year's span, since
// some run over New Year).
func (s Season) In(now time.Time) bool {
	t := day(now.Year(), now.Month(), now.Day())
	for y := now.Year() - 1; y <= now.Year(); y++ {
		from, to := s.span(y)
		if !t.Before(from) && !t.After(to) {
			return true
		}
	}
	return false
}

// Current lists the seasons in season at now, in the usual order.
func Current(now time.Time) []Season {
	var out []Season
	for _, s := range All {
		if s.In(now) {
			out = append(out, s)
		}
	}
	return out
}

// text is a book's searchable words, with a space on each side so a word
// can be matched at its start.
const text = `(' ' || LOWER(b.title || ' ' || b.tags || ' ' || b.premise || ' ' || b.blurb || ' ' || b.description) || ' ')`

// WordsCond is the SQL (over books b) for the season's books by its words.
// It reads every book's description, so the store keeps its matches
// (season_books) rather than running it on each visit.
func (s Season) WordsCond() (string, []any) {
	parts := make([]string, 0, len(s.Words))
	args := make([]any, 0, len(s.Words))
	for _, w := range s.Words {
		parts = append(parts, text+" LIKE ?")
		args = append(args, "% "+strings.ToLower(w)+"%")
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}
