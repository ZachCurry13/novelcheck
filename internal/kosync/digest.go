// Package kosync holds what NovelCheck needs to be a KOReader progress-sync
// server: KOReader names each book by a fingerprint of its file, which
// NovelCheck works out for the files it has to know which book it is.
package kosync

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
)

// PartialMD5 is KOReader's fingerprint of a file ("binary" document
// matching, its default): the MD5 of 1 KB read at offset 0 and at 1024·4^i
// for i = 0…10, stopping at the end of the file.
func PartialMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	buf := make([]byte, 1024)
	for i := -1; i <= 10; i++ {
		off := int64(0)
		if i >= 0 {
			off = 1024 << (2 * i)
		}
		n, err := f.ReadAt(buf, off)
		if n == 0 {
			break
		}
		h.Write(buf[:n])
		if err != nil && err != io.EOF {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
