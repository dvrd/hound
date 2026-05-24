package transaction

import (
	"bytes"
	"testing"
)

func TestCompactU16_Encode(t *testing.T) {
	tests := []struct {
		name  string
		value uint16
		want  []byte
	}{
		{"zero", 0, []byte{0x00}},
		{"one", 1, []byte{0x01}},
		{"127", 127, []byte{0x7f}},
		{"128", 128, []byte{0x80, 0x01}},
		{"255", 255, []byte{0xff, 0x01}},
		{"16383", 16383, []byte{0xff, 0x7f}},
		{"16384", 16384, []byte{0x80, 0x80, 0x01}},
		{"65535", 65535, []byte{0xff, 0xff, 0x03}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AppendCompactU16(nil, tt.value)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("AppendCompactU16(%d) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestCompactU16_RoundTrip(t *testing.T) {
	values := []uint16{0, 1, 127, 128, 255, 16383, 16384, 65535}
	for _, v := range values {
		encoded := AppendCompactU16(nil, v)
		decoded, n, err := DecodeCompactU16(encoded)
		if err != nil {
			t.Errorf("DecodeCompactU16(AppendCompactU16(%d)) error: %v", v, err)
			continue
		}
		if decoded != v {
			t.Errorf("round-trip %d: got %d", v, decoded)
		}
		if n != len(encoded) {
			t.Errorf("round-trip %d: consumed %d bytes, encoded %d bytes", v, n, len(encoded))
		}
	}
}

func TestCompactU16_DecodeEmpty(t *testing.T) {
	_, _, err := DecodeCompactU16([]byte{})
	if err == nil {
		t.Error("DecodeCompactU16(empty) should return error")
	}
}
