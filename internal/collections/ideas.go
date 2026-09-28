package collections

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/genres"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// About once a week, when background AI work may run, the AI looks at the
// family's library and proposes up to 3 themed collections, each already
// filled, for a parent to keep or drop. On by default with a local AI.

const (
	ideasEvery   = 7 * 24 * time.Hour
	maxIdeas     = 3 // waiting ideas; no new ones until a parent decides
	minSample    = 30
	minIdeaBooks = 5
)

// IdeasOn reports whether weekly ideas are on.
func (s *Service) IdeasOn() bool {
	switch s.Store.Setting(store.KeyCollectionIdeas) {
	case "on":
		return true
	case "off":
		return false
	}
	return s.Local != nil && s.Local()
}

func (s *Service) ideasDue(now time.Time) bool {
	if !s.IdeasOn() || (s.Allowed != nil && !s.Allowed()) || s.Store.PendingIdeas() >= maxIdeas {
		return false
	}
	last, err := time.Parse(time.RFC3339, s.Store.Setting(store.KeyCollectionIdeasLast))
	return err != nil || now.Sub(last) >= ideasEvery
}

// Loop checks every hour (the first time ten minutes after start).
func (s *Service) Loop(ctx context.Context) {
	wait := 10 * time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = time.Hour
		if s.ideasDue(time.Now()) {
			if err := safe.Run("collection ideas", func() error { return s.MakeIdeas(ctx) }); err != nil {
				log.Printf("collection ideas: %v", err)
			}
		}
	}
}

// MakeIdeas asks the AI for new ideas now and fills each; it says how many
// were saved through the notice.
func (s *Service) MakeIdeas(ctx context.Context) error {
	_ = s.Store.SetSetting(store.KeyCollectionIdeasLast, time.Now().UTC().Format(time.RFC3339))
	counts, sample, err := s.Store.LibrarySample(60)
	if err != nil || len(sample) < minSample {
		return err // a small library gets no ideas yet
	}
	var got struct {
		Ideas []struct {
			Name  string `json:"name"`
			Icon  string `json:"icon"`
			Theme string `json:"theme"`
		} `json:"ideas"`
	}
	if _, err := llm.Ask(ctx, s.Store, s.Store.AIConfigs(), 0, ideasSystem(s.Store.CollectionNames()), librarySummary(counts, sample),
		func(out string) error { return decode(out, &got) }); err != nil {
		return err
	}
	var made []string
	for _, idea := range got.Ideas {
		if s.Store.PendingIdeas() >= maxIdeas {
			break
		}
		name := strings.TrimSpace(idea.Name)
		if name == "" || strings.TrimSpace(idea.Theme) == "" {
			continue
		}
		picks, err := Fill(ctx, s.Store, idea.Theme, nil)
		if err != nil {
			return err
		}
		if len(picks) < minIdeaBooks {
			continue
		}
		icon := strings.TrimSpace(idea.Icon)
		if icon == "" || len([]rune(icon)) > 4 {
			icon = "💡"
		}
		id, err := s.Store.CreateCollection(store.NewCollection{Name: name, Icon: icon, Kind: "idea", Theme: idea.Theme, By: "AI"})
		if err != nil {
			return err
		}
		ids, reasons := split(picks)
		if _, err := s.Store.AddToCollection(id, ids, reasons); err != nil {
			return err
		}
		made = append(made, name)
	}
	if len(made) > 0 {
		s.Store.NotifyRoutine("collection-ideas", fmt.Sprintf("💡 New collection ideas: %s. Keep the ones you like on the Collections page.",
			strings.Join(made, ", ")), "#/collections")
	}
	return nil
}

// split turns picks into book ids and the AI's reasons.
func split(picks []Pick) ([]int64, map[int64]string) {
	ids := make([]int64, 0, len(picks))
	reasons := map[int64]string{}
	for _, p := range picks {
		ids = append(ids, p.Book.ID)
		reasons[p.Book.ID] = p.Reason
	}
	return ids, reasons
}

func ideasSystem(existing []string) string {
	return `You suggest themed book collections for a family's home library (parents and kids).
Suggest ` + fmt.Sprint(maxIdeas) + ` varied, family-friendly themes this library can fill with at least five books each:
a genre, a setting, a subject, an age group or a mood (for example "Dragon adventures for middle grade" or "Cozy mysteries").
Don't repeat these existing collections: ` + strings.Join(existing, "; ") + `.
Reply with JSON only: {"ideas": [{"name": "a short name", "icon": "one emoji", "theme": "one sentence on which books fit"}]}`
}

// librarySummary is the library as the ideas see it: genre counts and a
// sample of titles.
func librarySummary(counts map[string]int, sample []store.Book) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return counts[keys[i]] > counts[keys[j]] })
	var b strings.Builder
	b.WriteString("Genres in the library:")
	for _, k := range keys {
		fmt.Fprintf(&b, " %s %d;", genres.Label(k), counts[k])
	}
	b.WriteString("\n\nSome of the books:\n")
	for _, bk := range sample {
		fmt.Fprintf(&b, "%s | %s | %s\n", bk.Title, bk.Author, strings.Trim(bk.Genres, ","))
	}
	return b.String()
}
