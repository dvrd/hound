package transaction

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestTransaction_SignAndVerify(t *testing.T) {
	// Generate deterministic keypair
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)

	var fromPubkey Pubkey
	copy(fromPubkey[:], pubKey)

	recipient, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	var blockhash Pubkey

	ix := SystemTransfer(fromPubkey, recipient, 1_000_000_000)
	msg := NewMessage(fromPubkey, []Instruction{ix}, blockhash)

	tx, err := NewTransaction(msg, []ed25519.PrivateKey{privKey})
	if err != nil {
		t.Fatalf("NewTransaction failed: %v", err)
	}

	// Signature should be 64 bytes
	if len(tx.Signatures[0]) != 64 {
		t.Errorf("signature length = %d, want 64", len(tx.Signatures[0]))
	}

	// Verify signature
	serializedMsg := msg.Serialize()
	if !ed25519.Verify(pubKey, serializedMsg, tx.Signatures[0]) {
		t.Error("signature verification failed")
	}
}

func TestTransaction_Serialize(t *testing.T) {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)

	var fromPubkey Pubkey
	copy(fromPubkey[:], pubKey)

	recipient, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	var blockhash Pubkey

	ix := SystemTransfer(fromPubkey, recipient, 100)
	msg := NewMessage(fromPubkey, []Instruction{ix}, blockhash)

	tx, err := NewTransaction(msg, []ed25519.PrivateKey{privKey})
	if err != nil {
		t.Fatalf("NewTransaction failed: %v", err)
	}

	serialized := tx.Serialize()
	if len(serialized) == 0 {
		t.Error("serialized transaction is empty")
	}

	// Should start with compact-u16 encoding of 1 (one signature)
	if serialized[0] != 0x01 {
		t.Errorf("first byte = 0x%02x, want 0x01 (compact-u16 for 1)", serialized[0])
	}
}

func TestTransaction_ToBase64(t *testing.T) {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)

	var fromPubkey Pubkey
	copy(fromPubkey[:], pubKey)

	recipient, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	var blockhash Pubkey

	ix := SystemTransfer(fromPubkey, recipient, 100)
	msg := NewMessage(fromPubkey, []Instruction{ix}, blockhash)

	tx, err := NewTransaction(msg, []ed25519.PrivateKey{privKey})
	if err != nil {
		t.Fatalf("NewTransaction failed: %v", err)
	}

	b64 := tx.ToBase64()
	if b64 == "" {
		t.Error("base64 transaction is empty")
	}

	// Verify base64 decodes back to serialized bytes
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	serialized := tx.Serialize()
	if len(decoded) != len(serialized) {
		t.Errorf("decoded length = %d, serialized length = %d", len(decoded), len(serialized))
	}
}

func TestTransaction_WrongSignerCount(t *testing.T) {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)

	var fromPubkey Pubkey
	copy(fromPubkey[:], pubKey)

	recipient, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	var blockhash Pubkey

	ix := SystemTransfer(fromPubkey, recipient, 100)
	msg := NewMessage(fromPubkey, []Instruction{ix}, blockhash)

	// Pass 2 signers when only 1 is required
	_, err := NewTransaction(msg, []ed25519.PrivateKey{privKey, privKey})
	if err == nil {
		t.Error("expected error for wrong signer count")
	}

	// Pass 0 signers
	_, err = NewTransaction(msg, []ed25519.PrivateKey{})
	if err == nil {
		t.Error("expected error for zero signers")
	}
}
