// Package kindleapp embeds the NovelCheck Kindle app (for a jailbroken Kindle without KOReader), so NovelCheck can
// hand it out already set up for the person who downloads it. Unzipped onto the Kindle, it puts NovelCheck.sh (a
// "book" with the NovelCheck cover that opens the app) in documents and the app itself in extensions.
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

// Version is the app's version, shown on the download.
const Version = "1.0.0"

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
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
