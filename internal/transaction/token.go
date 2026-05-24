package transaction

import "encoding/binary"

// TokenTransferChecked creates an SPL Token transferChecked instruction.
// Data format: uint8(12) (transferChecked index) + uint64(amount) LE + uint8(decimals) = 10 bytes.
func TokenTransferChecked(source, mint, destination, owner Pubkey, amount uint64, decimals uint8) Instruction {
	data := make([]byte, 10)
	data[0] = 12 // transferChecked instruction index
	binary.LittleEndian.PutUint64(data[1:9], amount)
	data[9] = decimals

	return Instruction{
		ProgramID: TokenProgramID,
		Accounts: []AccountMeta{
			{Pubkey: source, IsSigner: false, IsWritable: true},
			{Pubkey: mint, IsSigner: false, IsWritable: false},
			{Pubkey: destination, IsSigner: false, IsWritable: true},
			{Pubkey: owner, IsSigner: true, IsWritable: false},
		},
		Data: data,
	}
}
