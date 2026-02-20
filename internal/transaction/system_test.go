package transaction

import (
	"encoding/binary"
	"testing"
)

func TestSystemTransfer_DataLayout(t *testing.T) {
	from, _ := PubkeyFromBase58("11111111111111111111111111111111")
	to, _ := PubkeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")

	ix := SystemTransfer(from, to, 1_000_000_000)

	// Data should be exactly 12 bytes
	if len(ix.Data) != 12 {
		t.Fatalf("data length = %d, want 12", len(ix.Data))
	}

	// First 4 bytes: instruction index = 2
	idx := binary.LittleEndian.Uint32(ix.Data[0:4])
	if idx != 2 {
		t.Errorf("instruction index = %d, want 2", idx)
	}

	// Next 8 bytes: lamports
	lamports := binary.LittleEndian.Uint64(ix.Data[4:12])
	if lamports != 1_000_000_000 {
		t.Errorf("lamports = %d, want 1000000000", lamports)
	}
}

func TestSystemTransfer_Accounts(t *testing.T) {
	from, _ := PubkeyFromBase58("11111111111111111111111111111111")
	to, _ := PubkeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")

	ix := SystemTransfer(from, to, 100)

	if len(ix.Accounts) != 2 {
		t.Fatalf("accounts count = %d, want 2", len(ix.Accounts))
	}

	// From: signer + writable
	if !ix.Accounts[0].IsSigner {
		t.Error("from account should be signer")
	}
	if !ix.Accounts[0].IsWritable {
		t.Error("from account should be writable")
	}

	// To: not signer, writable
	if ix.Accounts[1].IsSigner {
		t.Error("to account should not be signer")
	}
	if !ix.Accounts[1].IsWritable {
		t.Error("to account should be writable")
	}
}

func TestSystemTransfer_ProgramID(t *testing.T) {
	from, _ := PubkeyFromBase58("11111111111111111111111111111111")
	to, _ := PubkeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")

	ix := SystemTransfer(from, to, 0)
	if ix.ProgramID != SystemProgramID {
		t.Errorf("ProgramID = %s, want %s", ix.ProgramID, SystemProgramID)
	}
}

func TestSystemTransfer_BoundaryValues(t *testing.T) {
	from, _ := PubkeyFromBase58("11111111111111111111111111111111")
	to, _ := PubkeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")

	tests := []uint64{0, 1, 1_000_000_000, ^uint64(0)}
	for _, lamports := range tests {
		ix := SystemTransfer(from, to, lamports)
		got := binary.LittleEndian.Uint64(ix.Data[4:12])
		if got != lamports {
			t.Errorf("lamports = %d, want %d", got, lamports)
		}
	}
}
