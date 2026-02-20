package transaction

import "encoding/binary"

// SystemTransfer creates a System Program transfer instruction.
// Data format: uint32(2) LE (transfer instruction index) + uint64(lamports) LE = 12 bytes.
func SystemTransfer(from, to Pubkey, lamports uint64) Instruction {
	data := make([]byte, 12)
	binary.LittleEndian.PutUint32(data[0:4], 2) // transfer instruction index
	binary.LittleEndian.PutUint64(data[4:12], lamports)

	return Instruction{
		ProgramID: SystemProgramID,
		Accounts: []AccountMeta{
			{Pubkey: from, IsSigner: true, IsWritable: true},
			{Pubkey: to, IsSigner: false, IsWritable: true},
		},
		Data: data,
	}
}
