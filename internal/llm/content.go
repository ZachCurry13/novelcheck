package llm

import (
	"encoding/json"

	"github.com/zachcurry13/novelcheck/internal/content"
)

// contentFrom reads the "content" answer (a list of keys, or {"key": true})
// into known item keys, matching labels and common other names too. An older
// answer's "lgbtq_content": true counts as the "lgbtq" item.
func contentFrom(raw json.RawMessage, lgbtq bool) []string {
	keys := []string{}
	seen := map[string]bool{}
	add := func(s string) {
		if k := content.Normalize(s); k != "" && !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for _, s := range customFlagsFrom(raw) {
		add(s)
	}
	if lgbtq {
		add("lgbtq")
	}
	return keys
}
