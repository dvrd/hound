package transaction

import "testing"

func TestPrograms_SystemProgramID(t *testing.T) {
	expected := "11111111111111111111111111111111"
	if SystemProgramID.String() != expected {
		t.Errorf("SystemProgramID = %q, want %q", SystemProgramID.String(), expected)
	}
	if len(SystemProgramID) != 32 {
		t.Errorf("SystemProgramID length = %d, want 32", len(SystemProgramID))
	}
}

func TestPrograms_TokenProgramID(t *testing.T) {
	expected := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	if TokenProgramID.String() != expected {
		t.Errorf("TokenProgramID = %q, want %q", TokenProgramID.String(), expected)
	}
}

func TestPrograms_ATAProgramID(t *testing.T) {
	expected := "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"
	if ATAProgramID.String() != expected {
		t.Errorf("ATAProgramID = %q, want %q", ATAProgramID.String(), expected)
	}
}

func TestPrograms_SysvarRentID(t *testing.T) {
	expected := "SysvarRent111111111111111111111111111111111"
	if SysvarRentID.String() != expected {
		t.Errorf("SysvarRentID = %q, want %q", SysvarRentID.String(), expected)
	}
}

func TestPrograms_SOLMint(t *testing.T) {
	expected := "So11111111111111111111111111111111111111112"
	if SOLMint.String() != expected {
		t.Errorf("SOLMint = %q, want %q", SOLMint.String(), expected)
	}
}

func TestPrograms_AllAre32Bytes(t *testing.T) {
	ids := []Pubkey{SystemProgramID, TokenProgramID, ATAProgramID, SysvarRentID, SOLMint}
	for _, id := range ids {
		if len(id) != 32 {
			t.Errorf("%s length = %d, want 32", id.String(), len(id))
		}
	}
}
