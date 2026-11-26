// Package kindleapp embeds the NovelCheck Kindle app (for a jailbroken Kindle without KOReader), so NovelCheck can
// hand it out already set up for the person who downloads it. Unzipped onto the Kindle, it puts NovelCheck.sh (a
// "book" with the NovelCheck cover that opens the app) in documents, and the app itself in extensions (with
// check.sh, which the app's "Send a check" runs). Nothing else goes in documents: it would show as a book.
package kindleapp

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
)

//go:embed app NovelCheck.sh cover.png
var files embed.FS

// Version is the app's version. Raise it whenever a file in app/ changes: the app compares it with its own and
// offers to update (Files and File below are what it fetches).
const Version = "1.23.0"

// MinServer is the oldest NovelCheck server this app works with. The app also updates from GitHub, where it can be newer
// than the person's server: it only takes such an update when the server is at least this version. Raise it when an app
// change needs a server change.
const MinServer = "1.99.3"

// Files are the app's files the app can fetch to update itself (never config.js, which is per account).
func Files() []string {
	entries, _ := files.ReadDir("app")
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// File is one of Files' contents.
func File(name string) ([]byte, bool) {
	for _, n := range Files() {
		if n == name {
			b, err := files.ReadFile("app/" + n)
			return b, err == nil
		}
	}
	return nil, false
}

// Zip is the app as a zip to unzip onto the Kindle's main folder, connected to server as user with the sync code.
func Zip(server, user, code, name string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name string, data []byte) error {
		w, err := zw.Create(name)
		if err == nil {
			_, err = w.Write(data)
		}
		return err
	}
	entries, err := files.ReadDir("app")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		data, err := files.ReadFile("app/" + e.Name())
		if err != nil {
			return nil, err
		}
		if err := add("extensions/novelcheck-app/"+e.Name(), data); err != nil {
			return nil, err
		}
	}
	if err := add("extensions/novelcheck-app/config.js", Config(server, user, code, name)); err != nil {
		return nil, err
	}
	if err := add("documents/NovelCheck.sh", Launcher()); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// deviceName is what the Kindle's reports are called: the name given at download ("Zach Paperwhite") with
// " (Amazon reader)", so NovelCheck can tell it from the KOReader plugin's report of the same Kindle. No name: "Kindle".
func deviceName(name string) string {
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 || r == '/' || r == '\\' || r == '"' {
			return ' '
		}
		return r
	}, name))
	if len([]rune(name)) > 40 {
		name = string([]rune(name)[:40])
	}
	if name == "" {
		return "Kindle"
	}
	return name
}

// Config is config.js: this account's sign-in (the plugin's sign-in sends the sync code's MD5, never the code).
func Config(server, user, code, name string) []byte {
	sum := md5.Sum([]byte(code))
	cfg, _ := json.Marshal(map[string]string{"server": strings.TrimRight(server, "/"), "user": user,
		"key": hex.EncodeToString(sum[:]), "device": deviceName(name), "version": Version})
	return []byte("// Made by NovelCheck for this account.\nvar NC = " + string(cfg) + ";\n")
}

// Launcher is documents/NovelCheck.sh with its icon.
func Launcher() []byte {
	script, _ := files.ReadFile("NovelCheck.sh")
	icon, _ := files.ReadFile("cover.png")
	return bytes.Replace(script, []byte("{{ICON}}"), []byte(base64.StdEncoding.EncodeToString(icon)), 1)
}
