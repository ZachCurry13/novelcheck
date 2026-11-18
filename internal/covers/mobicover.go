package covers

import (
	"encoding/binary"
	"io"
	"os"
)

// mobiCover reads the cover picture of a MOBI/AZW/AZW3 file (a Palm database): record 0 holds the MOBI header,
// which gives the first picture's record, and its EXTH part says which picture is the cover (type 201), or the
// small one (202) when there is no cover. Pictures are never locked, even in a book whose text is.
func mobiCover(file string) []byte {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	head := make([]byte, 78)
	if _, err := io.ReadFull(f, head); err != nil {
		return nil
	}
	n := int(binary.BigEndian.Uint16(head[76:78]))
	if n < 2 || n > 100000 {
		return nil
	}
	list := make([]byte, n*8)
	if _, err := io.ReadFull(f, list); err != nil {
		return nil
	}
	info, err := f.Stat()
	if err != nil {
		return nil
	}
	start := func(i int) int64 {
		if i >= n {
			return info.Size()
		}
		return int64(binary.BigEndian.Uint32(list[i*8:]))
	}
	record := func(i int) []byte {
		a, b := start(i), start(i+1)
		if i < 0 || i >= n || a <= 0 || b <= a || b-a > 8<<20 || b > info.Size() {
			return nil
		}
		buf := make([]byte, b-a)
		if _, err := f.ReadAt(buf, a); err != nil {
			return nil
		}
		return buf
	}
	r0 := record(0)
	if len(r0) < 0x84 || string(r0[16:20]) != "MOBI" {
		return nil
	}
	mobiLen := int(binary.BigEndian.Uint32(r0[20:24]))
	firstImage := int(binary.BigEndian.Uint32(r0[0x6C:0x70]))
	if binary.BigEndian.Uint32(r0[0x80:0x84])&0x40 == 0 || firstImage <= 0 || firstImage >= n {
		return nil
	}
	exth := 16 + mobiLen
	if exth+12 > len(r0) || string(r0[exth:exth+4]) != "EXTH" {
		return nil
	}
	count := int(binary.BigEndian.Uint32(r0[exth+8 : exth+12]))
	cover, thumb := -1, -1
	for p, k := exth+12, 0; k < count && p+8 <= len(r0); k++ {
		typ, size := binary.BigEndian.Uint32(r0[p:]), int(binary.BigEndian.Uint32(r0[p+4:]))
		if size < 8 || p+size > len(r0) {
			break
		}
		if size >= 12 && (typ == 201 || typ == 202) {
			v := int(binary.BigEndian.Uint32(r0[p+8:]))
			if typ == 201 {
				cover = v
			} else {
				thumb = v
			}
		}
		p += size
	}
	if cover < 0 {
		cover = thumb
	}
	if cover < 0 || cover == 0xFFFFFFFF {
		return nil
	}
	return record(firstImage + cover)
}
