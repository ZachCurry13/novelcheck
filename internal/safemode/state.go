package safemode

import (
	"errors"
	"sync"
)

// ErrByEnv means safe mode can't be left from the app: it's set in the
// app's environment.
var ErrByEnv = errors.New("safe mode is set by NOVELCHECK_SAFE_MODE in the app's settings: remove it there and restart the app")

// State is safe mode while the app runs.
type State struct {
	mu      sync.Mutex
	dataDir string
	reason  string
	start   func()
}

// NewState remembers why safe mode is on ("" = it isn't) and how to start
// the background work when an admin leaves it.
func NewState(dataDir, reason string, start func()) *State {
	return &State{dataDir: dataDir, reason: reason, start: start}
}

// Reason says why safe mode is on, or "".
func (s *State) Reason() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason
}

// Leave turns safe mode off and starts the background work.
func (s *State) Leave() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	switch s.reason {
	case "":
		s.mu.Unlock()
		return nil
	case ByEnv:
		s.mu.Unlock()
		return ErrByEnv
	}
	if err := SetFlag(s.dataDir, false); err != nil {
		s.mu.Unlock()
		return err
	}
	Clean(s.dataDir)
	s.reason = ""
	s.mu.Unlock()
	s.start()
	return nil
}
