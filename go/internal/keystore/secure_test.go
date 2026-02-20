package keystore_test

import (
	"testing"

	"github.com/dvrd/hound/internal/keystore"
)

func TestZeroBytes(t *testing.T) {
	b := []byte{0xFF, 0xAB, 0x12, 0x34, 0x56}
	keystore.ZeroBytes(b)
	for i, v := range b {
		if v != 0 {
			t.Errorf("ZeroBytes: byte[%d] = 0x%02X, want 0x00", i, v)
		}
	}
}

func TestZeroBytesEmpty(t *testing.T) {
	b := []byte{}
	keystore.ZeroBytes(b) // should not panic
}

func TestZeroBytesNil(t *testing.T) {
	keystore.ZeroBytes(nil) // should not panic
}

func TestZeroSlice(t *testing.T) {
	b := []byte{0x01, 0x02, 0x03, 0x04}
	keystore.ZeroSlice(b)
	for i, v := range b {
		if v != 0 {
			t.Errorf("ZeroSlice: byte[%d] = 0x%02X, want 0x00", i, v)
		}
	}
}

func TestZeroBytesLargeSlice(t *testing.T) {
	b := make([]byte, 1024)
	for i := range b {
		b[i] = 0xFF
	}
	keystore.ZeroBytes(b)
	for i, v := range b {
		if v != 0 {
			t.Errorf("ZeroBytes: byte[%d] = 0x%02X, want 0x00", i, v)
		}
	}
}
