package api_test

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
)

// fakeSMTP accepts any mail and records the AUTH PLAIN password of each login.
func fakeSMTP(t *testing.T) (port string, passwords func() []string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var mu sync.Mutex
	var got []string
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				fmt.Fprint(c, "220 fake\r\n")
				inData := false
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimRight(line, "\r\n")
					switch {
					case inData:
						if line == "." {
							inData = false
							fmt.Fprint(c, "250 ok\r\n")
						}
					case strings.HasPrefix(line, "EHLO"):
						fmt.Fprint(c, "250-fake\r\n250 AUTH PLAIN\r\n")
					case strings.HasPrefix(line, "AUTH PLAIN "):
						raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, "AUTH PLAIN "))
						parts := strings.Split(string(raw), "\x00")
						mu.Lock()
						got = append(got, parts[len(parts)-1])
						mu.Unlock()
						fmt.Fprint(c, "235 ok\r\n")
					case line == "DATA":
						inData = true
						fmt.Fprint(c, "354 go\r\n")
					case line == "QUIT":
						fmt.Fprint(c, "221 bye\r\n")
						return
					default:
						fmt.Fprint(c, "250 ok\r\n")
					}
				}
			}(conn)
		}
	}()
	_, port, _ = net.SplitHostPort(ln.Addr().String())
	return port, func() []string { mu.Lock(); defer mu.Unlock(); return append([]string{}, got...) }
}

// "Send test email" must test what's on screen, even before Save.
func TestSMTPTestUsesOnScreenValues(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	port, passwords := fakeSMTP(t)
	admin.do("PUT", "/api/admin/settings", map[string]string{"smtp_host": "127.0.0.1", "smtp_port": port,
		"smtp_username": "me@example.com", "smtp_password": "old-saved", "smtp_from": "me@example.com"}, true)

	res, out := admin.do("POST", "/api/admin/smtp-test", map[string]string{"to": "kid@kindle.com", "smtp_password": "typed-not-saved"}, true)
	if res.StatusCode != 200 {
		t.Fatalf("test email: %d %v", res.StatusCode, out)
	}
	admin.do("POST", "/api/admin/smtp-test", map[string]string{"to": "kid@kindle.com", "smtp_password": "********"}, true)
	if got := passwords(); len(got) != 2 || got[0] != "typed-not-saved" || got[1] != "old-saved" {
		t.Fatalf("passwords used: %v", got)
	}
}
