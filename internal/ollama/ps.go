package ollama

import (
	"context"
	"time"
)

// Loaded is a model Ollama currently holds in memory.
type Loaded struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"` // total memory the model uses
	VRAMBytes int64  `json:"vram_bytes"` // part of it on the graphics card
	ExpiresAt string `json:"expires_at"` // when Ollama unloads it if idle
}

// PS lists loaded models (Ollama's /api/ps, same data as `ollama ps`).
// Ollama doesn't expose CPU/GPU utilisation percentages.
func PS(ctx context.Context, base string) ([]Loaded, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	var res struct {
		Models []struct {
			Name      string `json:"name"`
			Size      int64  `json:"size"`
			SizeVRAM  int64  `json:"size_vram"`
			ExpiresAt string `json:"expires_at"`
		} `json:"models"`
	}
	if err := getJSON(ctx, base+"/api/ps", &res); err != nil {
		return nil, err
	}
	out := []Loaded{}
	for _, m := range res.Models {
		out = append(out, Loaded{Name: m.Name, SizeBytes: m.Size, VRAMBytes: m.SizeVRAM, ExpiresAt: m.ExpiresAt})
	}
	return out, nil
}
