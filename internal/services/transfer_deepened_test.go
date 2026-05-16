package services_test

import (
	"errors"
	"testing"

	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
)

func TestValidateRecipient_Valid(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewTransferService(services.NewKeystoreService(db), db)

	// Valid base58 Solana address (32-byte ed25519 pubkey)
	err := svc.ValidateRecipient("11111111111111111111111111111111", "otherAddress")
	if err != nil {
		t.Errorf("expected nil for valid address, got: %v", err)
	}
}

func TestValidateRecipient_InvalidAddress(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewTransferService(services.NewKeystoreService(db), db)

	err := svc.ValidateRecipient("not_a_real_address!!!", "someAddr")
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
	if !errors.Is(err, models.ErrInvalidRecipient) {
		t.Errorf("expected ErrInvalidRecipient, got: %v", err)
	}
}

func TestValidateRecipient_SelfSend(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewTransferService(services.NewKeystoreService(db), db)

	addr := "11111111111111111111111111111111"
	err := svc.ValidateRecipient(addr, addr)
	if err == nil {
		t.Fatal("expected error for self-send")
	}
	if !errors.Is(err, models.ErrSendToSelf) {
		t.Errorf("expected ErrSendToSelf, got: %v", err)
	}
}

func TestEstimateSendFee_SOL(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewTransferService(services.NewKeystoreService(db), db)

	fee := svc.EstimateSendFee(true)
	if fee != 5000 {
		t.Errorf("SOL fee = %d, want 5000", fee)
	}
}

func TestEstimateSendFee_SPL(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewTransferService(services.NewKeystoreService(db), db)

	fee := svc.EstimateSendFee(false)
	expected := uint64(5000 + 2_039_280)
	if fee != expected {
		t.Errorf("SPL fee = %d, want %d", fee, expected)
	}
}
