package api

import "testing"

// Only a stop for a service that may be back soon is tried again; the counts and kept suggestions survive a resume.
func TestScanRetryableAndResume(t *testing.T) {
	for note, want := range map[string]bool{
		"Calibre didn't answer: connection refused":                      true,
		"Hardcover: Hardcover says too many requests; it will try again": true,
		"Hardcover didn't accept the token":                              false,
		"Stopped; what was found so far is below.":                       false,
	} {
		if got := scanRetryable(note); got != want {
			t.Errorf("%q: %v", note, got)
		}
	}
	var j hcJob
	j.start("scan", 100)
	j.update(func(s *hcJobState) { s.Done, s.Found = 50, 7 })
	j.end("Calibre didn't answer")
	j.wait("resting")
	if !j.state().Running {
		t.Fatal("resting keeps the scan running (Stop shows)")
	}
	j.resume(50)
	if st := j.state(); !st.Running || st.Done != 50 || st.Found != 7 || st.Total != 100 || st.Note != "" {
		t.Fatalf("resume: %+v", st)
	}
}
