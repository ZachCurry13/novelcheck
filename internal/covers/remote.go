package covers

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// remoteHosts are the only places a list's cover image is fetched from: the
// New York Times' image store and Open Library.
var remoteHosts = map[string]bool{"storage.googleapis.com": true, "covers.openlibrary.org": true}

// Remote returns a cover a Discover list pointed to (for books Open Library
// has no ISBN cover for), fetched once and kept. Other hosts are refused.
func (c Cache) Remote(ctx context.Context, raw string) (string, os.FileInfo, bool) {
	u, err := url.Parse(raw)
	if c.Dir == "" || err != nil || u.Scheme != "https" || !remoteHosts[u.Hostname()] {
		return "", nil, false
	}
	sum := sha1.Sum([]byte(raw))
	out := filepath.Join(c.Dir, "rm-"+hex.EncodeToString(sum[:10])+".jpg")
	if info, err := os.Stat(out); err == nil {
		return out, info, true
	}
	miss := out + ".none"
	if info, err := os.Stat(miss); err == nil && time.Since(info.ModTime()) < 7*24*time.Hour {
		return "", nil, false
	}
	return c.fetch(ctx, raw, out, miss)
}
