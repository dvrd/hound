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

func TestPubkeyFromBytes_Valid(t *testing.T) {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i)
	}
	pk, err := PubkeyFromBytes(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pk[0] != 0 || pk[31] != 31 {
		t.Errorf("pubkey bytes mismatch")
	}
}

func TestPubkeyFromBytes_TooShort(t *testing.T) {
	_, err := PubkeyFromBytes(make([]byte, 16))
	if err == nil {
		t.Fatal("expected error for 16-byte input")
	}
}

func TestPubkeyFromBytes_TooLong(t *testing.T) {
	_, err := PubkeyFromBytes(make([]byte, 64))
	if err == nil {
		t.Fatal("expected error for 64-byte input")
	}
}

func TestPubkeyFromBytes_Empty(t *testing.T) {
	_, err := PubkeyFromBytes(nil)
	if err == nil {
		t.Fatal("expected error for nil input")
	}
}
