// Package version holds the running build's version and compares versions.
package version

import (
	"strconv"
	"strings"
)

// Version is set at build time:
//
//	go build -ldflags "-X github.com/zachcurry13/novelcheck/internal/version.Version=1.1.0"
//
// Release builds use the release number; builds from main between releases
// use `git describe` output such as "1.1.0-3-gabc1234" (3 commits after 1.1.0).
var Version = "dev"

// core parses "v1.2.3", "1.2.3" or "1.2.3-4-gabc" into [1 2 3]; ok is false for
// anything else (e.g. "dev").
func core(v string) (nums [3]int, ok bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nums, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nums, false
		}
		nums[i] = n
	}
	return nums, true
}

// Valid reports whether v looks like a semantic version.
func Valid(v string) bool {
	_, ok := core(v)
	return ok
}

// Newer reports whether release is a newer release than current. Unknown
// current versions (e.g. "dev" builds) never report updates.
func Newer(release, current string) bool {
	r, ok1 := core(release)
	c, ok2 := core(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range 3 {
		if r[i] != c[i] {
			return r[i] > c[i]
		}
	}
	return false
}
