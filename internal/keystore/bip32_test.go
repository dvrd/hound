package keystore

import (
	"encoding/hex"
	"testing"
)

func TestDeriveMasterKeySLIP0010Vector(t *testing.T) {
	// SLIP-0010 test vector for Ed25519
	// The test vector uses a 16-byte seed, not a 64-byte BIP39 seed.
	seedHex := "000102030405060708090a0b0c0d0e0f"
	seedBytes, err := hex.DecodeString(seedHex)
	if err != nil {
		t.Fatalf("hex.DecodeString() error: %v", err)
	}

	// Use the internal deriveMasterKeyFromBytes which accepts variable-length seeds
	hd := deriveMasterKeyFromBytes(seedBytes)

	expectedKeyHex := "2b4be7f19ee27bbf30c667b642d5f4aa69fd169872f8fc3059c08ebae2eb19e7"
	expectedChainHex := "90046a93de5380a72b5e45010748567d5ea02bbf6522f979e05c0d8d8ca9fffb"

	gotKeyHex := hex.EncodeToString(hd.Key[:])
	gotChainHex := hex.EncodeToString(hd.ChainCode[:])

	if gotKeyHex != expectedKeyHex {
		t.Errorf("master key =\n  %s\nwant:\n  %s", gotKeyHex, expectedKeyHex)
	}
	if gotChainHex != expectedChainHex {
		t.Errorf("master chain code =\n  %s\nwant:\n  %s", gotChainHex, expectedChainHex)
	}
	if hd.Depth != 0 {
		t.Errorf("master depth = %d, want 0", hd.Depth)
	}
}

func TestDeriveMasterKeyBIP39Seed(t *testing.T) {
	// Verify DeriveMasterKey works with a full 64-byte seed
	var seed [64]byte
	seed[0] = 0x01
	seed[63] = 0xFF

	hd := DeriveMasterKey(seed)
	if hd.Depth != 0 {
		t.Errorf("master depth = %d, want 0", hd.Depth)
	}

	// Key should be non-zero
	allZero := true
	for _, b := range hd.Key {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("master key is all zeros")
	}
}

func TestDeriveChildKeyDepth(t *testing.T) {
	var seed [64]byte
	seed[0] = 0x01

	master := DeriveMasterKey(seed)
	if master.Depth != 0 {
		t.Fatalf("master depth = %d, want 0", master.Depth)
	}

	child, err := DeriveChildKey(master, 44)
	if err != nil {
		t.Fatalf("DeriveChildKey() error: %v", err)
	}
	if child.Depth != 1 {
		t.Errorf("child depth = %d, want 1", child.Depth)
	}

	grandchild, err := DeriveChildKey(child, 501)
	if err != nil {
		t.Fatalf("DeriveChildKey() error: %v", err)
	}
	if grandchild.Depth != 2 {
		t.Errorf("grandchild depth = %d, want 2", grandchild.Depth)
	}
}

func TestParseDerivationPath(t *testing.T) {
	tests := []struct {
		path    string
		want    []uint32
		wantErr bool
	}{
		{"m/44'/501'/0'/0'", []uint32{44, 501, 0, 0}, false},
		{"m/44'/501'/1'/0'", []uint32{44, 501, 1, 0}, false},
		{"m/44'/501'", []uint32{44, 501}, false},
		{"m/44h/501h/0h", []uint32{44, 501, 0}, false},
		{"", nil, true},           // empty
		{"44'/501'", nil, true},   // missing m
		{"m/44/501", nil, true},   // not hardened
		{"m/44'/abc'", nil, true}, // invalid number
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, err := ParseDerivationPath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseDerivationPath(%q) should return error", tt.path)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDerivationPath(%q) error: %v", tt.path, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseDerivationPath(%q) returned %d indices, want %d", tt.path, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index[%d] = %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDeriveFromPath(t *testing.T) {
	// Use SLIP-0010 test vector seed (16 bytes, zero-padded to 64 for the public API)
	// Note: This won't match the SLIP-0010 test vector exactly because the seed is
	// zero-padded, but it verifies the derivation chain works correctly.
	var seed [64]byte
	seed[0] = 0xAB

	// Derive m/0' and verify depth
	hd, err := DeriveFromPath(seed, "m/0'")
	if err != nil {
		t.Fatalf("DeriveFromPath() error: %v", err)
	}
	if hd.Depth != 1 {
		t.Errorf("depth = %d, want 1", hd.Depth)
	}

	// Key should be 32 bytes and non-zero
	allZero := true
	for _, b := range hd.Key {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("derived key is all zeros")
	}
}

func TestDeriveFromPathInvalid(t *testing.T) {
	var seed [64]byte
	_, err := DeriveFromPath(seed, "invalid")
	if err == nil {
		t.Error("DeriveFromPath(invalid) should return error")
	}
}

func TestDeriveFromPathSolana(t *testing.T) {
	// Standard Solana BIP44 path: m/44'/501'/0'/0'
	var seed [64]byte
	seed[0] = 0xAB

	hd, err := DeriveFromPath(seed, "m/44'/501'/0'/0'")
	if err != nil {
		t.Fatalf("DeriveFromPath() error: %v", err)
	}
	if hd.Depth != 4 {
		t.Errorf("depth = %d, want 4", hd.Depth)
	}
}

func TestDeriveChildKeySLIP0010Vector(t *testing.T) {
	// Full SLIP-0010 test vector: derive m/0' from the 16-byte seed
	seedHex := "000102030405060708090a0b0c0d0e0f"
	seedBytes, _ := hex.DecodeString(seedHex)

	master := deriveMasterKeyFromBytes(seedBytes)

	// SLIP-0010 test vector for m/0'
	child, err := DeriveChildKey(master, 0) // hardened bit added automatically
	if err != nil {
		t.Fatalf("DeriveChildKey() error: %v", err)
	}

	expectedKeyHex := "68e0fe46dfb67e368c75379acec591dad19df3cde26e63b93a8e704f1dade7a3"
	expectedChainHex := "8b59aa11380b624e81507a27fedda59fea6d0b779a778918a2fd3590e16e9c69"

	gotKeyHex := hex.EncodeToString(child.Key[:])
	gotChainHex := hex.EncodeToString(child.ChainCode[:])

	if gotKeyHex != expectedKeyHex {
		t.Errorf("child key m/0' =\n  %s\nwant:\n  %s", gotKeyHex, expectedKeyHex)
	}
	if gotChainHex != expectedChainHex {
		t.Errorf("child chain code m/0' =\n  %s\nwant:\n  %s", gotChainHex, expectedChainHex)
	}
}
