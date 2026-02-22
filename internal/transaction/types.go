package transaction

import (
	"crypto/ed25519"
	"fmt"

	"github.com/mr-tron/base58"
)

// Pubkey is a 32-byte Solana public key.
type Pubkey [32]byte

// String returns the base58 representation of the public key.
func (p Pubkey) String() string {
	return base58.Encode(p[:])
}

// PubkeyFromBase58 decodes a base58-encoded public key.
func PubkeyFromBase58(s string) (Pubkey, error) {
	b, err := base58.Decode(s)
	if err != nil {
		return Pubkey{}, fmt.Errorf("decode pubkey %q: %w", s, err)
	}
	if len(b) != 32 {
		return Pubkey{}, fmt.Errorf("pubkey %q: expected 32 bytes, got %d", s, len(b))
	}
	var pk Pubkey
	copy(pk[:], b)
	return pk, nil
}

// PubkeyFromBytes creates a Pubkey from a byte slice.
// Returns an error if the slice is not exactly 32 bytes.
func PubkeyFromBytes(b []byte) (Pubkey, error) {
	if len(b) != 32 {
		return Pubkey{}, fmt.Errorf("pubkey: expected 32 bytes, got %d", len(b))
	}
	var pk Pubkey
	copy(pk[:], b)
	return pk, nil
}

// PubkeyFromPublicKey creates a Pubkey from an ed25519 public key.
func PubkeyFromPublicKey(pub ed25519.PublicKey) Pubkey {
	var pk Pubkey
	copy(pk[:], pub)
	return pk
}

// AccountMeta describes an account used by an instruction.
type AccountMeta struct {
	Pubkey     Pubkey
	IsSigner   bool
	IsWritable bool
}

// Instruction is an uncompiled Solana instruction.
type Instruction struct {
	ProgramID Pubkey
	Accounts  []AccountMeta
	Data      []byte
}

// MessageHeader contains the account counts for a Solana message.
type MessageHeader struct {
	NumRequiredSignatures       uint8
	NumReadonlySignedAccounts   uint8
	NumReadonlyUnsignedAccounts uint8
}

// CompiledInstruction is an instruction with account indices instead of pubkeys.
type CompiledInstruction struct {
	ProgramIDIndex uint8
	AccountIndices []uint8
	Data           []byte
}

// Message is a Solana transaction message.
type Message struct {
	Header          MessageHeader
	AccountKeys     []Pubkey
	RecentBlockhash Pubkey
	Instructions    []CompiledInstruction
}

// Transaction is a signed Solana transaction.
type Transaction struct {
	Signatures        [][]byte
	Message           Message
	serializedMessage []byte
}
