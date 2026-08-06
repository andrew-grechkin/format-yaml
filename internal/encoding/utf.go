// BOM-driven UTF transcoding at the pipeline's byte gateway. Files written in UTF-16 or UTF-32 are converted to UTF-8
// in one place so every downstream pass can stay UTF-8-only. Byte order marks are removed on the way in and never
// re-emitted - the output side always emits UTF-8 without a BOM.
package encoding

import (
	"bytes"
	"errors"
	"strings"
	"unicode/utf16"
)

// Well-known byte-order marks. Order-of-check matters at call sites: UTF-32 LE starts with the UTF-16 LE prefix, so the
// longer marker must be tested first.
var (
	BOMUTF8    = []byte{0xEF, 0xBB, 0xBF}
	BOMUTF16BE = []byte{0xFE, 0xFF}
	BOMUTF16LE = []byte{0xFF, 0xFE}
	BOMUTF32BE = []byte{0x00, 0x00, 0xFE, 0xFF}
	BOMUTF32LE = []byte{0xFF, 0xFE, 0x00, 0x00}
)

// Detects an encoding-signaling BOM and, when the input isn't UTF-8, transcodes the payload to UTF-8 on the fly so the
// rest of the pipeline can stay UTF-8-only. Output is always UTF-8 - a file that arrived as UTF-16 leaves as UTF-8,
// which is the modern default and what most YAML consumers expect anyway.
func NormalizeToUTF8(src []byte) ([]byte, error) {
	switch {
	case bytes.HasPrefix(src, BOMUTF32BE):
		return decodeUTF32(src[len(BOMUTF32BE):], true)
	case bytes.HasPrefix(src, BOMUTF32LE):
		return decodeUTF32(src[len(BOMUTF32LE):], false)
	case bytes.HasPrefix(src, BOMUTF16BE):
		return decodeUTF16(src[len(BOMUTF16BE):], true)
	case bytes.HasPrefix(src, BOMUTF16LE):
		return decodeUTF16(src[len(BOMUTF16LE):], false)
	}
	return bytes.TrimPrefix(src, BOMUTF8), nil
}

func decodeUTF16(src []byte, bigEndian bool) ([]byte, error) {
	if len(src)%2 != 0 {
		return nil, errors.New("UTF-16 input has odd byte length")
	}
	units := make([]uint16, len(src)/2)
	for i := range units {
		hi, lo := src[2*i], src[2*i+1]
		if bigEndian {
			units[i] = uint16(hi)<<8 | uint16(lo)
		} else {
			units[i] = uint16(lo)<<8 | uint16(hi)
		}
	}
	return []byte(string(utf16.Decode(units))), nil
}

func decodeUTF32(src []byte, bigEndian bool) ([]byte, error) {
	if len(src)%4 != 0 {
		return nil, errors.New("UTF-32 input length is not a multiple of 4")
	}
	var sb strings.Builder
	sb.Grow(len(src))
	for i := 0; i < len(src); i += 4 {
		var r rune
		if bigEndian {
			r = rune(src[i])<<24 | rune(src[i+1])<<16 | rune(src[i+2])<<8 | rune(src[i+3])
		} else {
			r = rune(src[i+3])<<24 | rune(src[i+2])<<16 | rune(src[i+1])<<8 | rune(src[i])
		}
		sb.WriteRune(r)
	}
	return []byte(sb.String()), nil
}
