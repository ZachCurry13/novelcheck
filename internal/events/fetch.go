package events

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// maxPage caps a fetched page.
const maxPage = 4 << 20

// ErrPrivate refuses addresses on the home network: a parent types the page
// address, and it must not reach devices on the network.
var ErrPrivate = errors.New("only public web pages can be read; paste the list instead")

var client = &http.Client{
	Timeout: 20 * time.Second,
	Transport: &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{Timeout: 10 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip == nil || !public(ip) {
				return ErrPrivate
			}
			return nil
		}}).DialContext,
	},
}

func public(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast())
}

// Fetch reads an event's page. Some sites refuse programs; pasting the page
// always works.
func Fetch(ctx context.Context, rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("that doesn't look like a web address (https://…)")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; NovelCheck; +https://github.com/ZachCurry13/novelcheck)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	res, err := client.Do(req)
	if err != nil {
		if errors.Is(err, ErrPrivate) {
			return "", ErrPrivate
		}
		return "", fmt.Errorf("the page couldn't be read (%v); paste the list instead", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the site answered %s; paste the list instead", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxPage))
	return string(body), err
}
