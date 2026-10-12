// Package kindleplugin embeds the KOReader plugin, so NovelCheck can hand it
// out already set up for the person who downloads it.
package kindleplugin

import (
	"archive/zip"
	"bytes"
	"embed"
	"io/fs"
	"regexp"
	"strings"
)

//go:embed all:novelcheck.koplugin
var files embed.FS

const folder = "novelcheck.koplugin"

var versionRE = regexp.MustCompile(`version\s*=\s*"([^"]+)"`)

// Version is the plugin's version (from its _meta.lua).
func Version() string {
	b, _ := files.ReadFile(folder + "/_meta.lua")
	if m := versionRE.FindSubmatch(b); m != nil {
		return string(m[1])
	}
	return ""
}

// luaString quotes s as a Lua string literal.
func luaString(s string) string {
	r := strings.NewReplacer(`\`, `\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\x00", "")
	return `"` + r.Replace(s) + `"`
}

// Zip is the plugin folder as a zip, with a config.lua that connects it to
// server as user with the sync code (the plugin reads it on first start).
func Zip(server, user, code string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	err := fs.WalkDir(files, folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := files.ReadFile(path)
		if err != nil {
			return err
		}
		w, err := zw.Create(path)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		return nil, err
	}
	w, err := zw.Create(folder + "/config.lua")
	if err != nil {
		return nil, err
	}
	cfg := "-- Made by NovelCheck for this account: the plugin connects with it on first start.\nreturn {\n" +
		"    server = " + luaString(server) + ",\n    user = " + luaString(user) + ",\n    code = " + luaString(code) + ",\n}\n"
	if _, err := w.Write([]byte(cfg)); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
