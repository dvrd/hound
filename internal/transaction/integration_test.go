package transaction_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	"github.com/dvrd/hound/internal/transaction"
)

func TestFullSOLTransferTransaction(t *testing.T) {
	// 1. Generate a deterministic keypair from known seed
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)

	var fromPubkey transaction.Pubkey
	copy(fromPubkey[:], pubKey)

	// 2. Create a recipient
	recipientSeed := make([]byte, 32)
	for i := range recipientSeed {
		recipientSeed[i] = byte(i + 32)
	}
	recipientPriv := ed25519.NewKeyFromSeed(recipientSeed)
	recipientPub := recipientPriv.Public().(ed25519.PublicKey)
	var toPubkey transaction.Pubkey
	copy(toPubkey[:], recipientPub)

	// 3. Build transfer instruction (1 SOL = 1_000_000_000 lamports)
	ix := transaction.SystemTransfer(fromPubkey, toPubkey, 1_000_000_000)

	// 4. Use a fake blockhash (32 zero bytes)
	var blockhash transaction.Pubkey

	// 5. Build message
	msg := transaction.NewMessage(fromPubkey, []transaction.Instruction{ix}, blockhash)

	// 6. Verify message structure
	if msg.Header.NumRequiredSignatures != 1 {
		t.Errorf("expected 1 required signature, got %d", msg.Header.NumRequiredSignatures)
	}
	if len(msg.AccountKeys) != 3 { // from, to, system program
		t.Errorf("expected 3 account keys, got %d", len(msg.AccountKeys))
	}
	// Fee payer must be at index 0
	if msg.AccountKeys[0] != fromPubkey {
		t.Error("fee payer should be at index 0")
	}

	// 7. Sign transaction
	tx, err := transaction.NewTransaction(msg, []ed25519.PrivateKey{privKey})
	if err != nil {
		t.Fatalf("NewTransaction failed: %v", err)
	}

	// 8. Verify signature
	serializedMsg := msg.Serialize()
	if len(tx.Signatures) != 1 {
		t.Fatalf("expected 1 signature, got %d", len(tx.Signatures))
	}
	if len(tx.Signatures[0]) != 64 {
		t.Errorf("signature should be 64 bytes, got %d", len(tx.Signatures[0]))
	}
	if !ed25519.Verify(pubKey, serializedMsg, tx.Signatures[0]) {
		t.Error("signature verification failed")
	}

	// 9. Verify serialization
	serialized := tx.Serialize()
	if len(serialized) == 0 {
		t.Error("serialized transaction is empty")
	}

	// 10. Verify base64 round-trip
	b64 := tx.ToBase64()
	if b64 == "" {
		t.Error("base64 transaction is empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	if !bytes.Equal(decoded, serialized) {
		t.Error("base64 decode doesn't match serialized bytes")
	}
}

func TestFullSPLTransferTransaction(t *testing.T) {
	// 1. Generate deterministic keypairs
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)

	var fromPubkey transaction.Pubkey
	copy(fromPubkey[:], pubKey)

	recipientSeed := make([]byte, 32)
	for i := range recipientSeed {
		recipientSeed[i] = byte(i + 32)
	}
	recipientPriv := ed25519.NewKeyFromSeed(recipientSeed)
	recipientPub := recipientPriv.Public().(ed25519.PublicKey)
	var toPubkey transaction.Pubkey
	copy(toPubkey[:], recipientPub)

	// 2. Use USDC mint
	usdcMint, err := transaction.PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	if err != nil {
		t.Fatalf("failed to parse USDC mint: %v", err)
	}

	// 3. Derive ATAs
	senderATA, err := transaction.DeriveATA(fromPubkey, usdcMint)
	if err != nil {
		t.Fatalf("failed to derive sender ATA: %v", err)
	}
	recipientATA, err := transaction.DeriveATA(toPubkey, usdcMint)
	if err != nil {
		t.Fatalf("failed to derive recipient ATA: %v", err)
	}

	// 4. Build instructions: create ATA + transfer
	createATAIx, err := transaction.CreateATAInstruction(fromPubkey, toPubkey, usdcMint)
	if err != nil {
		t.Fatalf("failed to create ATA instruction: %v", err)
	}

	// Transfer 1 USDC (6 decimals = 1_000_000 base units)
	transferIx := transaction.TokenTransferChecked(
		senderATA, usdcMint, recipientATA, fromPubkey,
		1_000_000, 6,
	)

	// 5. Build message with 2 instructions
	var blockhash transaction.Pubkey
	msg := transaction.NewMessage(fromPubkey, []transaction.Instruction{createATAIx, transferIx}, blockhash)

	// 6. Verify structure
	if msg.Header.NumRequiredSignatures != 1 {
		t.Errorf("expected 1 required signature, got %d", msg.Header.NumRequiredSignatures)
	}
	if len(msg.Instructions) != 2 {
		t.Errorf("expected 2 compiled instructions, got %d", len(msg.Instructions))
	}
	// Should have multiple account keys (sender, recipient, ATAs, mint, programs, sysvar)
	if len(msg.AccountKeys) < 5 {
		t.Errorf("expected at least 5 account keys for SPL transfer with ATA creation, got %d", len(msg.AccountKeys))
	}

	// 7. Sign and verify
	tx, err := transaction.NewTransaction(msg, []ed25519.PrivateKey{privKey})
	if err != nil {
		t.Fatalf("NewTransaction failed: %v", err)
	}

	serializedMsg := msg.Serialize()
	if !ed25519.Verify(pubKey, serializedMsg, tx.Signatures[0]) {
		t.Error("signature verification failed for SPL transfer")
	}

	// 8. Verify serialization
	serialized := tx.Serialize()
	if len(serialized) == 0 {
		t.Error("serialized SPL transaction is empty")
	}

	b64 := tx.ToBase64()
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	if !bytes.Equal(decoded, serialized) {
		t.Error("base64 decode doesn't match serialized bytes for SPL transfer")
	}
}
