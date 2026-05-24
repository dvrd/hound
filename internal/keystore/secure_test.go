package keystore_test

import (
	"strings"
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

func TestJoinWordsToBytesMatchesStringsJoin(t *testing.T) {
	tests := []struct {
		name  string
		words []string
	}{
		{"12 words", strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")},
		{"single word", []string{"hello"}},
		{"two words", []string{"foo", "bar"}},
		{"empty", []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expected := strings.Join(tc.words, " ")
			got := keystore.JoinWordsToBytes(tc.words)
			if string(got) != expected {
				t.Errorf("JoinWordsToBytes = %q, want %q", string(got), expected)
			}
		})
	}
}

func TestJoinWordsToBytesIsMutable(t *testing.T) {
	words := []string{"abandon", "abandon", "about"}
	buf := keystore.JoinWordsToBytes(words)

	// Verify it's non-empty
	if len(buf) == 0 {
		t.Fatal("JoinWordsToBytes returned empty slice")
	}

	// Zero it — this is the whole point
	keystore.ZeroBytes(buf)
	for i, v := range buf {
		if v != 0 {
			t.Errorf("after ZeroBytes: byte[%d] = 0x%02X, want 0x00", i, v)
		}
	}
}

func TestJoinWordsToBytesEmpty(t *testing.T) {
	got := keystore.JoinWordsToBytes([]string{})
	if len(got) != 0 {
		t.Errorf("JoinWordsToBytes([]) length = %d, want 0", len(got))
	}
}

func TestJoinWordsToBytesNil(t *testing.T) {
	got := keystore.JoinWordsToBytes(nil)
	if len(got) != 0 {
		t.Errorf("JoinWordsToBytes(nil) length = %d, want 0", len(got))
	}
}
