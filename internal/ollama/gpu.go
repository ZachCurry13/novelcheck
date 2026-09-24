package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GPU is what NovelCheck could learn about the Ollama server's graphics
// memory. Ollama has no "what GPU do you have" call, so this is read from how
// much of a loaded model sits on the GPU.
type GPU struct {
	Kind      string `json:"kind"` // "none" (CPU only), "about" (≈VRAM), "at_least", "manual", "unknown"
	VRAMBytes int64  `json:"vram_bytes"`
	Basis     string `json:"basis,omitempty"` // model the measurement came from
}

// ModelInfo is a model that works well for rating books.
type ModelInfo struct {
	Name   string  `json:"name"`
	Label  string  `json:"label"`
	SizeGB float64 `json:"size_gb"` // Ollama download / memory size
	Note   string  `json:"note"`
}

// Catalog lists good book-rating models from small to large.
var Catalog = []ModelInfo{
	{"qwen2.5:3b", "Qwen 2.5 3B", 1.9, "small and quick"},
	{"llama3.2", "Llama 3.2 3B", 2.0, "fastest; fine without a GPU"},
	{"qwen2.5:7b", "Qwen 2.5 7B", 4.7, "great balance of speed and accuracy"},
	{"llama3.1:8b", "Llama 3.1 8B", 4.9, "good alternative"},
	{"gemma3:12b", "Gemma 3 12B", 8.1, "strong, a bit slower"},
	{"qwen2.5:14b", "Qwen 2.5 14B", 9.0, "very accurate ratings"},
	{"gemma3:27b", "Gemma 3 27B", 17, "large"},
	{"qwen2.5:32b", "Qwen 2.5 32B", 20, "most accurate; needs a big GPU"},
}

// balanced are the preferred everyday picks, best first.
var balanced = []string{"qwen2.5:14b", "qwen2.5:7b", "llama3.2"}

// headroomGB is spare GPU memory a model needs beyond its size (context,
// buffers) to run entirely on the GPU.
const headroomGB = 1.5

// Rec is a catalog model labelled for this GPU.
type Rec struct {
	ModelInfo
	Fit string `json:"fit"` // best | powerful | best_powerful | fits | too_big | cpu_ok | cpu_slow | ""
}

// Recommend labels every catalog model for g.
func Recommend(g GPU) []Rec {
	out := make([]Rec, len(Catalog))
	for i, m := range Catalog {
		out[i] = Rec{ModelInfo: m}
	}
	switch g.Kind {
	case "none":
		for i := range out {
			out[i].Fit = "cpu_slow"
			if out[i].SizeGB <= 2.1 {
				out[i].Fit = "cpu_ok"
			}
		}
		return out
	case "about", "at_least", "manual":
	default:
		return out // unknown: no labels
	}
	vram := float64(g.VRAMBytes) / (1 << 30)
	fits := func(m ModelInfo) bool { return m.SizeGB+headroomGB <= vram }
	powerful, best := "", ""
	for _, m := range Catalog {
		if fits(m) {
			powerful = m.Name // catalog is sorted small to large
		}
	}
	for _, name := range balanced {
		for _, m := range Catalog {
			if m.Name == name && fits(m) && best == "" {
				best = name
			}
		}
	}
	for i := range out {
		switch {
		case out[i].Name == best && best == powerful:
			out[i].Fit = "best_powerful"
		case out[i].Name == best:
			out[i].Fit = "best"
		case out[i].Name == powerful:
			out[i].Fit = "powerful"
		case fits(out[i].ModelInfo):
			out[i].Fit = "fits"
		default:
			out[i].Fit = "too_big"
		}
	}
	return out
}

// fromLoaded reads the GPU from a loaded model's memory split.
func fromLoaded(l Loaded) GPU {
	switch {
	case l.VRAMBytes <= 0:
		return GPU{Kind: "none", Basis: l.Name}
	case float64(l.VRAMBytes) < float64(l.SizeBytes)*0.98:
		return GPU{Kind: "about", VRAMBytes: l.VRAMBytes, Basis: l.Name}
	default:
		return GPU{Kind: "at_least", VRAMBytes: l.VRAMBytes, Basis: l.Name}
	}
}

// MeasureGPU looks at loaded models. With probe, if none is loaded it loads
// the largest downloaded model for a moment, reads the split, and unloads it.
func MeasureGPU(ctx context.Context, base string, probe bool) (GPU, error) {
	if g, ok := largestLoaded(ctx, base); ok {
		return g, nil
	}
	if !probe {
		return GPU{Kind: "unknown"}, nil
	}
	var tags struct {
		Models []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"models"`
	}
	if err := getJSON(ctx, base+"/api/tags", &tags); err != nil {
		return GPU{Kind: "unknown"}, err
	}
	name, size := "", int64(0)
	for _, m := range tags.Models {
		if m.Size > size {
			name, size = m.Name, m.Size
		}
	}
	if name == "" {
		return GPU{Kind: "unknown"}, fmt.Errorf("download a model first, then check the GPU")
	}
	if err := setKeepAlive(ctx, base, name, "1m"); err != nil {
		return GPU{Kind: "unknown"}, fmt.Errorf("couldn't load %s: %w", name, err)
	}
	g, ok := largestLoaded(ctx, base)
	_ = setKeepAlive(context.Background(), base, name, "0") // unload again
	if !ok {
		return GPU{Kind: "unknown"}, fmt.Errorf("%s loaded but Ollama didn't report it", name)
	}
	return g, nil
}

func largestLoaded(ctx context.Context, base string) (GPU, bool) {
	loaded, err := PS(ctx, base)
	if err != nil || len(loaded) == 0 {
		return GPU{}, false
	}
	big := loaded[0]
	for _, l := range loaded[1:] {
		if l.SizeBytes > big.SizeBytes {
			big = l
		}
	}
	return fromLoaded(big), true
}

// setKeepAlive loads a model (keep "1m") or unloads it (keep "0") with an
// empty generate request, the way `ollama run`/`ollama stop` do.
func setKeepAlive(ctx context.Context, base, model, keep string) error {
	body, _ := json.Marshal(map[string]any{"model": model, "prompt": "", "keep_alive": keep, "stream": false})
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute) // a big model can take a while to load
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama answered %s", res.Status)
	}
	return nil
}
