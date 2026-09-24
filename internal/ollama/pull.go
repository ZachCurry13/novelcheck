package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// PullStatus reports a model download's progress to the admin panel.
type PullStatus struct {
	Active    bool    `json:"active"`
	Model     string  `json:"model"`
	Server    string  `json:"server"`
	Status    string  `json:"status"` // Ollama's step, e.g. "pulling manifest"
	Percent   float64 `json:"percent"`
	Done      bool    `json:"done"`
	Error     string  `json:"error,omitempty"`
	StartedAt string  `json:"started_at,omitempty"`
}

// Puller runs one model download at a time in the background.
type Puller struct {
	// OnError, if set, is called when a download fails (for notifications).
	OnError func(model string, err error)

	mu     sync.Mutex
	status PullStatus
}

func (p *Puller) Status() PullStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

func (p *Puller) update(fn func(*PullStatus)) {
	p.mu.Lock()
	fn(&p.status)
	p.mu.Unlock()
}

// validModel keeps model names to Ollama's charset (e.g. "llama3.2", "qwen2.5:7b").
func validModel(m string) bool {
	if m == "" || len(m) > 120 {
		return false
	}
	for _, r := range m {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(":._-/", r)) {
			return false
		}
	}
	return true
}

// Start begins downloading model on server; it returns immediately.
func (p *Puller) Start(server, model string) error {
	if !validModel(model) {
		return errors.New("invalid model name")
	}
	p.mu.Lock()
	if p.status.Active {
		p.mu.Unlock()
		return errors.New("a download is already running")
	}
	p.status = PullStatus{Active: true, Model: model, Server: server, Status: "starting",
		StartedAt: time.Now().UTC().Format(time.RFC3339)}
	p.mu.Unlock()
	go p.run(server, model)
	return nil
}

func (p *Puller) run(server, model string) {
	err := p.pull(server, model)
	if err != nil && p.OnError != nil {
		p.OnError(model, err)
	}
	p.update(func(s *PullStatus) {
		s.Active = false
		if err != nil {
			s.Error = err.Error()
			return
		}
		s.Done, s.Percent, s.Status = true, 100, "success"
	})
}

func (p *Puller) pull(server, model string) error {
	// Big models can take a long time on home internet; cap at 3 hours.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
	defer cancel()
	body, _ := json.Marshal(map[string]any{"model": model, "stream": true})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return fmt.Errorf("can't reach Ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama answered %s", resp.Status)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var ev struct {
			Status    string `json:"status"`
			Total     int64  `json:"total"`
			Completed int64  `json:"completed"`
			Error     string `json:"error"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		if ev.Error != "" {
			return errors.New(ev.Error)
		}
		p.update(func(s *PullStatus) {
			s.Status = ev.Status
			if ev.Total > 0 {
				s.Percent = float64(ev.Completed) * 100 / float64(ev.Total)
			}
		})
		if ev.Status == "success" {
			return nil
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return errors.New("download ended before finishing")
}
