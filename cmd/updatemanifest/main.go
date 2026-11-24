// updatemanifest writes what a GitHub release carries so the Kindle app and the KOReader plugin can update
// themselves from GitHub, without waiting for the person to update their NovelCheck server:
//
//	kindle-app.json / kindle-plugin.json   version, the oldest server they work with, the download address, the files
//	kindle-app-<file> / kindle-plugin-<file>   each file, as a release download
//
// Run by the release workflow: go run ./cmd/updatemanifest -out dist -base https://github.com/<repo>/releases/download/<tag>/
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"

	kindleapp "github.com/zachcurry13/novelcheck/kindle-app"
	kindleplugin "github.com/zachcurry13/novelcheck/kindle-plugin"
)

type manifest struct {
	Version   string   `json:"version"`
	MinServer string   `json:"min_server"`
	Base      string   `json:"base"`
	Prefix    string   `json:"prefix"`
	Files     []string `json:"files"`
}

func write(out, base, kind, version, minServer string, names []string, file func(string) ([]byte, bool)) {
	m := manifest{Version: version, MinServer: minServer, Base: base, Prefix: kind + "-", Files: names}
	for _, n := range names {
		data, ok := file(n)
		if !ok {
			log.Fatalf("%s: no file %s", kind, n)
		}
		if err := os.WriteFile(filepath.Join(out, m.Prefix+n), data, 0o644); err != nil {
			log.Fatal(err)
		}
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(out, kind+".json"), b, 0o644); err != nil {
		log.Fatal(err)
	}
}

func main() {
	out := flag.String("out", "dist", "folder for the release files")
	base := flag.String("base", "", "where the release's files can be downloaded (ends with /)")
	flag.Parse()
	if *base == "" || (*base)[len(*base)-1] != '/' {
		log.Fatal("-base must be the release's download address ending with /")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	write(*out, *base, "kindle-app", kindleapp.Version, kindleapp.MinServer, kindleapp.Files(), kindleapp.File)
	// the launcher "book" (not an app file: the app never updates it), for the KindleForge package's install.sh
	if err := os.WriteFile(filepath.Join(*out, "kindle-app-NovelCheck.sh"), kindleapp.Launcher(), 0o644); err != nil {
		log.Fatal(err)
	}
	write(*out, *base, "kindle-plugin", kindleplugin.Version(), kindleplugin.MinServer, kindleplugin.UpdatableFiles(), kindleplugin.File)
}
