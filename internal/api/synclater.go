package api

import "sync"

// laterSync re-reads the Calibre library in the background after a save. Saves that come
// while a read is running are folded into one more read, so five quick saves cost two
// reads at most, not five one after another. ratings of the saved books are kept (accept).
type laterSync struct {
	mu      sync.Mutex
	running bool
	again   bool
	accept  []int64
}

func (s *Server) syncSoon(accept []int64) {
	l := &s.later
	l.mu.Lock()
	l.accept = append(l.accept, accept...)
	if l.running {
		l.again = true
		l.mu.Unlock()
		return
	}
	l.running = true
	l.mu.Unlock()
	go func() {
		for {
			l.mu.Lock()
			acc := l.accept
			l.accept, l.again = nil, false
			l.mu.Unlock()
			if s.Syncer != nil && s.Syncer.Available() {
				if _, err := s.Syncer.Run(); err == nil {
					_ = s.Store.AcceptModified(acc) // the only change was ours: the ratings stay
				}
			}
			l.mu.Lock()
			if !l.again {
				l.running = false
				l.mu.Unlock()
				return
			}
			l.mu.Unlock()
		}
	}()
}
