package audiobookshelf

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
)

// Track is one audio file of an audiobook; Start is where it begins, in
// seconds from the start of the whole book.
type Track struct {
	Ino      string  `json:"ino"`
	Title    string  `json:"title"`
	Start    float64 `json:"start"`
	Duration float64 `json:"duration"`
	Mime     string  `json:"mime"`
}

// Chapter is a named part of the whole book.
type Chapter struct {
	Title string  `json:"title"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Bookmark is a saved spot.
type Bookmark struct {
	Time  float64 `json:"time"`
	Title string  `json:"title"`
}

// Progress is how far someone is.
type Progress struct {
	ItemID      string  `json:"item_id"`
	CurrentTime float64 `json:"current_time"`
	Duration    float64 `json:"duration"`
	Fraction    float64 `json:"progress"`
	Finished    bool    `json:"finished"`
	Updated     int64   `json:"updated"` // ms since 1970
}

// Detail is what the player needs for one audiobook.
type Detail struct {
	Tracks    []Track    `json:"tracks"`
	Chapters  []Chapter  `json:"chapters"`
	Bookmarks []Bookmark `json:"bookmarks"`
	Progress  *Progress  `json:"progress"`
	Duration  float64    `json:"duration"`
}

type rawProgress struct {
	ItemID      string  `json:"libraryItemId"`
	CurrentTime float64 `json:"currentTime"`
	Duration    float64 `json:"duration"`
	Progress    float64 `json:"progress"`
	Finished    bool    `json:"isFinished"`
	LastUpdate  int64   `json:"lastUpdate"`
}

func (r rawProgress) progress() *Progress {
	return &Progress{ItemID: r.ItemID, CurrentTime: r.CurrentTime, Duration: r.Duration, Fraction: r.Progress, Finished: r.Finished, Updated: r.LastUpdate}
}

// Me reads the signed-in person: every book they have progress on, and their bookmarks.
func (c *Client) Me(ctx context.Context) (map[string]*Progress, map[string][]Bookmark, error) {
	var out struct {
		Progress  []rawProgress `json:"mediaProgress"`
		Bookmarks []struct {
			ItemID string  `json:"libraryItemId"`
			Time   float64 `json:"time"`
			Title  string  `json:"title"`
		} `json:"bookmarks"`
	}
	if err := c.JSON(ctx, http.MethodGet, "/api/me", nil, &out); err != nil {
		return nil, nil, err
	}
	prog, marks := map[string]*Progress{}, map[string][]Bookmark{}
	for _, p := range out.Progress {
		prog[p.ItemID] = p.progress()
	}
	for _, b := range out.Bookmarks {
		marks[b.ItemID] = append(marks[b.ItemID], Bookmark{b.Time, b.Title})
	}
	return prog, marks, nil
}

// Detail reads an audiobook's files and chapters, and this person's place and bookmarks in it.
func (c *Client) Detail(ctx context.Context, id string) (Detail, error) {
	var it struct {
		Media struct {
			Duration   float64 `json:"duration"`
			AudioFiles []struct {
				Index    int     `json:"index"`
				Ino      string  `json:"ino"`
				Duration float64 `json:"duration"`
				Mime     string  `json:"mimeType"`
				Meta     struct {
					Filename string `json:"filename"`
				} `json:"metadata"`
			} `json:"audioFiles"`
			Chapters []Chapter `json:"chapters"`
		} `json:"media"`
	}
	if err := c.JSON(ctx, http.MethodGet, "/api/items/"+url.PathEscape(id)+"?expanded=1", nil, &it); err != nil {
		return Detail{}, err
	}
	d := Detail{Duration: it.Media.Duration, Chapters: it.Media.Chapters, Tracks: []Track{}, Bookmarks: []Bookmark{}}
	files := it.Media.AudioFiles
	sort.Slice(files, func(i, j int) bool { return files[i].Index < files[j].Index })
	var at float64
	for i, f := range files {
		d.Tracks = append(d.Tracks, Track{Ino: f.Ino, Title: fmt.Sprintf("%d. %s", i+1, f.Meta.Filename), Start: at, Duration: f.Duration, Mime: f.Mime})
		at += f.Duration
	}
	if prog, marks, err := c.Me(ctx); err == nil {
		d.Progress, d.Bookmarks = prog[id], append(d.Bookmarks, marks[id]...)
	}
	return d, nil
}

// SaveProgress records where the person is (seconds into the whole book).
func (c *Client) SaveProgress(ctx context.Context, id string, at, duration float64, finished bool) error {
	frac := 0.0
	if duration > 0 {
		frac = at / duration
	}
	return c.JSON(ctx, http.MethodPatch, "/api/me/progress/"+url.PathEscape(id),
		map[string]any{"currentTime": at, "duration": duration, "progress": frac, "isFinished": finished}, nil)
}

// AddBookmark saves a spot; DeleteBookmark removes the one at that time.
func (c *Client) AddBookmark(ctx context.Context, id string, at float64, title string) error {
	return c.JSON(ctx, http.MethodPost, "/api/me/item/"+url.PathEscape(id)+"/bookmark", map[string]any{"time": at, "title": title}, nil)
}

func (c *Client) DeleteBookmark(ctx context.Context, id string, at float64) error {
	return c.JSON(ctx, http.MethodDelete, fmt.Sprintf("/api/me/item/%s/bookmark/%d", url.PathEscape(id), int(at)), nil, nil)
}
