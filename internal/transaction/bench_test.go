package transaction

import "testing"

func BenchmarkMessageSerialize(b *testing.B) {
	// Build a realistic message: 5 accounts, 3 instructions (priority fee + transfer)
	feePayer := Pubkey{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	to := Pubkey{33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48,
		49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64}
	blockhash := Pubkey{99, 98, 97, 96, 95, 94, 93, 92, 91, 90, 89, 88, 87, 86,
		85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68}

	instructions := []Instruction{
		SystemTransfer(feePayer, to, 1_000_000_000),
	}
	instructions = append(PriorityFeeInstructions(PriorityFeeMedium), instructions...)

	msg := NewMessage(feePayer, instructions, blockhash)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = msg.Serialize()
	}
}
