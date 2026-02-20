package database

import (
	"errors"
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func TestInsertAndGetAllWallets(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	wallets := []models.Wallet{
		{Address: "addr1", Label: "Wallet 1", IsPrimary: false, WalletType: models.WalletTypeBIP44Standard, DerivationPath: "m/44'/501'/0'/0'", AccountIndex: 0},
		{Address: "addr2", Label: "Wallet 2", IsPrimary: true, WalletType: models.WalletTypeLegacy, DerivationPath: "legacy-sha256", AccountIndex: 0},
		{Address: "addr3", Label: "Wallet 3", IsPrimary: false, WalletType: models.WalletTypeSolanaCLI, DerivationPath: "m/44'/501'", AccountIndex: 0},
	}
	for _, w := range wallets {
		if err := db.InsertWallet(w); err != nil {
			t.Fatalf("InsertWallet(%s): %v", w.Label, err)
		}
	}

	got, err := db.GetAllWallets()
	if err != nil {
		t.Fatalf("GetAllWallets: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}

	// Primary wallet should be first (is_primary DESC)
	if got[0].Address != "addr2" {
		t.Errorf("first wallet = %q, want %q (primary)", got[0].Address, "addr2")
	}
	if !got[0].IsPrimary {
		t.Error("first wallet IsPrimary = false, want true")
	}

	// Non-primary wallets ordered by added_at ASC
	if got[1].Address != "addr1" {
		t.Errorf("second wallet = %q, want %q", got[1].Address, "addr1")
	}
	if got[2].Address != "addr3" {
		t.Errorf("third wallet = %q, want %q", got[2].Address, "addr3")
	}
}

func TestGetPrimaryWallet(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	w := models.Wallet{
		Address:        "primary_addr",
		Label:          "Primary",
		IsPrimary:      true,
		WalletType:     models.WalletTypeBIP44Standard,
		DerivationPath: "m/44'/501'/0'/0'",
	}
	if err := db.InsertWallet(w); err != nil {
		t.Fatalf("InsertWallet: %v", err)
	}

	got, err := db.GetPrimaryWallet()
	if err != nil {
		t.Fatalf("GetPrimaryWallet: %v", err)
	}
	if got.Address != "primary_addr" {
		t.Errorf("Address = %q, want %q", got.Address, "primary_addr")
	}
	if !got.IsPrimary {
		t.Error("IsPrimary = false, want true")
	}
}

func TestGetPrimaryWalletNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	_, err := db.GetPrimaryWallet()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrWalletNotFound) {
		t.Errorf("error = %v, want ErrWalletNotFound", err)
	}
}

func TestGetWalletByAddress(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	w := models.Wallet{
		Address:        "test_addr",
		Label:          "Test",
		WalletType:     models.WalletTypeBIP44Change,
		DerivationPath: "m/44'/501'/0'",
		AccountIndex:   0,
	}
	if err := db.InsertWallet(w); err != nil {
		t.Fatalf("InsertWallet: %v", err)
	}

	got, err := db.GetWalletByAddress("test_addr")
	if err != nil {
		t.Fatalf("GetWalletByAddress: %v", err)
	}
	if got.Label != "Test" {
		t.Errorf("Label = %q, want %q", got.Label, "Test")
	}
	if got.WalletType != models.WalletTypeBIP44Change {
		t.Errorf("WalletType = %v, want %v", got.WalletType, models.WalletTypeBIP44Change)
	}
}

func TestGetWalletByAddressNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	_, err := db.GetWalletByAddress("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrWalletNotFound) {
		t.Errorf("error = %v, want ErrWalletNotFound", err)
	}
}

func TestSetPrimaryWallet(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	wallets := []models.Wallet{
		{Address: "addr1", Label: "W1", IsPrimary: true, WalletType: models.WalletTypeLegacy, DerivationPath: "legacy-sha256"},
		{Address: "addr2", Label: "W2", IsPrimary: false, WalletType: models.WalletTypeLegacy, DerivationPath: "legacy-sha256"},
	}
	for _, w := range wallets {
		if err := db.InsertWallet(w); err != nil {
			t.Fatalf("InsertWallet(%s): %v", w.Label, err)
		}
	}

	// Switch primary to addr2
	if err := db.SetPrimaryWallet("addr2"); err != nil {
		t.Fatalf("SetPrimaryWallet: %v", err)
	}

	// Verify addr2 is now primary
	primary, err := db.GetPrimaryWallet()
	if err != nil {
		t.Fatalf("GetPrimaryWallet: %v", err)
	}
	if primary.Address != "addr2" {
		t.Errorf("primary Address = %q, want %q", primary.Address, "addr2")
	}

	// Verify addr1 is no longer primary
	w1, err := db.GetWalletByAddress("addr1")
	if err != nil {
		t.Fatalf("GetWalletByAddress(addr1): %v", err)
	}
	if w1.IsPrimary {
		t.Error("addr1 IsPrimary = true, want false after switch")
	}
}

func TestSetPrimaryWalletNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	err := db.SetPrimaryWallet("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrWalletNotFound) {
		t.Errorf("error = %v, want ErrWalletNotFound", err)
	}
}

func TestDeleteWallet(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	w := models.Wallet{
		Address:        "del_addr",
		Label:          "Delete Me",
		WalletType:     models.WalletTypeLegacy,
		DerivationPath: "legacy-sha256",
	}
	if err := db.InsertWallet(w); err != nil {
		t.Fatalf("InsertWallet: %v", err)
	}

	if err := db.DeleteWallet("del_addr"); err != nil {
		t.Fatalf("DeleteWallet: %v", err)
	}

	_, err := db.GetWalletByAddress("del_addr")
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
	if !errors.Is(err, models.ErrWalletNotFound) {
		t.Errorf("error = %v, want ErrWalletNotFound", err)
	}
}

func TestDeleteWalletNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	err := db.DeleteWallet("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrWalletNotFound) {
		t.Errorf("error = %v, want ErrWalletNotFound", err)
	}
}

func TestWalletCount(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	count, err := db.WalletCount()
	if err != nil {
		t.Fatalf("WalletCount: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	for i := range 3 {
		w := models.Wallet{
			Address:        "addr" + string(rune('A'+i)),
			Label:          "W",
			WalletType:     models.WalletTypeLegacy,
			DerivationPath: "legacy-sha256",
		}
		if err := db.InsertWallet(w); err != nil {
			t.Fatalf("InsertWallet: %v", err)
		}
	}

	count, err = db.WalletCount()
	if err != nil {
		t.Fatalf("WalletCount: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}
