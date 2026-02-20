package transaction

import (
	"crypto/sha256"
	"fmt"

	"filippo.io/edwards25519"
)

// DeriveATA computes the Associated Token Account address for a wallet and mint.
func DeriveATA(wallet, mint Pubkey) (Pubkey, error) {
	return FindProgramAddress(
		[][]byte{wallet[:], TokenProgramID[:], mint[:]},
		ATAProgramID,
	)
}

// FindProgramAddress finds a valid program-derived address (PDA) for the given seeds and program ID.
// It tries bump seeds from 255 down to 0, returning the first off-curve result.
func FindProgramAddress(seeds [][]byte, programID Pubkey) (Pubkey, error) {
	for bump := uint8(255); ; bump-- {
		candidate, err := CreateProgramAddress(append(seeds, []byte{bump}), programID)
		if err == nil {
			return candidate, nil
		}
		if bump == 0 {
			break
		}
	}
	return Pubkey{}, fmt.Errorf("could not find valid PDA")
}

// CreateProgramAddress creates a program-derived address from seeds and a program ID.
// Returns an error if the resulting address is on the Ed25519 curve (invalid PDA).
func CreateProgramAddress(seeds [][]byte, programID Pubkey) (Pubkey, error) {
	h := sha256.New()
	for _, seed := range seeds {
		if len(seed) > 32 {
			return Pubkey{}, fmt.Errorf("seed too long: %d bytes (max 32)", len(seed))
		}
		h.Write(seed)
	}
	h.Write(programID[:])
	h.Write([]byte("ProgramDerivedAddress"))

	hashBytes := h.Sum(nil)

	// Check if the result is on the Ed25519 curve — if so, it's invalid as a PDA
	if isOnCurve(hashBytes) {
		return Pubkey{}, fmt.Errorf("derived address is on the Ed25519 curve")
	}

	var pk Pubkey
	copy(pk[:], hashBytes)
	return pk, nil
}

// isOnCurve checks if a 32-byte value is a valid Ed25519 curve point.
func isOnCurve(b []byte) bool {
	if len(b) != 32 {
		return false
	}
	_, err := new(edwards25519.Point).SetBytes(b)
	return err == nil
}

// CreateATAInstruction creates an instruction to create an Associated Token Account.
func CreateATAInstruction(funder, wallet, mint Pubkey) (Instruction, error) {
	ata, err := DeriveATA(wallet, mint)
	if err != nil {
		return Instruction{}, fmt.Errorf("derive ATA: %w", err)
	}

	return Instruction{
		ProgramID: ATAProgramID,
		Accounts: []AccountMeta{
			{Pubkey: funder, IsSigner: true, IsWritable: true},
			{Pubkey: ata, IsSigner: false, IsWritable: true},
			{Pubkey: wallet, IsSigner: false, IsWritable: false},
			{Pubkey: mint, IsSigner: false, IsWritable: false},
			{Pubkey: SystemProgramID, IsSigner: false, IsWritable: false},
			{Pubkey: TokenProgramID, IsSigner: false, IsWritable: false},
			{Pubkey: SysvarRentID, IsSigner: false, IsWritable: false},
		},
		Data: nil, // ATA create instruction has no data
	}, nil
}
