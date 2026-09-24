package llm

import (
	"encoding/json"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// customFlagsSection asks the AI about the family's own filters (Admin →
// Custom AI filters) and adds "custom_flags" to the JSON it returns.
func customFlagsSection(flags []store.CustomFlag) string {
	if len(flags) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n4. Custom Flags (the family's own topics). Mark a flag true only if the book clearly contains it:\n")
	keys := make([]string, 0, len(flags))
	for _, f := range flags {
		b.WriteString("- " + f.Key + " (" + oneLine(f.Label) + ")")
		if d := oneLine(f.Description); d != "" {
			b.WriteString(": " + d)
		}
		b.WriteString("\n")
		keys = append(keys, `"`+f.Key+`": true | false`)
	}
	b.WriteString("Add this to the JSON object: \"custom_flags\": {" + strings.Join(keys, ", ") + "}")
	return b.String()
}

// oneLine keeps an admin's text from breaking the prompt's layout.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// customFlagsFrom reads {"swearing": true, "gore": "false"} (or a plain list
// like ["swearing"]) into the keys marked true. Anything else is ignored so a
// sloppy answer never costs the whole rating.
func customFlagsFrom(raw json.RawMessage) []string {
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	var on []string
	for k, v := range obj {
		switch strings.ToLower(strings.Trim(strings.TrimSpace(string(v)), `"`)) {
		case "true", "yes", "1":
			on = append(on, k)
		}
	}
	return on
}
