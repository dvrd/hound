package transaction

import (
	"encoding/binary"
	"testing"
)

func TestTokenTransferChecked_DataLayout(t *testing.T) {
	src, _ := PubkeyFromBase58("11111111111111111111111111111111")
	mint, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	dst, _ := PubkeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	owner, _ := PubkeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")

	ix := TokenTransferChecked(src, mint, dst, owner, 1_000_000, 6)

	if len(ix.Data) != 10 {
		t.Fatalf("data length = %d, want 10", len(ix.Data))
	}
	if ix.Data[0] != 12 {
		t.Errorf("instruction index = %d, want 12", ix.Data[0])
	}
	amount := binary.LittleEndian.Uint64(ix.Data[1:9])
	if amount != 1_000_000 {
		t.Errorf("amount = %d, want 1000000", amount)
	}
	if ix.Data[9] != 6 {
		t.Errorf("decimals = %d, want 6", ix.Data[9])
	}
}

func TestTokenTransferChecked_Accounts(t *testing.T) {
	src, _ := PubkeyFromBase58("11111111111111111111111111111111")
	mint, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	dst, _ := PubkeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	owner, _ := PubkeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")

	ix := TokenTransferChecked(src, mint, dst, owner, 100, 6)

	if len(ix.Accounts) != 4 {
		t.Fatalf("accounts count = %d, want 4", len(ix.Accounts))
	}
	// source: writable
	if !ix.Accounts[0].IsWritable {
		t.Error("source should be writable")
	}
	// mint: readonly
	if ix.Accounts[1].IsWritable {
		t.Error("mint should be readonly")
	}
	// destination: writable
	if !ix.Accounts[2].IsWritable {
		t.Error("destination should be writable")
	}
	// owner: signer
	if !ix.Accounts[3].IsSigner {
		t.Error("owner should be signer")
	}
}
