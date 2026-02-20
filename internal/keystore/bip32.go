package keystore

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

const hardenedBit = 0x80000000

// HDKey represents a hierarchical deterministic key.
type HDKey struct {
	Key       [32]byte
	ChainCode [32]byte
	Depth     uint8
}

// deriveMasterKeyFromBytes derives the master key from a seed of any length using SLIP-0010.
// Uses HMAC-SHA512 with key "ed25519 seed".
func deriveMasterKeyFromBytes(seed []byte) HDKey {
	mac := hmac.New(sha512.New, []byte("ed25519 seed"))
	mac.Write(seed)
	sum := mac.Sum(nil) // 64 bytes

	var hd HDKey
	copy(hd.Key[:], sum[:32])
	copy(hd.ChainCode[:], sum[32:])
	hd.Depth = 0
	return hd
}

// DeriveMasterKey derives the master key from a BIP39 seed using SLIP-0010.
// Uses HMAC-SHA512 with key "ed25519 seed".
func DeriveMasterKey(seed [64]byte) HDKey {
	return deriveMasterKeyFromBytes(seed[:])
}

// DeriveChildKey derives a child key from a parent using hardened derivation.
// Ed25519 only supports hardened keys (index must have bit 31 set).
// If the hardened bit is not set, it is added automatically.
func DeriveChildKey(parent HDKey, index uint32) (HDKey, error) {
	// Ensure hardened bit is set
	index |= hardenedBit

	// data = 0x00 || parent.Key (32 bytes) || index (4 bytes big-endian) = 37 bytes
	data := make([]byte, 37)
	data[0] = 0x00
	copy(data[1:33], parent.Key[:])
	binary.BigEndian.PutUint32(data[33:37], index)

	mac := hmac.New(sha512.New, parent.ChainCode[:])
	mac.Write(data)
	sum := mac.Sum(nil) // 64 bytes

	var child HDKey
	copy(child.Key[:], sum[:32])
	copy(child.ChainCode[:], sum[32:])
	child.Depth = parent.Depth + 1
	return child, nil
}

// ParseDerivationPath parses a path like "m/44'/501'/0'/0'" into indices.
// All indices must be hardened (end with ' or h). Returns raw indices WITHOUT the hardened bit.
func ParseDerivationPath(path string) ([]uint32, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("empty derivation path")
	}

	parts := strings.Split(path, "/")
	if parts[0] != "m" {
		return nil, fmt.Errorf("derivation path must start with 'm', got %q", parts[0])
	}

	indices := make([]uint32, 0, len(parts)-1)
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}

		// Strip hardened suffix
		hardened := false
		if strings.HasSuffix(part, "'") || strings.HasSuffix(part, "h") {
			hardened = true
			part = part[:len(part)-1]
		}

		if !hardened {
			return nil, fmt.Errorf("Ed25519 requires all indices to be hardened: %q", part)
		}

		val, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid index %q: %w", part, err)
		}

		indices = append(indices, uint32(val))
	}

	return indices, nil
}

// DeriveFromPath derives a key from seed following the given derivation path.
func DeriveFromPath(seed [64]byte, path string) (HDKey, error) {
	indices, err := ParseDerivationPath(path)
	if err != nil {
		return HDKey{}, err
	}

	key := DeriveMasterKey(seed)
	for _, idx := range indices {
		key, err = DeriveChildKey(key, idx)
		if err != nil {
			return HDKey{}, err
		}
	}

	return key, nil
}
