package database

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func makeTestHyperliquidData(addr, label string) EncryptedHyperliquidData {
	data := EncryptedHyperliquidData{
		Address:            addr,
		Label:              label,
		APIWalletName:      "test-api-wallet",
		EncryptedAPIKey:    make([]byte, 32),
		EncryptedAPISecret: make([]byte, 48),
		IsActive:           false,
	}
	rand.Read(data.EncryptedAPIKey)
	rand.Read(data.EncryptedAPISecret)
	rand.Read(data.Salt[:])
	rand.Read(data.NonceKey[:])
	rand.Read(data.NonceSecret[:])
	rand.Read(data.TagKey[:])
	rand.Read(data.TagSecret[:])
	rand.Read(data.PasswordHash[:])
	return data
}

func TestSaveAndLoadHyperliquidWallets(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	data1 := makeTestHyperliquidData("hl_addr_1", "HL Wallet 1")
	data2 := makeTestHyperliquidData("hl_addr_2", "HL Wallet 2")

	if err := db.SaveHyperliquidWallet(data1); err != nil {
		t.Fatalf("SaveHyperliquidWallet(1): %v", err)
	}
	if err := db.SaveHyperliquidWallet(data2); err != nil {
		t.Fatalf("SaveHyperliquidWallet(2): %v", err)
	}

	// LoadHyperliquidWallets returns minimal data (no encrypted fields)
	wallets, err := db.LoadHyperliquidWallets()
	if err != nil {
		t.Fatalf("LoadHyperliquidWallets: %v", err)
	}

	if len(wallets) != 2 {
		t.Fatalf("len = %d, want 2", len(wallets))
	}

	// Verify minimal fields are populated
	found := false
	for _, w := range wallets {
		if w.Address == "hl_addr_1" {
			found = true
			if w.Label != "HL Wallet 1" {
				t.Errorf("Label = %q, want %q", w.Label, "HL Wallet 1")
			}
			if w.APIWalletName != "test-api-wallet" {
				t.Errorf("APIWalletName = %q, want %q", w.APIWalletName, "test-api-wallet")
			}
			// Encrypted fields should be zero/nil (not loaded)
			if len(w.EncryptedAPIKey) != 0 {
				t.Error("EncryptedAPIKey should be empty in minimal load")
			}
		}
	}
	if !found {
		t.Error("hl_addr_1 not found in results")
	}
}

func TestLoadHyperliquidWalletCredentials(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	data := makeTestHyperliquidData("hl_addr_1", "HL Wallet 1")
	if err := db.SaveHyperliquidWallet(data); err != nil {
		t.Fatalf("SaveHyperliquidWallet: %v", err)
	}

	got, err := db.LoadHyperliquidWalletCredentials("hl_addr_1")
	if err != nil {
		t.Fatalf("LoadHyperliquidWalletCredentials: %v", err)
	}

	// Verify all fields including encrypted ones
	if got.Address != data.Address {
		t.Errorf("Address = %q, want %q", got.Address, data.Address)
	}
	if got.Label != data.Label {
		t.Errorf("Label = %q, want %q", got.Label, data.Label)
	}
	if !bytes.Equal(got.EncryptedAPIKey, data.EncryptedAPIKey) {
		t.Error("EncryptedAPIKey mismatch")
	}
	if !bytes.Equal(got.EncryptedAPISecret, data.EncryptedAPISecret) {
		t.Error("EncryptedAPISecret mismatch")
	}
	if got.Salt != data.Salt {
		t.Error("Salt mismatch")
	}
	if got.NonceKey != data.NonceKey {
		t.Error("NonceKey mismatch")
	}
	if got.NonceSecret != data.NonceSecret {
		t.Error("NonceSecret mismatch")
	}
	if got.TagKey != data.TagKey {
		t.Error("TagKey mismatch")
	}
	if got.TagSecret != data.TagSecret {
		t.Error("TagSecret mismatch")
	}
	if got.PasswordHash != data.PasswordHash {
		t.Error("PasswordHash mismatch")
	}
}

func TestLoadHyperliquidWalletCredentialsNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	_, err := db.LoadHyperliquidWalletCredentials("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrWalletNotFound) {
		t.Errorf("error = %v, want ErrWalletNotFound", err)
	}
}

func TestSetActiveHyperliquidWallet(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	data1 := makeTestHyperliquidData("hl_addr_1", "HL 1")
	data1.IsActive = true
	data2 := makeTestHyperliquidData("hl_addr_2", "HL 2")

	if err := db.SaveHyperliquidWallet(data1); err != nil {
		t.Fatalf("SaveHyperliquidWallet(1): %v", err)
	}
	if err := db.SaveHyperliquidWallet(data2); err != nil {
		t.Fatalf("SaveHyperliquidWallet(2): %v", err)
	}

	// Switch active to addr2
	if err := db.SetActiveHyperliquidWallet("hl_addr_2"); err != nil {
		t.Fatalf("SetActiveHyperliquidWallet: %v", err)
	}

	// Verify addr2 is now active
	active, err := db.GetActiveHyperliquidWallet()
	if err != nil {
		t.Fatalf("GetActiveHyperliquidWallet: %v", err)
	}
	if active.Address != "hl_addr_2" {
		t.Errorf("active Address = %q, want %q", active.Address, "hl_addr_2")
	}

	// Verify addr1 is no longer active
	wallets, err := db.LoadHyperliquidWallets()
	if err != nil {
		t.Fatalf("LoadHyperliquidWallets: %v", err)
	}
	for _, w := range wallets {
		if w.Address == "hl_addr_1" && w.IsActive {
			t.Error("hl_addr_1 should not be active after switch")
		}
	}
}

func TestSetActiveHyperliquidWalletNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	err := db.SetActiveHyperliquidWallet("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrWalletNotFound) {
		t.Errorf("error = %v, want ErrWalletNotFound", err)
	}
}

func TestGetActiveHyperliquidWalletNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	_, err := db.GetActiveHyperliquidWallet()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrWalletNotFound) {
		t.Errorf("error = %v, want ErrWalletNotFound", err)
	}
}
