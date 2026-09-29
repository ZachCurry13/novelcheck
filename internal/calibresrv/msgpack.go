package calibresrv

import (
	"encoding/binary"
	"fmt"
	"math"
)

// msgpack writes the few MessagePack types calibre's command interface needs
// when a command carries file data (add_format): arrays, strings, bytes,
// integers, booleans and nil. JSON can't carry the file's bytes.
func msgpack(v any) ([]byte, error) {
	var out []byte
	var enc func(v any) error
	head := func(small, b8, b16, b32 byte, n int) {
		switch {
		case small != 0 && n < 16:
			out = append(out, small|byte(n))
		case small != 0 && small == 0xa0 && n < 32:
			out = append(out, small|byte(n))
		case b8 != 0 && n <= math.MaxUint8:
			out = append(out, b8, byte(n))
		case n <= math.MaxUint16:
			out = append(out, b16)
			out = binary.BigEndian.AppendUint16(out, uint16(n))
		default:
			out = append(out, b32)
			out = binary.BigEndian.AppendUint32(out, uint32(n))
		}
	}
	enc = func(v any) error {
		switch x := v.(type) {
		case nil:
			out = append(out, 0xc0)
		case bool:
			out = append(out, map[bool]byte{false: 0xc2, true: 0xc3}[x])
		case int:
			if x < 0 {
				out = append(out, 0xd3)
				out = binary.BigEndian.AppendUint64(out, uint64(int64(x)))
			} else if x < 128 {
				out = append(out, byte(x))
			} else {
				out = append(out, 0xcf)
				out = binary.BigEndian.AppendUint64(out, uint64(x))
			}
		case string:
			head(0xa0, 0xd9, 0xda, 0xdb, len(x))
			out = append(out, x...)
		case []byte:
			head(0, 0xc4, 0xc5, 0xc6, len(x))
			out = append(out, x...)
		case []any:
			head(0x90, 0, 0xdc, 0xdd, len(x))
			for _, e := range x {
				if err := enc(e); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("msgpack: can't write %T", v)
		}
		return nil
	}
	err := enc(v)
	return out, err
}
