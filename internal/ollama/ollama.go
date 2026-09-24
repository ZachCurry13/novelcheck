// Package ollama helps connect NovelCheck to an Ollama server (for example
// the TrueNAS Ollama app): it finds the server, lists its models, and
// downloads new ones with progress, so no terminal commands are needed.
package ollama

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Server is a reachable Ollama instance.
type Server struct {
	URL     string   `json:"url"` // e.g. http://192.168.1.50:11434
	Version string   `json:"version"`
	Models  []string `json:"models"`
}

// Ports tried during discovery: Ollama's default and common TrueNAS app ports.
var Ports = []string{"11434", "30068"}

var client = &http.Client{Timeout: 4 * time.Second}

// Normalize validates a user-entered address and strips any /v1 suffix.
func Normalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("enter the Ollama address")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("that doesn't look like an address, for example http://192.168.1.50:11434")
	}
	u.Path = strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), "/v1")
	u.RawQuery, u.Fragment = "", ""
	return strings.TrimSuffix(u.String(), "/"), nil
}

// Probe checks one address and returns its version and installed models.
func Probe(ctx context.Context, base string) (*Server, error) {
	var ver struct {
		Version string `json:"version"`
	}
	if err := getJSON(ctx, base+"/api/version", &ver); err != nil {
		return nil, err
	}
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := getJSON(ctx, base+"/api/tags", &tags); err != nil {
		return nil, err
	}
	s := &Server{URL: base, Version: ver.Version, Models: []string{}}
	for _, m := range tags.Models {
		s.Models = append(s.Models, m.Name)
	}
	return s, nil
}

func getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", u, resp.Status)
	}
	return json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(out)
}

// Candidates lists addresses where an Ollama server is likely to be: the
// Docker host (the TrueNAS box, reached through the container's gateway),
// host.docker.internal, and a same-network container named "ollama".
func Candidates(extra ...string) []string {
	hosts := []string{}
	if gw := defaultGateway(); gw != "" {
		hosts = append(hosts, gw)
	}
	hosts = append(hosts, "host.docker.internal", "ollama", "localhost")
	var out []string
	seen := map[string]bool{}
	add := func(u string) {
		if n, err := Normalize(u); err == nil && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, e := range extra {
		if e != "" {
			add(e)
		}
	}
	for _, h := range hosts {
		for _, p := range Ports {
			add("http://" + net.JoinHostPort(h, p))
		}
	}
	return out
}

// Discover probes all candidates in parallel and returns those that answer.
func Discover(ctx context.Context, extra ...string) []Server {
	cands := Candidates(extra...)
	results := make([]*Server, len(cands))
	var wg sync.WaitGroup
	for i, c := range cands {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s, err := Probe(ctx, c); err == nil {
				results[i] = s
			}
		}()
	}
	wg.Wait()
	found := []Server{}
	versions := map[string]bool{} // the same server can answer on several names
	for _, s := range results {
		if s == nil {
			continue
		}
		key := s.Version + "|" + strings.Join(s.Models, ",")
		if versions[key] {
			continue
		}
		versions[key] = true
		found = append(found, *s)
	}
	return found
}

// defaultGateway reads the container's default route from /proc/net/route.
func defaultGateway() string {
	b, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 3 || f[1] != "00000000" {
			continue
		}
		raw, err := hex.DecodeString(f[2])
		if err != nil || len(raw) != 4 {
			continue
		}
		// /proc stores the address little-endian.
		return net.IPv4(raw[3], raw[2], raw[1], raw[0]).String()
	}
	return ""
}
