package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Model is an installed model and the disk space it takes.
type Model struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"` // bytes on disk
	ModifiedAt string `json:"modified_at"`
	Params     string `json:"parameter_size"` // e.g. "3.2B"
	Quant      string `json:"quantization"`   // e.g. "Q4_K_M"
}

// Installed lists the models on an Ollama server, biggest first.
func Installed(ctx context.Context, base string) ([]Model, error) {
	var tags struct {
		Models []struct {
			Name       string `json:"name"`
			Size       int64  `json:"size"`
			ModifiedAt string `json:"modified_at"`
			Details    struct {
				ParameterSize     string `json:"parameter_size"`
				QuantizationLevel string `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := getJSON(ctx, base+"/api/tags", &tags); err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(tags.Models))
	for _, m := range tags.Models {
		out = append(out, Model{Name: m.Name, Size: m.Size, ModifiedAt: m.ModifiedAt,
			Params: m.Details.ParameterSize, Quant: m.Details.QuantizationLevel})
	}
	for i := 1; i < len(out); i++ { // largest first: what frees the most space
		for j := i; j > 0 && out[j].Size > out[j-1].Size; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

// Delete removes a model from the Ollama server to free disk space.
func Delete(ctx context.Context, base, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("no model named")
	}
	body, _ := json.Marshal(map[string]string{"model": name, "name": name}) // newer and older Ollama
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, base+"/api/delete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("Ollama has no model called %q", name)
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("Ollama couldn't delete %q: %s %s", name, resp.Status, bytes.TrimSpace(msg))
	}
	return nil
}
