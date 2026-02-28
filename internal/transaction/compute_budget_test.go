package transaction

import (
	"encoding/binary"
	"testing"
)

func TestSetComputeUnitLimit_DataLayout(t *testing.T) {
	ix := SetComputeUnitLimit(200_000)

	if len(ix.Data) != 5 {
		t.Fatalf("SetComputeUnitLimit data length = %d, want 5", len(ix.Data))
	}
	if ix.Data[0] != computeBudgetSetComputeUnitLimit {
		t.Errorf("discriminator = %d, want %d", ix.Data[0], computeBudgetSetComputeUnitLimit)
	}
	got := binary.LittleEndian.Uint32(ix.Data[1:])
	if got != 200_000 {
		t.Errorf("units = %d, want 200000", got)
	}
}

func TestSetComputeUnitLimit_ProgramID(t *testing.T) {
	ix := SetComputeUnitLimit(100_000)
	if ix.ProgramID != ComputeBudgetProgramID {
		t.Errorf("programID mismatch: got %v, want ComputeBudgetProgramID", ix.ProgramID)
	}
	if ix.Accounts != nil {
		t.Error("ComputeBudget SetComputeUnitLimit should have no accounts")
	}
}

func TestSetComputeUnitPrice_DataLayout(t *testing.T) {
	const price uint64 = 10_000
	ix := SetComputeUnitPrice(price)

	if len(ix.Data) != 9 {
		t.Fatalf("SetComputeUnitPrice data length = %d, want 9", len(ix.Data))
	}
	if ix.Data[0] != computeBudgetSetComputeUnitPrice {
		t.Errorf("discriminator = %d, want %d", ix.Data[0], computeBudgetSetComputeUnitPrice)
	}
	got := binary.LittleEndian.Uint64(ix.Data[1:])
	if got != price {
		t.Errorf("microLamports = %d, want %d", got, price)
	}
}

func TestSetComputeUnitPrice_ProgramID(t *testing.T) {
	ix := SetComputeUnitPrice(5_000)
	if ix.ProgramID != ComputeBudgetProgramID {
		t.Errorf("programID mismatch: got %v, want ComputeBudgetProgramID", ix.ProgramID)
	}
	if ix.Accounts != nil {
		t.Error("ComputeBudget SetComputeUnitPrice should have no accounts")
	}
}

func TestMicroLamportsForLevel(t *testing.T) {
	tests := []struct {
		level PriorityFeeLevel
		want  uint64
	}{
		{PriorityFeeNone, 0},
		{PriorityFeeLow, 1_000},
		{PriorityFeeMedium, 10_000},
		{PriorityFeeHigh, 100_000},
	}
	for _, tt := range tests {
		got := MicroLamportsForLevel(tt.level)
		if got != tt.want {
			t.Errorf("MicroLamportsForLevel(%d) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestPriorityFeeInstructions_None(t *testing.T) {
	ixs := PriorityFeeInstructions(PriorityFeeNone)
	if ixs != nil {
		t.Errorf("PriorityFeeNone should return nil, got %v", ixs)
	}
}

func TestPriorityFeeInstructions_Medium(t *testing.T) {
	ixs := PriorityFeeInstructions(PriorityFeeMedium)
	if len(ixs) != 2 {
		t.Fatalf("expected 2 instructions, got %d", len(ixs))
	}
	// First: SetComputeUnitLimit
	if ixs[0].Data[0] != computeBudgetSetComputeUnitLimit {
		t.Error("first instruction should be SetComputeUnitLimit")
	}
	limit := binary.LittleEndian.Uint32(ixs[0].Data[1:])
	if limit != DefaultComputeUnitLimit {
		t.Errorf("compute unit limit = %d, want %d", limit, DefaultComputeUnitLimit)
	}
	// Second: SetComputeUnitPrice
	if ixs[1].Data[0] != computeBudgetSetComputeUnitPrice {
		t.Error("second instruction should be SetComputeUnitPrice")
	}
	price := binary.LittleEndian.Uint64(ixs[1].Data[1:])
	if price != MicroLamportsForLevel(PriorityFeeMedium) {
		t.Errorf("micro-lamports = %d, want %d", price, MicroLamportsForLevel(PriorityFeeMedium))
	}
}

func TestPriorityFeeInstructions_AllLevelsReturnTwoInstructions(t *testing.T) {
	for _, level := range []PriorityFeeLevel{PriorityFeeLow, PriorityFeeMedium, PriorityFeeHigh} {
		ixs := PriorityFeeInstructions(level)
		if len(ixs) != 2 {
			t.Errorf("level %d: expected 2 instructions, got %d", level, len(ixs))
		}
	}
}

func TestComputeBudgetProgramID_Is32Bytes(t *testing.T) {
	var zero [32]byte
	if ComputeBudgetProgramID == zero {
		t.Error("ComputeBudgetProgramID should not be the zero pubkey")
	}
}
