package suggest

// Description similarity: TF-IDF word vectors compared by cosine, so two
// books score for the unusual words they share ("dragon", "regency",
// "heist"), not for "the" or "novel".

import (
	"math"
	"regexp"
	"strings"
)

var wordRE = regexp.MustCompile(`\p{L}{3,}`)

var stop = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`the and for but not are was were has had have with from this that these those
		they them their there then than what when where which while who whom whose will would can could should must
		into onto over under about after before again also just only even ever very more most much many some such each
		every other another any all both few own same his her hers him she its our ours your yours you out off down
		upon once here how why because until against between through during above below one two three first second
		new york times bestselling bestseller author authors book books novel novels story stories series edition
		read reader readers reading page pages award winning praise acclaimed life world year years now never always
		way make makes made take takes took find finds found back still well like know knows known time own`) {
		stop[w] = true
	}
}

type vector map[string]float64

// words counts the meaningful words in s (first 400 or so).
func words(s string) map[string]int {
	out := map[string]int{}
	for _, w := range wordRE.FindAllString(strings.ToLower(s), 400) {
		if !stop[w] {
			out[stem(w)]++
		}
	}
	return out
}

// stem folds simple plurals together ("dragons", "riders", "stories").
func stem(w string) string {
	switch {
	case len(w) > 4 && strings.HasSuffix(w, "ies"):
		return w[:len(w)-3] + "y"
	case len(w) > 4 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") && !strings.HasSuffix(w, "us") && !strings.HasSuffix(w, "is"):
		return w[:len(w)-1]
	}
	return w
}

// corpus holds how many books use each word, for weighting.
type corpus struct {
	df map[string]int
	n  int
}

func newCorpus(docs []string) corpus {
	c := corpus{df: map[string]int{}, n: len(docs)}
	for _, d := range docs {
		for w := range words(d) {
			c.df[w]++
		}
	}
	return c
}

// vector is s as unit-length TF-IDF weights.
func (c corpus) vector(s string) vector {
	v := vector{}
	var norm float64
	for w, tf := range words(s) {
		df := max(c.df[w], 1)
		x := (1 + math.Log(float64(tf))) * math.Log(1+float64(c.n)/float64(df))
		v[w] = x
		norm += x * x
	}
	if norm == 0 {
		return v
	}
	norm = math.Sqrt(norm)
	for w := range v {
		v[w] /= norm
	}
	return v
}

func cosine(a, b vector) float64 {
	if len(a) > len(b) {
		a, b = b, a
	}
	var dot float64
	for w, x := range a {
		dot += x * b[w]
	}
	return dot
}
