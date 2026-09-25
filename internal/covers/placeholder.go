package covers

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"strings"
)

// Placeholder draws a plain cover (SVG) with the title and author, in a
// colour picked from the title so books don't all look alike.
func Placeholder(title, author string) []byte {
	h := fnv.New32a()
	h.Write([]byte(title))
	hue := h.Sum32() % 360
	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 300" width="200" height="300">`+
		`<rect width="200" height="300" fill="hsl(%d,35%%,24%%)"/><rect x="12" y="12" width="176" height="276" fill="none" stroke="hsl(%d,40%%,55%%)" stroke-width="2"/>`+
		`<text x="100" y="%d" fill="#f1f5f9" font-family="Georgia,serif" font-size="19" text-anchor="middle">`, hue, hue, 120-9*min(len(wrap(title, 15, 5)), 5))
	for i, line := range wrap(title, 15, 5) {
		fmt.Fprintf(&b, `<tspan x="100" dy="%d">%s</tspan>`, map[bool]int{true: 0, false: 24}[i == 0], esc(line))
	}
	b.WriteString(`</text>`)
	for i, line := range wrap(author, 22, 2) {
		fmt.Fprintf(&b, `<text x="100" y="%d" fill="#cbd5e1" font-family="system-ui,sans-serif" font-size="13" text-anchor="middle">%s</text>`, 250+i*17, esc(line))
	}
	b.WriteString(`</svg>`)
	return b.Bytes()
}

// wrap breaks s into at most n lines of about width characters.
func wrap(s string, width, n int) []string {
	var lines []string
	cur := ""
	for _, w := range strings.Fields(s) {
		switch {
		case cur == "":
			cur = w
		case len([]rune(cur))+1+len([]rune(w)) <= width:
			cur += " " + w
		default:
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) > n {
		lines = lines[:n]
		lines[n-1] += "…"
	}
	return lines
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
