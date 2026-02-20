package transaction

import "testing"

func TestATA_DeriveATA(t *testing.T) {
	// Use well-known test addresses
	wallet, err := PubkeyFromBase58("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU")
	if err != nil {
		t.Fatalf("parse wallet: %v", err)
	}
	mint, err := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	if err != nil {
		t.Fatalf("parse mint: %v", err)
	}

	ata, err := DeriveATA(wallet, mint)
	if err != nil {
		t.Fatalf("DeriveATA error: %v", err)
	}

	// Verify it's a valid 32-byte address
	if len(ata) != 32 {
		t.Errorf("ATA length = %d, want 32", len(ata))
	}

	// Verify it's deterministic
	ata2, err := DeriveATA(wallet, mint)
	if err != nil {
		t.Fatalf("DeriveATA second call error: %v", err)
	}
	if ata != ata2 {
		t.Error("DeriveATA should be deterministic")
	}

	// Verify it's different from wallet and mint
	if ata == wallet {
		t.Error("ATA should differ from wallet")
	}
	if ata == mint {
		t.Error("ATA should differ from mint")
	}

	t.Logf("Derived ATA: %s", ata.String())
}

func TestATA_CreateATAInstruction(t *testing.T) {
	funder, _ := PubkeyFromBase58("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU")
	wallet, _ := PubkeyFromBase58("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU")
	mint, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")

	ix, err := CreateATAInstruction(funder, wallet, mint)
	if err != nil {
		t.Fatalf("CreateATAInstruction error: %v", err)
	}

	// Should have 7 accounts
	if len(ix.Accounts) != 7 {
		t.Fatalf("accounts count = %d, want 7", len(ix.Accounts))
	}

	// Funder: signer + writable
	if !ix.Accounts[0].IsSigner || !ix.Accounts[0].IsWritable {
		t.Error("funder should be signer+writable")
	}

	// ATA: writable, not signer
	if ix.Accounts[1].IsSigner || !ix.Accounts[1].IsWritable {
		t.Error("ATA should be writable, not signer")
	}

	// Wallet: readonly
	if ix.Accounts[2].IsWritable {
		t.Error("wallet should be readonly")
	}

	// Mint: readonly
	if ix.Accounts[3].IsWritable {
		t.Error("mint should be readonly")
	}

	// Program ID
	if ix.ProgramID != ATAProgramID {
		t.Errorf("ProgramID = %s, want %s", ix.ProgramID, ATAProgramID)
	}

	// Data should be empty/nil
	if len(ix.Data) != 0 {
		t.Errorf("data length = %d, want 0", len(ix.Data))
	}
}

func TestATA_FindProgramAddress_Deterministic(t *testing.T) {
	wallet, _ := PubkeyFromBase58("11111111111111111111111111111111")
	mint, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")

	pda1, err := FindProgramAddress(
		[][]byte{wallet[:], TokenProgramID[:], mint[:]},
		ATAProgramID,
	)
	if err != nil {
		t.Fatalf("FindProgramAddress error: %v", err)
	}

	pda2, err := FindProgramAddress(
		[][]byte{wallet[:], TokenProgramID[:], mint[:]},
		ATAProgramID,
	)
	if err != nil {
		t.Fatalf("FindProgramAddress second call error: %v", err)
	}

	if pda1 != pda2 {
		t.Error("FindProgramAddress should be deterministic")
	}
}
