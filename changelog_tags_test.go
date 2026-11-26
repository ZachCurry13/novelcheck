package novelcheck

import (
	"strings"
	"testing"
)

// The audience tag {admin} only ever ends a bullet line, so the page can cut those notes for people it does not concern.
func TestChangelogAudienceTagsEndTheLine(t *testing.T) {
	for _, line := range strings.Split(Changelog, "\n") {
		if i := strings.Index(line, "{admin}"); i >= 0 {
			if strings.TrimSpace(line[i:]) != "{admin}" || !(strings.HasPrefix(line, "- ") || (len(line) > 2 && line[0] >= '0' && line[0] <= '9')) {
				t.Errorf("a misplaced tag: %.80s", line)
			}
		}
	}
}
