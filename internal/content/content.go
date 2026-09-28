// Package content is the catalog of detailed content items (language,
// violence, gore, substances and other themes) the AI marks on each book, so
// families can hide exactly what they care about. The romance scale (peppers),
// the spiritual flags and the family's custom filters live elsewhere.
package content

import "strings"

// Version goes up when items are added or their meaning changes, so books
// checked under an older list are offered for a new check. The same check
// writes the card blurb (books.premise): 2 = the blurb was added (v1.20).
const Version = 2

// Item is one thing a book can contain, e.g. "Gun violence".
type Item struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"` // what the AI looks for; "" = the label says it
}

// Group is a family of items with an icon, e.g. ⚔️ Violence.
type Group struct {
	Key   string `json:"key"`
	Icon  string `json:"icon"`
	Label string `json:"label"`
	Items []Item `json:"items"`
}

// Groups is the catalog, in display order.
var Groups = []Group{
	{"language", "🗣️", "Language", []Item{
		{"profanity", "Profanity", "mild swearing such as damn or hell"},
		{"strong_profanity", "Strong profanity / F-word", "the F-word or equally strong swearing"},
		{"blasphemy", "Religious profanity / blasphemy", "God's or Jesus' name used as a curse, or mocking the sacred"},
		{"sexual_language", "Sexual language", "crude sexual words or talk"},
		{"crude_humor", "Crude / vulgar humor", "gross-out, bathroom or vulgar jokes"},
		{"slurs", "Slurs", "racial, ethnic or other hateful slurs"},
		{"insults", "Insults / name-calling", "mean insults or name-calling"},
	}},
	{"violence", "⚔️", "Violence", []Item{
		{"fights", "Physical fights", "fistfights, brawls or beatings"},
		{"weapons", "Weapons", "swords, knives, bows or guns used against people or creatures"},
		{"gun_violence", "Gun violence", "people shot or shot at"},
		{"stabbing", "Stabbing", ""},
		{"murder", "Murder", "a character is deliberately killed"},
		{"war", "War / battles", ""},
		{"torture", "Torture", ""},
		{"domestic_violence", "Domestic violence", "violence at home or between partners"},
		{"sexual_violence", "Sexual violence / assault", "rape or sexual assault, attempted or off the page included"},
		{"child_violence", "Violence against children", ""},
		{"animal_harm", "Animal violence / death", "animals hurt or killed, a pet dying included"},
	}},
	{"gore", "🩸", "Gore", []Item{
		{"blood", "Blood", ""},
		{"graphic_injuries", "Graphic injuries", "wounds described in detail"},
		{"graphic_deaths", "Graphic deaths", "deaths described in gory detail"},
		{"dismemberment", "Dismemberment", ""},
		{"mutilation", "Mutilation", ""},
		{"body_horror", "Body horror", "bodies twisted, infested or transformed in disturbing ways"},
		{"organs", "Organs / bodily contents", ""},
		{"corpses", "Corpses", "dead bodies described"},
	}},
	{"substances", "🍺", "Substance Use", []Item{
		{"alcohol", "Alcohol use", "characters drink alcohol"},
		{"underage_drinking", "Underage drinking", ""},
		{"tobacco", "Tobacco / smoking", ""},
		{"vaping", "Vaping", ""},
		{"marijuana", "Marijuana", ""},
		{"drugs", "Recreational / illegal drugs", ""},
		{"prescription_misuse", "Prescription drug misuse", ""},
		{"drug_dealing", "Drug dealing", ""},
		{"addiction", "Addiction", ""},
		{"overdose", "Overdose / withdrawal", ""},
	}},
	{"other", "🧩", "Other Content", []Item{
		{"self_harm", "Suicide / self-harm", "suicide, suicidal thoughts or self-harm"},
		{"eating_disorders", "Eating disorders", ""},
		{"abuse", "Abuse", "emotional, physical or sexual abuse, or neglect"},
		{"bullying", "Bullying", ""},
		{"death_grief", "Death / grief", "a loved one dies, grieving"},
		{"religious_themes", "Religious themes", "faith, prayer or religion as a theme"},
		{"lgbtq", "LGBTQ+ themes", "LGBTQ+ characters or relationships"},
		{"pregnancy", "Pregnancy / childbirth", ""},
		{"mental_health", "Mental health themes", "depression, anxiety, trauma or other mental illness"},
	}},
}

var (
	items   = map[string]string{} // item key -> group key
	byLabel = map[string]string{} // squashed label -> item key
)

// aliases are other names models use for an item.
var aliases = map[string]string{
	"sexual_assault": "sexual_violence", "rape": "sexual_violence", "suicide": "self_harm",
	"illegal_drugs": "drugs", "recreational_drugs": "drugs", "war_battles": "war", "battles": "war",
	"f_word": "strong_profanity", "swearing": "profanity", "smoking": "tobacco", "lgbtq_themes": "lgbtq",
	"lgbtq_content": "lgbtq", "animal_death": "animal_harm", "grief": "death_grief",
}

func init() {
	for _, g := range Groups {
		for _, it := range g.Items {
			items[it.Key] = g.Key
			byLabel[squash(it.Label)] = it.Key
		}
	}
}

// squash keeps only letters and digits, lower-cased.
func squash(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Normalize maps what a model wrote ("War / battles", "gun-violence",
// "sexual assault") to an item key, or "" when it isn't one.
func Normalize(s string) string {
	k := strings.Trim(strings.NewReplacer(" ", "_", "-", "_", "/", "_").Replace(strings.ToLower(strings.TrimSpace(s))), "_")
	for strings.Contains(k, "__") {
		k = strings.ReplaceAll(k, "__", "_")
	}
	if _, ok := items[k]; ok {
		return k
	}
	if a, ok := aliases[k]; ok {
		return a
	}
	return byLabel[squash(s)]
}

// GroupOf returns an item's group key ("" for unknown items).
func GroupOf(item string) string { return items[item] }

// ItemsOf returns the item keys of a group.
func ItemsOf(group string) []string {
	for _, g := range Groups {
		if g.Key == group {
			keys := make([]string, len(g.Items))
			for i, it := range g.Items {
				keys[i] = it.Key
			}
			return keys
		}
	}
	return nil
}

// Label returns an item's or group's label ("" when unknown).
func Label(key string) string {
	for _, g := range Groups {
		if g.Key == key {
			return g.Label
		}
		for _, it := range g.Items {
			if it.Key == key {
				return it.Label
			}
		}
	}
	return ""
}
