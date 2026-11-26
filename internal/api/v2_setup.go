package api

// setupStep is one line of the admin's "Getting started" list on Home.
type setupStep struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Done     bool   `json:"done"`
	Optional bool   `json:"optional,omitempty"`
	Link     string `json:"link"`
}

// setupSteps is what a new NovelCheck needs, in the order to do it, each with whether it is done and where to do it. Home
// shows it to the admins until every step is done (or they hide it for a week).
func (s *Server) setupSteps() []setupStep {
	books := 0
	if c, err := s.Store.StatusCounts(); err == nil {
		for _, n := range c {
			books += n
		}
	}
	users, _ := s.Store.CountUsers()
	return []setupStep{
		{"books", "Add your books", "Point NovelCheck at your Calibre library (or upload books) so there is something to rate.", books > 0, false, "#/admin?tab=delivery"},
		{"ai", "Choose an AI to rate them", "An AI reads each book's description and gives it a spice level. A free one is enough to start.", len(s.Store.AIConfigs()) > 0, false, "#/admin?tab=ai"},
		{"hardcover", "Connect Hardcover", "Hardcover gives NovelCheck ratings, genres, covers and the true titles. It is free; copy a token from your account.", s.Store.RatingsToken() != "", false, "#/settings?s=connections"},
		{"family", "Add the people who will use it", "Each person gets their own Nightstand and rules; kids' accounts see only what you allow.", users > 1, false, "#/admin?tab=users"},
		{"backup", "Make a backup", "A copy of NovelCheck's data is made every day; check one exists and keep a copy somewhere else too.", len(s.backupFiles()) > 0, true, "#/health"},
		{"kindle", "Set up a Kindle or KOReader", "Optional: the Kindle app or the KOReader plugin sends what is on the e-reader and brings back your Nightstand.", len(s.Store.AllDevices()) > 0, true, "#/settings?s=kindle"},
	}
}

// setupDone says whether every step that is not optional is done.
func setupDone(steps []setupStep) bool {
	for _, st := range steps {
		if !st.Done && !st.Optional {
			return false
		}
	}
	return true
}
