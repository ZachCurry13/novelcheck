package ollama

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Registry is Ollama's model library, asked whether a model has a newer
// version than the one installed.
var Registry = "https://registry.ollama.ai"

var registryClient = &http.Client{Timeout: 20 * time.Second}

// LocalDigests returns each installed model's digest (name → sha256 of its
// manifest), as Ollama reports it.
func LocalDigests(ctx context.Context, base string) (map[string]string, error) {
	var tags struct {
		Models []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"models"`
	}
	if err := getJSON(ctx, base+"/api/tags", &tags); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, m := range tags.Models {
		out[m.Name] = strings.TrimPrefix(m.Digest, "sha256:")
	}
	return out, nil
}

// RemoteDigest is the library's current digest for a model such as
// "qwen2.5:7b" (the sha256 of its manifest, which is what Ollama stores).
func RemoteDigest(ctx context.Context, name string) (string, error) {
	repo, tag, ok := strings.Cut(name, ":")
	if !ok || tag == "" {
		tag = "latest"
	}
	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v2/%s/manifests/%s", Registry, repo, tag), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	res, err := registryClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the model library answered %s for %s", res.Status, name)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

// Update is a model with a newer version in the library.
type Update struct {
	Server string `json:"server"` // the Ollama address
	Model  string `json:"model"`
	Digest string `json:"digest"` // the library's version
}

// Outdated returns which of models (installed on the Ollama at base) have a
// newer version in the library. Models not installed or not in the library
// (your own) are skipped.
func Outdated(ctx context.Context, base string, models []string) ([]Update, error) {
	local, err := LocalDigests(ctx, base)
	if err != nil {
		return nil, err
	}
	var out []Update
	var errs []string
	for _, m := range models {
		have, ok := local[m]
		if !ok && !strings.Contains(m, ":") {
			have, ok = local[m+":latest"]
		}
		if !ok || have == "" {
			continue
		}
		latest, err := RemoteDigest(ctx, m)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if latest != have {
			out = append(out, Update{Server: base, Model: m, Digest: latest})
		}
	}
	if len(out) == 0 && len(errs) > 0 && len(errs) == len(models) {
		return nil, errors.New(strings.Join(errs, "; "))
	}
	return out, nil
}
