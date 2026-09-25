package deepread

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/epub"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// ErrNoEPUB means there's no EPUB copy of the book in the Calibre library.
var ErrNoEPUB = errors.New("Deep Scan needs an EPUB copy of this book in your Calibre library (Calibre can convert other formats to EPUB)")

// Runner reads approved Deep Scans one at a time, alongside normal rating.
type Runner struct {
	Store      *store.Store
	CalibreDir string // the read-only mount; EPUBs must be inside it
	wake       chan struct{}
}

func New(st *store.Store, calibreDir string) *Runner {
	return &Runner{Store: st, CalibreDir: calibreDir, wake: make(chan struct{}, 1)}
}

// Wake starts the next scan now instead of at the next check.
func (r *Runner) Wake() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// Run works through the queue until ctx ends. Every couple of minutes it
// also adds new Up Next books of the users chosen for automatic scans.
func (r *Runner) Run(ctx context.Context) {
	r.Store.ResumeDeepReads()
	for ctx.Err() == nil { // on shutdown a scan stops mid-way and resumes after the restart
		_ = safe.Run("queueing automatic Deep Scans", func() error { r.autoQueue(); return nil })
		if d, ok := r.Store.NextDeepRead(); ok {
			if err := safe.Run("Deep Scan of "+d.Title, func() error { return r.scan(ctx, d) }); err != nil {
				log.Printf("deep scan %d: %v", d.ID, err)
				_ = r.Store.FinishDeepRead(d.ID, "error", "", err.Error(), nil)
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-r.wake:
		case <-time.After(2 * time.Minute):
		}
	}
}

// EPUBFor finds the book's EPUB inside the Calibre library.
func (r *Runner) EPUBFor(bookID int64) (string, bool) {
	copies, err := r.Store.BookCopies(bookID)
	if err != nil {
		return "", false
	}
	for _, c := range copies {
		if c.Source != "calibre" || !strings.EqualFold(c.Format, "epub") || !inside(r.CalibreDir, c.Path) {
			continue
		}
		if st, err := os.Stat(c.Path); err == nil && st.Mode().IsRegular() {
			return c.Path, true
		}
	}
	return "", false
}

func inside(dir, p string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(p))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// partSize picks words per part for the AI that will do the reading.
func (r *Runner) partSize() int {
	if ais := r.Store.AIConfigs(); len(ais) > 0 && ais[0].Provider != "anthropic" && llm.IsLocal(ais[0].BaseURL) {
		return LocalWordsPerPart
	}
	return CloudWordsPerPart
}

// Prepare reads the book and returns its parts and what they should cost.
func (r *Runner) Prepare(bookID int64) ([]Part, Estimate, error) {
	path, ok := r.EPUBFor(bookID)
	if !ok {
		return nil, Estimate{}, ErrNoEPUB
	}
	sections, err := epub.Read(path)
	if err != nil {
		return nil, Estimate{}, err
	}
	parts := Split(sections, r.partSize())
	return parts, EstimateParts(parts), nil
}

// Queue adds approved Deep Scans for these books (skipping ones without an
// EPUB or already open) and returns how many were added plus their estimate.
func (r *Runner) Queue(bookIDs []int64, source, by string) (int, Estimate) {
	var total Estimate
	n := 0
	for _, id := range bookIDs {
		_, e, err := r.Prepare(id)
		if err != nil {
			continue
		}
		if _, err := r.Store.RequestDeepRead(id, source, by, "", true, e.Words, e.Parts, e.Tokens); err != nil {
			continue
		}
		total = add(total, e)
		n++
	}
	if n > 0 {
		r.Wake()
	}
	return n, total
}

// NextInQueues picks up to n Up Next books that can be scanned (have an
// EPUB, never scanned), in reading-list order, with their total estimate.
func (r *Runner) NextInQueues(n int) ([]int64, Estimate, error) {
	ids, err := r.Store.DeepCandidates(nil, n*4) // some may lack an EPUB
	if err != nil {
		return nil, Estimate{}, err
	}
	var picked []int64
	var total Estimate
	for _, id := range ids {
		if len(picked) == n {
			break
		}
		if _, e, err := r.Prepare(id); err == nil {
			picked = append(picked, id)
			total = add(total, e)
		}
	}
	return picked, total, nil
}

// autoQueue scans new Up Next books of the users chosen in Admin.
func (r *Runner) autoQueue() {
	users := DeepUsers(r.Store.Setting(store.KeyDeepUsers))
	if len(users) == 0 {
		return
	}
	ids, err := r.Store.DeepCandidates(users, 10)
	if err != nil || len(ids) == 0 {
		return
	}
	r.Queue(ids, "auto", "automatic")
}

// DeepUsers reads the "deep_scan_users" setting (up to 3 user ids).
func DeepUsers(setting string) []int64 {
	var ids []int64
	for _, f := range strings.Split(setting, ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(f), 10, 64); err == nil && id > 0 && len(ids) < store.MaxDeepUsers {
			ids = append(ids, id)
		}
	}
	return ids
}

func add(a, b Estimate) Estimate {
	return Estimate{Words: a.Words + b.Words, Parts: a.Parts + b.Parts, Tokens: a.Tokens + b.Tokens, Output: a.Output + b.Output}
}
