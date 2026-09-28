package discover

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Settings the service keeps for itself.
const (
	keyRatedDay   = "discover_rated_day"   // the day ratings were last queued (2006-01-02)
	keyRatedCount = "discover_rated_count" // how many that day
)

// ErrBusy means a refresh is already running.
var ErrBusy = errors.New("Discover is already refreshing its lists")

// Service refreshes the lists daily and hands new books to the rating worker.
type Service struct {
	Store  *store.Store
	Client *Client
	Queue  func(ids ...int64) // the rating worker's queue
	Pause  time.Duration      // between NYT calls (they allow 5 a minute)

	mu      sync.Mutex
	running bool
}

func New(st *store.Store, queue func(ids ...int64)) *Service {
	return &Service{Store: st, Client: NewClient(), Queue: queue, Pause: 13 * time.Second}
}

// Due reports whether the lists are more than a day old.
func (s *Service) Due(now time.Time) bool {
	last, err := time.Parse(time.RFC3339, s.Store.Setting(store.KeyDiscoverLastRefresh))
	return err != nil || now.Sub(last) >= 23*time.Hour
}

// Running reports whether a refresh is under way.
func (s *Service) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Refresh fetches every list, queues today's ratings and returns a summary.
func (s *Service) Refresh(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return "", ErrBusy
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	var problems []string
	books, lists := 0, 0
	save := func(list string, entries []store.DiscoverEntry, err error) {
		if err == nil && len(entries) > 0 {
			var n int
			if n, err = s.Store.SaveDiscoverList(list, entries); n > 0 {
				books, lists = books+n, lists+1
			}
		}
		if err != nil {
			problems = append(problems, err.Error())
		}
	}
	nytOK := false
	if key := strings.TrimSpace(s.Store.Setting(store.KeyNYTAPIKey)); key != "" {
		for i, list := range nytLists {
			if i > 0 && !sleep(ctx, s.Pause) {
				return "", ctx.Err()
			}
			entries, err := s.Client.NYTList(ctx, key, strings.TrimPrefix(list, "nyt:"))
			save(list, entries, err)
			nytOK = nytOK || err == nil && len(entries) > 0
			if errors.Is(err, ErrNYTKey) {
				s.Store.Notify("warning", "discover", err.Error(), "#/admin")
				break
			}
		}
	}
	if nytOK {
		for _, l := range fallbackLists {
			_ = s.Store.ClearDiscoverList(l)
		}
		s.Store.Resolve("discover")
	} else {
		trending, err := s.Client.OLTrending(ctx, perRow)
		save(listTrending, trending, err)
		for _, l := range []string{listTeenOL, listKidsOL} {
			entries, err := s.Client.OLSubject(ctx, strings.TrimPrefix(l, "ol:"), perRow)
			save(l, entries, err)
		}
	}
	classics, err := s.Client.OLSubject(ctx, "classics", 30)
	save(listClassics, classics, err)

	queued := s.queueRatings(time.Now())
	summary := fmt.Sprintf("%d books on %d lists; %d sent to be rated", books, lists, queued)
	if len(problems) > 0 {
		summary += ". Problems: " + strings.Join(problems, "; ")
	}
	_ = s.Store.SetSetting(store.KeyDiscoverLastRefresh, time.Now().UTC().Format(time.RFC3339))
	_ = s.Store.SetSetting(store.KeyDiscoverLastResult, summary)
	if books == 0 && len(problems) > 0 {
		return summary, errors.New(problems[0])
	}
	return summary, nil
}

// queueRatings sends up to the daily allowance of unrated listed books to
// the rating worker (best ranked first) and returns how many it sent.
func (s *Service) queueRatings(now time.Time) int {
	day := now.Format("2006-01-02")
	done := 0
	if s.Store.Setting(keyRatedDay) == day {
		done = s.Store.SettingInt(keyRatedCount)
	}
	ids, err := s.Store.QueueDiscoverRatings(s.Store.SettingInt(store.KeyDiscoverDaily) - done)
	if err != nil || len(ids) == 0 {
		return 0
	}
	_ = s.Store.SetSetting(keyRatedDay, day)
	_ = s.Store.SetSetting(keyRatedCount, strconv.Itoa(done+len(ids)))
	if s.Queue != nil {
		s.Queue(ids...)
	}
	return len(ids)
}

// Test checks a New York Times key with one list and says how many books it has.
func (s *Service) Test(ctx context.Context, key string) (int, error) {
	entries, err := s.Client.NYTList(ctx, strings.TrimSpace(key), strings.TrimPrefix(listTeen, "nyt:"))
	return len(entries), err
}

// Loop refreshes the lists once a day while Discover is switched on.
func (s *Service) Loop(ctx context.Context) {
	if !sleep(ctx, 2*time.Minute) { // let start-up (and a Calibre sync) settle first
		return
	}
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		if s.Store.SettingBool(store.KeyModuleDiscover) && s.Due(time.Now()) {
			_ = safe.Run("discover refresh", func() error {
				summary, err := s.Refresh(ctx)
				if err != nil && !errors.Is(err, ErrBusy) {
					log.Printf("discover: %v", err)
				} else if summary != "" {
					log.Printf("discover: %s", summary)
				}
				return nil
			})
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// sleep waits d, or returns false when ctx ends first.
func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
