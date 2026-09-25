// Package genres maps Calibre's tags onto a short list of categories people
// browse by, and tells fiction from nonfiction. Tags are matched on whole
// words, so "Science Fiction" isn't "Science" and "Universe" isn't "Verse".
package genres

import (
	"regexp"
	"strings"
)

// Genre is one category in the library filters.
type Genre struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	kind  int    // 1 fiction, -1 nonfiction, 0 either (an audience or form)
	match *regexp.Regexp
	not   *regexp.Regexp // tags that look like this genre but aren't
}

func g(key, label string, kind int, words, not string) Genre {
	re := func(w string) *regexp.Regexp {
		if w == "" {
			return nil
		}
		return regexp.MustCompile(`(?i)\b(?:` + w + `)(?:s|es)?\b`)
	}
	return Genre{Key: key, Label: label, kind: kind, match: re(words), not: re(not)}
}

// All lists the categories in the order the filters show them.
var All = []Genre{
	g("fantasy", "Fantasy", 1, `fantasy|fantasies|magic|dragon|fairy ?tale|sword and sorcery|mythology|mythic`, ""),
	g("scifi", "Science Fiction", 1, `science fiction|sci-?fi|sf|space opera|dystopia|dystopian|cyberpunk|time travel|post-apocalyptic|alternate history|alternative history`, ""),
	g("romance", "Romance", 1, `romance|romantic|love stor(?:y|ies)|romcom|rom-com`, ""),
	g("mystery", "Mystery & Thriller", 1, `myster(?:y|ies)|thriller|suspense|crime|detective|whodunn?it|cozy|cosy|espionage|spy`, `true crime`),
	g("horror", "Horror", 1, `horror|ghost|zombie|haunted|haunting`, ""),
	g("historical", "Historical Fiction", 1, `historical`, ""),
	g("literary", "Literary & Contemporary", 1, `literary|contemporary|general fiction|women's fiction|family saga|coming of age`, ""),
	g("classics", "Classics", 1, `classic`, ""),
	g("adventure", "Adventure", 1, `adventure|action`, ""),
	g("humor", "Humor", 0, `humou?r|comedy|comedic|satire|funny`, ""),
	g("ya", "Young Adult", 0, `young adult|ya|teen|teenage|teens`, ""),
	g("children", "Children's", 0, `children|children's|juvenile|middle grade|picture book|kids`, ""),
	g("graphic", "Comics & Graphic Novels", 0, `comic|graphic novel|manga`, ""),
	g("poetry", "Poetry", 0, `poetry|poem|verse`, ""),
	g("religion", "Religion & Spirituality", 0, `religion|religious|christian|christianity|faith|spiritual|spirituality|bible|theology|church|prayer|devotional`, ""),
	g("biography", "Biography & Memoir", -1, `biograph(?:y|ies)|memoir|autobiograph(?:y|ies)`, ""),
	g("history", "History", -1, `history|histories`, `alternate history|alternative history`),
	g("truecrime", "True Crime", -1, `true crime`, ""),
	g("selfhelp", "Self-help & Wellbeing", -1, `self-?help|personal development|personal growth|motivational|psychology|health|wellness|parenting|mindfulness`, ""),
	g("science", "Science & Nature", -1, `science|nature|physics|biology|astronomy|mathematics|chemistry|environment`, `science fiction`),
	g("business", "Business & Money", -1, `business|economics|finance|money|investing|management|leadership|entrepreneurship`, ""),
	g("cooking", "Cooking & Food", -1, `cooking|cookbook|cookery|recipe|baking|food`, ""),
}

var (
	nonfictionRE = regexp.MustCompile(`(?i)\bnon[- ]?fiction\b`)
	fictionRE    = regexp.MustCompile(`(?i)\bfiction\b`)
	byKey        = map[string]Genre{}
)

func init() {
	for _, x := range All {
		byKey[x.Key] = x
	}
}

// Valid reports whether key is one of the categories.
func Valid(key string) bool { _, ok := byKey[key]; return ok }

// Label is a category's name ("" for an unknown key).
func Label(key string) string { return byKey[key].Label }

// FromTags returns the categories a book's tags point to (at most 4, in the
// order of All) and "fiction", "nonfiction" or "" when it can't tell.
func FromTags(tags []string) ([]string, string) {
	var keys []string
	for _, x := range All {
		for _, t := range tags {
			if x.match.MatchString(t) && (x.not == nil || !x.not.MatchString(t)) {
				keys = append(keys, x.Key)
				break
			}
		}
		if len(keys) == 4 {
			break
		}
	}
	return keys, Kind(tags, keys)
}

// Kind is "fiction" or "nonfiction" from the tags (a "Nonfiction" or
// "Fiction" tag wins), else from the categories, else "".
func Kind(tags, keys []string) string {
	fiction, non := false, false
	for _, t := range tags {
		if nonfictionRE.MatchString(t) {
			non = true
		} else if fictionRE.MatchString(t) {
			fiction = true
		}
	}
	if non != fiction {
		return map[bool]string{true: "nonfiction", false: "fiction"}[non]
	}
	score := 0
	for _, k := range keys {
		score += byKey[k].kind
	}
	switch {
	case score > 0:
		return "fiction"
	case score < 0:
		return "nonfiction"
	}
	return ""
}

// Join stores categories as ",fantasy,romance," so one can be matched with LIKE.
func Join(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	return "," + strings.Join(keys, ",") + ","
}

// Split undoes Join.
func Split(s string) []string {
	var out []string
	for _, k := range strings.Split(strings.Trim(s, ","), ",") {
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}
