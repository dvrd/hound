package transaction

import "fmt"

// AppendCompactU16 appends a compact-u16 encoded value directly to dst.
func AppendCompactU16(dst []byte, value uint16) []byte {
	var buf [3]byte
	n := putCompactU16(buf[:], value)
	return append(dst, buf[:n]...)
}

// putCompactU16 writes the compact-u16 encoding into buf and returns bytes written.
func putCompactU16(buf []byte, value uint16) int {
	val := value
	i := 0
	for {
		b := byte(val & 0x7f)
		val >>= 7
		if val > 0 {
			b |= 0x80
		}
		buf[i] = b
		i++
		if val == 0 {
			break
		}
	}
	return i
}

// DecodeCompactU16 decodes a compact-u16 encoded value from data.
// Returns the decoded value, number of bytes consumed, and any error.
func DecodeCompactU16(data []byte) (uint16, int, error) {
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("compact-u16: empty input")
	}

	var val uint16
	for i := 0; i < 3; i++ {
		if i >= len(data) {
			return 0, 0, fmt.Errorf("compact-u16: unexpected end of input")
		}
		b := data[i]
		val |= uint16(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return val, i + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("compact-u16: value too large")
}
