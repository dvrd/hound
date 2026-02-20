package transaction

// NewMessage builds a Solana Message from a fee payer, instructions, and recent blockhash.
// It deduplicates accounts, sorts them into the correct order, and compiles instructions.
func NewMessage(feePayer Pubkey, instructions []Instruction, recentBlockhash Pubkey) Message {
	// 1. Collect all unique accounts
	type accountInfo struct {
		pubkey     Pubkey
		isSigner   bool
		isWritable bool
		isProgram  bool
	}

	accountMap := make(map[Pubkey]*accountInfo)

	// Fee payer is always first, always signer+writable
	accountMap[feePayer] = &accountInfo{
		pubkey:     feePayer,
		isSigner:   true,
		isWritable: true,
	}

	// Collect accounts from instructions
	for _, ix := range instructions {
		// Program IDs are readonly non-signers
		if _, exists := accountMap[ix.ProgramID]; !exists {
			accountMap[ix.ProgramID] = &accountInfo{
				pubkey:    ix.ProgramID,
				isProgram: true,
			}
		} else {
			accountMap[ix.ProgramID].isProgram = true
		}

		for _, acct := range ix.Accounts {
			if existing, exists := accountMap[acct.Pubkey]; exists {
				// Merge flags: if any instruction marks it signer/writable, keep that
				if acct.IsSigner {
					existing.isSigner = true
				}
				if acct.IsWritable {
					existing.isWritable = true
				}
			} else {
				accountMap[acct.Pubkey] = &accountInfo{
					pubkey:     acct.Pubkey,
					isSigner:   acct.IsSigner,
					isWritable: acct.IsWritable,
				}
			}
		}
	}

	// Program IDs are never signer, never writable
	for _, info := range accountMap {
		if info.isProgram {
			info.isSigner = false
			info.isWritable = false
		}
	}

	// 2. Sort into 4 groups: writable signers, readonly signers, writable non-signers, readonly non-signers
	var writableSigners, readonlySigners, writableNonSigners, readonlyNonSigners []Pubkey

	for _, info := range accountMap {
		if info.pubkey == feePayer {
			continue // fee payer handled separately
		}
		switch {
		case info.isSigner && info.isWritable:
			writableSigners = append(writableSigners, info.pubkey)
		case info.isSigner && !info.isWritable:
			readonlySigners = append(readonlySigners, info.pubkey)
		case !info.isSigner && info.isWritable:
			writableNonSigners = append(writableNonSigners, info.pubkey)
		default:
			readonlyNonSigners = append(readonlyNonSigners, info.pubkey)
		}
	}

	// Build ordered account keys: fee payer first
	accountKeys := make([]Pubkey, 0, len(accountMap))
	accountKeys = append(accountKeys, feePayer)
	accountKeys = append(accountKeys, writableSigners...)
	accountKeys = append(accountKeys, readonlySigners...)
	accountKeys = append(accountKeys, writableNonSigners...)
	accountKeys = append(accountKeys, readonlyNonSigners...)

	// 3. Compute header
	numSigners := 1 + len(writableSigners) + len(readonlySigners) // fee payer + other signers
	header := MessageHeader{
		NumRequiredSignatures:       uint8(numSigners),
		NumReadonlySignedAccounts:   uint8(len(readonlySigners)),
		NumReadonlyUnsignedAccounts: uint8(len(readonlyNonSigners)),
	}

	// 4. Build account index lookup
	indexMap := make(map[Pubkey]uint8, len(accountKeys))
	for i, key := range accountKeys {
		indexMap[key] = uint8(i)
	}

	// 5. Compile instructions
	compiledIxs := make([]CompiledInstruction, len(instructions))
	for i, ix := range instructions {
		accountIndices := make([]uint8, len(ix.Accounts))
		for j, acct := range ix.Accounts {
			accountIndices[j] = indexMap[acct.Pubkey]
		}
		compiledIxs[i] = CompiledInstruction{
			ProgramIDIndex: indexMap[ix.ProgramID],
			AccountIndices: accountIndices,
			Data:           ix.Data,
		}
	}

	return Message{
		Header:          header,
		AccountKeys:     accountKeys,
		RecentBlockhash: recentBlockhash,
		Instructions:    compiledIxs,
	}
}

// Serialize serializes a Message to bytes in Solana's wire format.
func (m Message) Serialize() []byte {
	var buf []byte

	// Header: 3 bytes
	buf = append(buf, m.Header.NumRequiredSignatures)
	buf = append(buf, m.Header.NumReadonlySignedAccounts)
	buf = append(buf, m.Header.NumReadonlyUnsignedAccounts)

	// Number of account keys (compact-u16)
	buf = append(buf, EncodeCompactU16(uint16(len(m.AccountKeys)))...)

	// Account keys (32 bytes each)
	for _, key := range m.AccountKeys {
		buf = append(buf, key[:]...)
	}

	// Recent blockhash (32 bytes)
	buf = append(buf, m.RecentBlockhash[:]...)

	// Number of instructions (compact-u16)
	buf = append(buf, EncodeCompactU16(uint16(len(m.Instructions)))...)

	// Each compiled instruction
	for _, ix := range m.Instructions {
		// Program ID index (1 byte)
		buf = append(buf, ix.ProgramIDIndex)

		// Number of accounts (compact-u16)
		buf = append(buf, EncodeCompactU16(uint16(len(ix.AccountIndices)))...)

		// Account indices (1 byte each)
		buf = append(buf, ix.AccountIndices...)

		// Data length (compact-u16)
		buf = append(buf, EncodeCompactU16(uint16(len(ix.Data)))...)

		// Data
		buf = append(buf, ix.Data...)
	}

	return buf
}
