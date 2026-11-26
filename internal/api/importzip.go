package api

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// unzipSafely unpacks a zip into dir: no path may leave dir, no more than maxUnzipEntries files and maxUnzipped bytes in all
// (a zip bomb stops at the limit), and links are not made.
func unzipSafely(src, dir string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	if len(zr.File) > maxUnzipEntries {
		return errors.New("too many files in the zip")
	}
	var total int64
	for _, f := range zr.File {
		target := filepath.Join(dir, f.Name)
		if rel, err := filepath.Rel(dir, target); err != nil || strings.HasPrefix(rel, "..") {
			return errors.New("the zip has a path outside its folder")
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		n, err := io.Copy(out, io.LimitReader(rc, maxUnzipped-total+1))
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
		if total += n; total > maxUnzipped {
			return errors.New("the zip unpacks to more than 2 GB")
		}
	}
	return nil
}
