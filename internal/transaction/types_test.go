package transaction

import "testing"

func TestTypes_PubkeyFromBase58_SystemProgram(t *testing.T) {
	addr := "11111111111111111111111111111111"
	pk, err := PubkeyFromBase58(addr)
	if err != nil {
		t.Fatalf("PubkeyFromBase58(%q) error: %v", addr, err)
	}
	if pk.String() != addr {
		t.Errorf("round-trip: got %q, want %q", pk.String(), addr)
	}
}

func TestTypes_PubkeyFromBase58_Invalid(t *testing.T) {
	_, err := PubkeyFromBase58("not-valid-base58!!!")
	if err == nil {
		t.Error("PubkeyFromBase58(invalid) should return error")
	}
}

func TestTypes_PubkeyFromBase58_WrongLength(t *testing.T) {
	// base58 of a short byte slice
	_, err := PubkeyFromBase58("1")
	if err == nil {
		t.Error("PubkeyFromBase58(short) should return error")
	}
}

func TestTypes_PubkeyString(t *testing.T) {
	var pk Pubkey // all zeros
	got := pk.String()
	if got != "11111111111111111111111111111111" {
		t.Errorf("zero pubkey String() = %q, want system program address", got)
	}
}

func TestTypes_AccountMeta(t *testing.T) {
	pk, _ := PubkeyFromBase58("11111111111111111111111111111111")
	am := AccountMeta{Pubkey: pk, IsSigner: true, IsWritable: true}
	if !am.IsSigner {
		t.Error("IsSigner should be true")
	}
	if !am.IsWritable {
		t.Error("IsWritable should be true")
	}
}
