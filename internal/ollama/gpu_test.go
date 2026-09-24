package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fits(recs []Rec) map[string]string {
	m := map[string]string{}
	for _, r := range recs {
		m[r.Name] = r.Fit
	}
	return m
}

func TestRecommend(t *testing.T) {
	gb := func(n float64) int64 { return int64(n * (1 << 30)) }
	// 8 GB card: 7B is the best; gemma3:12b (8.1) doesn't fit.
	r := fits(Recommend(GPU{Kind: "about", VRAMBytes: gb(8)}))
	if r["qwen2.5:7b"] != "best" || r["llama3.1:8b"] != "powerful" || r["qwen2.5:14b"] != "too_big" || r["llama3.2"] != "fits" {
		t.Fatalf("8 GB: %v", r)
	}
	// 12 GB: 14B is both the best and the most powerful that fits.
	if r := fits(Recommend(GPU{Kind: "manual", VRAMBytes: gb(12)})); r["qwen2.5:14b"] != "best_powerful" || r["gemma3:27b"] != "too_big" {
		t.Fatalf("12 GB: %v", r)
	}
	// 24 GB: 14B best, 32B most powerful.
	if r := fits(Recommend(GPU{Kind: "manual", VRAMBytes: gb(24)})); r["qwen2.5:14b"] != "best" || r["qwen2.5:32b"] != "powerful" {
		t.Fatalf("24 GB: %v", r)
	}
	// CPU only: small models OK, the rest slow.
	if r := fits(Recommend(GPU{Kind: "none"})); r["llama3.2"] != "cpu_ok" || r["qwen2.5:7b"] != "cpu_slow" {
		t.Fatalf("cpu: %v", r)
	}
	if r := fits(Recommend(GPU{Kind: "unknown"})); r["qwen2.5:7b"] != "" {
		t.Fatalf("unknown should not label: %v", r)
	}
}

// fakeGPUOllama answers tags/ps/generate; loading puts part of the model on the GPU.
func fakeGPUOllama(t *testing.T, vram int64) (*httptest.Server, *[]string) {
	var calls []string
	loaded := false
	size := int64(5_000_000_000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			fmt.Fprintf(w, `{"models":[{"name":"llama3.2:latest","size":2000000000},{"name":"qwen2.5:7b","size":%d}]}`, size)
		case "/api/ps":
			if loaded {
				fmt.Fprintf(w, `{"models":[{"name":"qwen2.5:7b","size":%d,"size_vram":%d}]}`, size, vram)
			} else {
				fmt.Fprint(w, `{"models":[]}`)
			}
		case "/api/generate":
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			calls = append(calls, fmt.Sprintf("%v:%v", req["model"], req["keep_alive"]))
			loaded = req["keep_alive"] != "0"
			fmt.Fprint(w, `{"done":true}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestMeasureGPU(t *testing.T) {
	srv, calls := fakeGPUOllama(t, 3_000_000_000) // 3 GB of 5 on the GPU
	if g, _ := MeasureGPU(context.Background(), srv.URL, false); g.Kind != "unknown" {
		t.Fatalf("without probe and nothing loaded: %+v", g)
	}
	g, err := MeasureGPU(context.Background(), srv.URL, true)
	if err != nil || g.Kind != "about" || g.VRAMBytes != 3_000_000_000 || g.Basis != "qwen2.5:7b" {
		t.Fatalf("probe: %+v %v", g, err)
	}
	if strings.Join(*calls, " ") != "qwen2.5:7b:1m qwen2.5:7b:0" {
		t.Fatalf("should load the largest model then unload it: %v", *calls)
	}
	cpu, _ := fakeGPUOllama(t, 0)
	if g, _ := MeasureGPU(context.Background(), cpu.URL, true); g.Kind != "none" {
		t.Fatalf("CPU only: %+v", g)
	}
}
