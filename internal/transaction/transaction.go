package transaction

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

// NewTransaction creates a signed transaction from a message and signers.
func NewTransaction(msg Message, signers []ed25519.PrivateKey) (*Transaction, error) {
	if len(signers) != int(msg.Header.NumRequiredSignatures) {
		return nil, fmt.Errorf(
			"expected %d signers, got %d",
			msg.Header.NumRequiredSignatures, len(signers),
		)
	}

	serializedMsg := msg.Serialize()

	signatures := make([][]byte, len(signers))
	for i, signer := range signers {
		sig := ed25519.Sign(signer, serializedMsg)
		signatures[i] = sig
	}

	return &Transaction{
		Signatures:        signatures,
		Message:           msg,
		serializedMessage: serializedMsg,
	}, nil
}

// Serialize serializes the transaction to bytes in Solana's wire format.
func (t *Transaction) Serialize() []byte {
	// Pre-compute capacity: compact-u16(1-3) + sigs(64 each) + message.
	msgLen := len(t.serializedMessage)
	if msgLen == 0 {
		msgLen = 256 // estimate for unserialized message
	}
	cap := 3 + len(t.Signatures)*64 + msgLen
	buf := make([]byte, 0, cap)

	// Number of signatures (compact-u16)
	buf = AppendCompactU16(buf, uint16(len(t.Signatures)))

	// Signatures (64 bytes each)
	for _, sig := range t.Signatures {
		buf = append(buf, sig...)
	}

	// Serialized message
	if t.serializedMessage != nil {
		buf = append(buf, t.serializedMessage...)
	} else {
		buf = append(buf, t.Message.Serialize()...)
	}

	return buf
}

// ToBase64 returns the base64-encoded serialized transaction.
func (t *Transaction) ToBase64() string {
	return base64.StdEncoding.EncodeToString(t.Serialize())
}
