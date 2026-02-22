package transaction

import (
	"bytes"
	"testing"
)

func TestMessage_SimpleSOLTransfer(t *testing.T) {
	sender, _ := PubkeyFromBase58("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU")
	recipient, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	var blockhash Pubkey // zero blockhash for testing

	ix := SystemTransfer(sender, recipient, 1_000_000_000)
	msg := NewMessage(sender, []Instruction{ix}, blockhash)

	// Header: 1 required signature, 0 readonly signed, 1 readonly unsigned (system program)
	if msg.Header.NumRequiredSignatures != 1 {
		t.Errorf("NumRequiredSignatures = %d, want 1", msg.Header.NumRequiredSignatures)
	}
	if msg.Header.NumReadonlySignedAccounts != 0 {
		t.Errorf("NumReadonlySignedAccounts = %d, want 0", msg.Header.NumReadonlySignedAccounts)
	}
	if msg.Header.NumReadonlyUnsignedAccounts != 1 {
		t.Errorf("NumReadonlyUnsignedAccounts = %d, want 1", msg.Header.NumReadonlyUnsignedAccounts)
	}

	// 3 account keys: sender, recipient, system program
	if len(msg.AccountKeys) != 3 {
		t.Fatalf("AccountKeys count = %d, want 3", len(msg.AccountKeys))
	}

	// Fee payer (sender) always at index 0
	if msg.AccountKeys[0] != sender {
		t.Errorf("AccountKeys[0] = %s, want sender %s", msg.AccountKeys[0], sender)
	}

	// System program should be last (readonly non-signer)
	if msg.AccountKeys[2] != SystemProgramID {
		t.Errorf("AccountKeys[2] = %s, want SystemProgramID %s", msg.AccountKeys[2], SystemProgramID)
	}

	// Serialized message should have non-zero length
	serialized := msg.Serialize()
	if len(serialized) == 0 {
		t.Error("serialized message is empty")
	}

	// Expected length: 3 (header) + 1 (compact-u16 for 3 accounts) + 3*32 (keys) + 32 (blockhash) + 1 (compact-u16 for 1 ix) + 1 (program idx) + 1 (compact-u16 for 2 accounts) + 2 (account indices) + 1 (compact-u16 for 12 data) + 12 (data)
	expectedLen := 3 + 1 + 96 + 32 + 1 + 1 + 1 + 2 + 1 + 12
	if len(serialized) != expectedLen {
		t.Errorf("serialized length = %d, want %d", len(serialized), expectedLen)
	}
}

func TestMessage_AccountDeduplication(t *testing.T) {
	account, _ := PubkeyFromBase58("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU")
	var blockhash Pubkey

	// Create an instruction that references the same account twice
	ix := Instruction{
		ProgramID: SystemProgramID,
		Accounts: []AccountMeta{
			{Pubkey: account, IsSigner: true, IsWritable: true},
			{Pubkey: account, IsSigner: false, IsWritable: true}, // same account again
		},
		Data: []byte{0x01},
	}

	msg := NewMessage(account, []Instruction{ix}, blockhash)

	// Account should appear only once (fee payer) + system program = 2
	if len(msg.AccountKeys) != 2 {
		t.Errorf("AccountKeys count = %d, want 2 (deduped account + system program)", len(msg.AccountKeys))
	}
}

func TestMessage_FeePayerAlwaysFirst(t *testing.T) {
	feePayer, _ := PubkeyFromBase58("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU")
	other, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	var blockhash Pubkey

	// Instruction where 'other' is signer+writable but fee payer is just referenced
	ix := Instruction{
		ProgramID: SystemProgramID,
		Accounts: []AccountMeta{
			{Pubkey: other, IsSigner: true, IsWritable: true},
			{Pubkey: feePayer, IsSigner: false, IsWritable: false},
		},
		Data: []byte{0x01},
	}

	msg := NewMessage(feePayer, []Instruction{ix}, blockhash)

	if msg.AccountKeys[0] != feePayer {
		t.Errorf("AccountKeys[0] = %s, want fee payer %s", msg.AccountKeys[0], feePayer)
	}
}

func TestMessage_MultipleInstructions(t *testing.T) {
	sender, _ := PubkeyFromBase58("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU")
	recipient1, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	recipient2, _ := PubkeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	var blockhash Pubkey

	ix1 := SystemTransfer(sender, recipient1, 100)
	ix2 := SystemTransfer(sender, recipient2, 200)

	msg := NewMessage(sender, []Instruction{ix1, ix2}, blockhash)

	// sender + recipient1 + recipient2 + system program = 4
	if len(msg.AccountKeys) != 4 {
		t.Errorf("AccountKeys count = %d, want 4", len(msg.AccountKeys))
	}

	// 2 compiled instructions
	if len(msg.Instructions) != 2 {
		t.Errorf("Instructions count = %d, want 2", len(msg.Instructions))
	}
}

func TestMessage_DeterministicSerialization(t *testing.T) {
	sender, _ := PubkeyFromBase58("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU")
	recipient1, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	recipient2, _ := PubkeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	var blockhash Pubkey

	ix1 := SystemTransfer(sender, recipient1, 100)
	ix2 := SystemTransfer(sender, recipient2, 200)

	// Build the same message 100 times and verify identical serialization
	var reference []byte
	for i := 0; i < 100; i++ {
		msg := NewMessage(sender, []Instruction{ix1, ix2}, blockhash)
		serialized := msg.Serialize()
		if i == 0 {
			reference = serialized
		} else if !bytes.Equal(serialized, reference) {
			t.Fatalf("iteration %d: serialization differs from reference (non-deterministic ordering)", i)
		}
	}
}
