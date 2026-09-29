// Package safemode decides whether NovelCheck starts without its background
// work (Calibre sync, rating, Deep Scans, Discover, events, collections, model
// checks): when NOVELCHECK_SAFE_MODE is set, when `novelcheck safe-mode on`
// left a flag file in the data folder, or when NovelCheck stopped
// unexpectedly 3 times within a few minutes of starting. The web app still
// runs, so an admin can look around and leave safe mode from the banner.
package safemode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	flagFile   = "SAFE_MODE"      // written by `novelcheck safe-mode on`
	startsFile = "startups.json"  // recent starts that didn't end cleanly
	crashes    = 3                // this many starts...
	window     = 10 * time.Minute // ...within this long means a crash loop
	HealthyAt  = 5 * time.Minute  // running this long counts as a good start
	EnvVar     = "NOVELCHECK_SAFE_MODE"
)

// Reasons say why safe mode is on.
const (
	ByEnv   = "env"   // NOVELCHECK_SAFE_MODE is set
	ByFlag  = "flag"  // `novelcheck safe-mode on`
	ByCrash = "crash" // stopped unexpectedly several times in a row
)

// Starting records this start and returns why safe mode is on ("" when it
// isn't).
func Starting(dataDir string, now time.Time) string {
	starts := recent(dataDir, now)
	starts = append(starts, now)
	save(dataDir, starts)
	if v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(EnvVar))); err == nil && v {
		return ByEnv
	}
	if FlagSet(dataDir) {
		return ByFlag
	}
	if len(starts) >= crashes {
		return ByCrash
	}
	return ""
}

// Clean forgets the recent starts: after running a while, or on a normal
// shutdown, the start wasn't part of a crash loop.
func Clean(dataDir string) { _ = os.Remove(filepath.Join(dataDir, startsFile)) }

// FlagSet reports whether `novelcheck safe-mode on` is in effect.
func FlagSet(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, flagFile))
	return err == nil
}

// SetFlag turns the safe-mode flag on or off for the next start.
func SetFlag(dataDir string, on bool) error {
	p := filepath.Join(dataDir, flagFile)
	if !on {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(p, []byte("Delete this file (or run `novelcheck safe-mode off`) to leave safe mode.\n"), 0o644)
}

func recent(dataDir string, now time.Time) []time.Time {
	var all, out []time.Time
	data, err := os.ReadFile(filepath.Join(dataDir, startsFile))
	if err == nil {
		_ = json.Unmarshal(data, &all)
	}
	for _, t := range all {
		if now.Sub(t) < window && !t.After(now) {
			out = append(out, t)
		}
	}
	return out
}

func save(dataDir string, starts []time.Time) {
	if data, err := json.Marshal(starts); err == nil {
		_ = os.WriteFile(filepath.Join(dataDir, startsFile), data, 0o644)
	}
}
