// Package kindleapp embeds the NovelCheck Kindle app (for a jailbroken Kindle without KOReader), so NovelCheck can
// hand it out already set up for the person who downloads it. Unzipped onto the Kindle, it puts NovelCheck.sh (a
// "book" with the NovelCheck cover that opens the app) and NovelCheck-Check.sh (a check that reports what the app
// needs, for when it doesn't work) in documents, and the app itself in extensions.
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

//go:embed app NovelCheck.sh NovelCheck-Check.sh cover.png
var files embed.FS

// Version is the app's version. Raise it whenever a file in app/ changes: the app compares it with its own and
// offers to update (Files and File below are what it fetches).
const Version = "1.2.1"

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
func Zip(server, user, code string) ([]byte, error) {
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
	sum := md5.Sum([]byte(code)) // the plugin's sign-in sends the sync code's MD5, never the code
	cfg, _ := json.Marshal(map[string]string{"server": strings.TrimRight(server, "/"), "user": user,
		"key": hex.EncodeToString(sum[:]), "device": "Kindle", "version": Version})
	if err := add("extensions/novelcheck-app/config.js", []byte("// Made by NovelCheck for this account.\nvar NC = "+string(cfg)+";\n")); err != nil {
		return nil, err
	}
	script, _ := files.ReadFile("NovelCheck.sh")
	icon, _ := files.ReadFile("cover.png")
	script = bytes.Replace(script, []byte("{{ICON}}"), []byte(base64.StdEncoding.EncodeToString(icon)), 1)
	if err := add("documents/NovelCheck.sh", script); err != nil {
		return nil, err
	}
	check, _ := files.ReadFile("NovelCheck-Check.sh")
	check = []byte(strings.NewReplacer("{{SERVER}}", shellQuote(strings.TrimRight(server, "/")), "{{USER}}", shellQuote(user),
		"{{KEY}}", shellQuote(hex.EncodeToString(sum[:])), "{{VERSION}}", Version).Replace(string(check)))
	if err := add("documents/NovelCheck-Check.sh", check); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// shellQuote quotes s as one word for the Kindle's shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
