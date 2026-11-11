package api

import (
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Calibre duplicates are found by title and author alone, which can't tell a second copy from another
// edition, an omnibus or a scan. markDoubts compares each extra entry's files with the copy that stays and
// notes anything that suggests it is not simply the same book again. An entry with a doubt is listed
// unticked, so it is only removed when the owner ticks it on purpose.
func markDoubts(entries []store.DupEntry, keep string) {
	var kept *store.DupEntry
	for i := range entries {
		if entries[i].CalibreID == keep {
			kept = &entries[i]
		}
	}
	if kept == nil {
		return
	}
	for i := range entries {
		if e := &entries[i]; e.CalibreID != keep {
			e.Doubt = doubt(*e, *kept)
		}
	}
}

// keepReason says in a few words why the suggested copy is the one to keep.
func keepReason(entries []store.DupEntry, keep string) string {
	var kept store.DupEntry
	for _, e := range entries {
		if e.CalibreID == keep {
			kept = e
		}
	}
	epub := func(e store.DupEntry) bool {
		for _, f := range e.Files {
			if strings.EqualFold(f.Format, "epub") {
				return true
			}
		}
		return false
	}
	onlyEPUB, moreFiles, larger := epub(kept), true, true
	for _, e := range entries {
		if e.CalibreID == keep {
			continue
		}
		onlyEPUB = onlyEPUB && !epub(e)
		moreFiles = moreFiles && len(kept.Files) > len(e.Files)
		larger = larger && largest(kept) > largest(e)
	}
	switch {
	case onlyEPUB:
		return "it is the one with an EPUB"
	case moreFiles:
		return "it has more formats"
	case larger:
		return "its file is the larger one"
	}
	return "the copies look alike, so the older Calibre entry stays"
}

func doubt(extra, kept store.DupEntry) string {
	has := map[string]int64{}
	for _, f := range kept.Files {
		has[strings.ToLower(f.Format)] = f.Size
	}
	// A file that is not just another e-book format of the text (a PDF, a comic, an audiobook) and that the
	// copy that stays doesn't have would be lost.
	for _, f := range extra.Files {
		ft := strings.ToLower(f.Format)
		if _, ok := has[ft]; !ok && !store.Replaceable[ft] {
			return "has a " + strings.ToUpper(ft) + " that the copy that stays doesn't have"
		}
	}
	a, b := largest(extra), largest(kept)
	for _, f := range extra.Files { // like with like when both have the same format
		if size := has[strings.ToLower(f.Format)]; size > 0 && f.Size > 0 && store.Replaceable[strings.ToLower(f.Format)] {
			a, b = f.Size, size
			break
		}
	}
	if a == 0 || b == 0 {
		return "the files couldn't be compared (their sizes aren't known)"
	}
	if big, small := max(a, b), min(a, b); big > small*5/2 {
		return fmt.Sprintf("the files differ a lot in size (%s here, %s in the copy that stays): maybe another edition, an omnibus or a sample", mb(a), mb(b))
	}
	return ""
}

// largest is the size of an entry's biggest e-book file.
func largest(e store.DupEntry) int64 {
	var best int64
	for _, f := range e.Files {
		if store.Replaceable[strings.ToLower(f.Format)] {
			best = max(best, f.Size)
		}
	}
	return best
}
