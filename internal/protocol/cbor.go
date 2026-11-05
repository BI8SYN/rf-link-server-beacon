package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"
)

// Encode is deliberately limited to the deterministic FMO signing subset.
func Encode(v any) []byte { var b bytes.Buffer; encode(&b, v); return b.Bytes() }
func head(b *bytes.Buffer, major byte, n uint64) {
	switch {
	case n < 24:
		b.WriteByte(major<<5 | byte(n))
	case n <= 255:
		b.WriteByte(major<<5 | 24)
		b.WriteByte(byte(n))
	case n <= 65535:
		b.WriteByte(major<<5 | 25)
		binary.Write(b, binary.BigEndian, uint16(n))
	case n <= 0xffffffff:
		b.WriteByte(major<<5 | 26)
		binary.Write(b, binary.BigEndian, uint32(n))
	default:
		b.WriteByte(major<<5 | 27)
		binary.Write(b, binary.BigEndian, n)
	}
}
func encode(b *bytes.Buffer, v any) {
	switch x := v.(type) {
	case string:
		head(b, 3, uint64(len(x)))
		b.WriteString(x)
	case []byte:
		head(b, 2, uint64(len(x)))
		b.Write(x)
	case uint64:
		head(b, 0, x)
	case int:
		if x < 0 {
			panic("negative FMO integer")
		}
		head(b, 0, uint64(x))
	case bool:
		if x {
			b.WriteByte(245)
		} else {
			b.WriteByte(244)
		}
	case []any:
		head(b, 4, uint64(len(x)))
		for _, e := range x {
			encode(b, e)
		}
	default:
		panic(fmt.Sprintf("unsupported CBOR type %T", v))
	}
}
func decode(data []byte) (any, error) {
	pos := 0
	v, e := item(data, &pos, 0)
	if e != nil {
		return nil, e
	}
	if pos != len(data) || !bytes.Equal(Encode(v), data) {
		return nil, errors.New("noncanonical CBOR")
	}
	return v, nil
}
func item(data []byte, p *int, depth int) (any, error) {
	if depth > 12 || *p >= len(data) {
		return nil, errors.New("truncated CBOR")
	}
	first := data[*p]
	*p++
	major, ai := first>>5, first&31
	var n uint64
	if ai < 24 {
		n = uint64(ai)
	} else {
		size := 0
		switch ai {
		case 24:
			size = 1
		case 25:
			size = 2
		case 26:
			size = 4
		case 27:
			size = 8
		default:
			return nil, errors.New("indefinite CBOR")
		}
		if *p+size > len(data) {
			return nil, errors.New("truncated CBOR")
		}
		for _, c := range data[*p : *p+size] {
			n = n<<8 | uint64(c)
		}
		*p += size
	}
	switch major {
	case 0:
		if n > MaxInteger {
			return nil, errors.New("unsafe integer")
		}
		return n, nil
	case 2, 3:
		if n > uint64(len(data)-*p) {
			return nil, errors.New("truncated CBOR string")
		}
		b := data[*p : *p+int(n)]
		*p += int(n)
		if major == 2 {
			return b, nil
		}
		if !utf8.Valid(b) {
			return nil, errors.New("invalid UTF-8")
		}
		return string(b), nil
	case 4:
		if n > 128 {
			return nil, errors.New("CBOR array limit")
		}
		a := make([]any, int(n))
		for i := range a {
			var e error
			a[i], e = item(data, p, depth+1)
			if e != nil {
				return nil, e
			}
		}
		return a, nil
	default:
		return nil, errors.New("unsupported CBOR")
	}
}
