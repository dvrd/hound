package transaction

import "encoding/binary"

// ComputeBudget instruction discriminators.
const (
	computeBudgetSetComputeUnitLimit uint8 = 2
	computeBudgetSetComputeUnitPrice uint8 = 3
)

// SetComputeUnitLimit builds a ComputeBudget::SetComputeUnitLimit instruction.
// units is the maximum number of compute units the transaction may consume.
func SetComputeUnitLimit(units uint32) Instruction {
	data := make([]byte, 5)
	data[0] = computeBudgetSetComputeUnitLimit
	binary.LittleEndian.PutUint32(data[1:], units)
	return Instruction{
		ProgramID: ComputeBudgetProgramID,
		Accounts:  nil, // ComputeBudget instructions have no accounts
		Data:      data,
	}
}

// SetComputeUnitPrice builds a ComputeBudget::SetComputeUnitPrice instruction.
// microLamports is the price per compute unit in micro-lamports (1 lamport = 1,000,000 micro-lamports).
// A price of 1000 micro-lamports/CU with a 200k CU limit adds 0.0002 SOL in priority fees.
func SetComputeUnitPrice(microLamports uint64) Instruction {
	data := make([]byte, 9)
	data[0] = computeBudgetSetComputeUnitPrice
	binary.LittleEndian.PutUint64(data[1:], microLamports)
	return Instruction{
		ProgramID: ComputeBudgetProgramID,
		Accounts:  nil,
		Data:      data,
	}
}

// DefaultComputeUnitLimit is a safe upper bound for simple SOL/SPL transfers.
const DefaultComputeUnitLimit = 200_000

// PriorityFeeLevel represents a user-selectable fee tier.
type PriorityFeeLevel int

const (
	PriorityFeeNone   PriorityFeeLevel = 0
	PriorityFeeLow    PriorityFeeLevel = 1
	PriorityFeeMedium PriorityFeeLevel = 2
	PriorityFeeHigh   PriorityFeeLevel = 3
)

// MicroLamportsForLevel returns the compute unit price in micro-lamports for a given fee level.
// Values are conservative defaults; callers can override with explicit micro-lamport values.
func MicroLamportsForLevel(level PriorityFeeLevel) uint64 {
	switch level {
	case PriorityFeeLow:
		return 1_000 // ~0.0002 SOL at 200k CU
	case PriorityFeeMedium:
		return 10_000 // ~0.002 SOL at 200k CU
	case PriorityFeeHigh:
		return 100_000 // ~0.02 SOL at 200k CU
	default:
		return 0
	}
}

// PriorityFeeInstructions returns the ComputeBudget instructions to prepend to a transaction.
// If level is PriorityFeeNone, returns nil (no instructions added).
func PriorityFeeInstructions(level PriorityFeeLevel) []Instruction {
	microLamports := MicroLamportsForLevel(level)
	if microLamports == 0 {
		return nil
	}
	return []Instruction{
		SetComputeUnitLimit(DefaultComputeUnitLimit),
		SetComputeUnitPrice(microLamports),
	}
}
