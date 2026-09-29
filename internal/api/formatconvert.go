package api

import (
	"context"
	"strconv"
	"time"

	"github.com/zachcurry13/novelcheck/internal/calibresrv"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// runConvert has calibre convert each book to EPUB, one at a time.
func (s *Server) runConvert(files []store.CalibreFile) {
	defer s.afterFormatJob()
	c, j := s.calibreClient(), &s.formats
	want := map[int]string{}
	for _, f := range files {
		if id, err := strconv.Atoi(f.CalibreID); err == nil {
			want[id] = f.Title
		}
	}
	have, err := s.checkTitles(c, want)
	if err != nil {
		j.end("Stopped: " + err.Error() + " Nothing was converted.")
		return
	}
	for _, f := range files {
		if j.stopped() {
			break
		}
		j.update(func(st *formatJobState) { st.Current, st.Percent = f.Title, 0 })
		id, _ := strconv.Atoi(f.CalibreID)
		if b, ok := have[id]; !ok || b.Has("EPUB") || !b.Has(f.Format) {
			j.update(func(st *formatJobState) { st.Skipped++ })
			continue
		}
		if why := s.convertOne(c, id, f.Format); why != "" {
			j.fail(f.Title, why)
			continue
		}
		j.update(func(st *formatJobState) { st.Done++ })
	}
	st := j.state()
	j.end("Converted " + count(st.Done, "book") + " to EPUB.")
}

// convertOne starts one conversion and follows it; calibre adds the EPUB
// once it's asked for the finished job's status. "" means it worked.
func (s *Server) convertOne(c *calibresrv.Client, id int, from string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	job, err := c.StartConversion(ctx, id, from)
	if err != nil {
		return err.Error()
	}
	for {
		st, err := c.ConversionStatus(ctx, job)
		if err != nil {
			return err.Error()
		}
		if !st.Running {
			if st.OK {
				return ""
			}
			return lastLine(st.Trace, st.Log, st.Msg)
		}
		pct := st.Percent
		if pct <= 1 {
			pct *= 100
		}
		s.formats.update(func(js *formatJobState) { js.Percent = int(pct) })
		select {
		case <-ctx.Done():
			return "took longer than 30 minutes"
		case <-time.After(2 * time.Second):
		}
	}
}
