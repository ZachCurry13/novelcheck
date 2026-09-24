package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// deviceHint is shown when NovelCheck answers from the internet: if a device
// still can't open it, that device's network is the problem.
const deviceHint = "If a phone or computer still can't open it, that device's network is probably remembering an old " +
	"\"not found\" answer (for up to 30 minutes). Try mobile data with Wi-Fi off, wait a bit, run ipconfig /flushdns " +
	"on Windows, or restart your router. A Pi-hole or ad-blocking DNS can also block new addresses."

// checkPublicAddress looks the public address up in DNS and loads it through
// Cloudflare, confirming NovelCheck itself answers (not just that the tunnel
// is connected). base is "https://books.example.com".
func checkPublicAddress(ctx context.Context, base string) (string, string, string) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return "error", "The public address looks wrong: " + base, "Fix it in Admin → Remote access (e.g. books.yourname.com)."
	}
	if net.ParseIP(u.Hostname()) == nil {
		if _, err := net.DefaultResolver.LookupHost(ctx, u.Hostname()); err != nil {
			var dns *net.DNSError
			if errors.As(err, &dns) && dns.IsNotFound {
				return "error", u.Hostname() + " doesn't exist in DNS yet",
					"In Cloudflare Zero Trust → Networks → Tunnels → your tunnel → Public Hostname, add " + u.Hostname() +
						" with service http://localhost:8080. Also check your domain shows Active in the Cloudflare dashboard. New addresses can take a few minutes."
			}
			return "warn", "Couldn't look up " + u.Hostname() + ": " + err.Error(), "NovelCheck's own DNS may be having trouble; try again shortly."
		}
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/api/setup", nil)
	res, err := client.Do(req)
	if err != nil {
		return "error", "Couldn't load " + base + ": " + err.Error(), "Check the Public Hostname in Cloudflare Zero Trust points to http://localhost:8080."
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var probe struct {
		Needed *bool `json:"needed"`
	}
	switch {
	case res.StatusCode == http.StatusOK && json.Unmarshal(body, &probe) == nil && probe.Needed != nil:
		return "ok", "Opens from the internet at " + base, deviceHint
	case res.StatusCode >= 300 && res.StatusCode < 400 && strings.Contains(res.Header.Get("Location"), "cloudflareaccess.com"):
		return "ok", "Reachable, behind a Cloudflare Access sign-in page", deviceHint
	case res.StatusCode == http.StatusNotFound && strings.Contains(res.Header.Get("Server"), "cloudflare"):
		return "error", "Cloudflare doesn't know where to send " + u.Hostname(),
			"In the tunnel's Public Hostname, check the address is exactly " + u.Hostname() + " and the service is http://localhost:8080."
	case res.StatusCode == 502 || res.StatusCode == 530 || res.StatusCode == 1033:
		return "error", fmt.Sprintf("Cloudflare answered %d: it can't reach NovelCheck through the tunnel", res.StatusCode),
			"Check the tunnel's Public Hostname service is http://localhost:8080 (not your server's IP), and that remote access shows Connected."
	default:
		return "error", fmt.Sprintf("%s answered %d, but it isn't NovelCheck", base, res.StatusCode),
			"Another site or service may be using this address. Check the Public Hostname in Cloudflare Zero Trust."
	}
}
