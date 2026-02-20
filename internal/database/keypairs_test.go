package database

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func makeTestKeypairData(addr string) EncryptedKeypairData {
	data := EncryptedKeypairData{
		Address:             addr,
		EncryptedPrivateKey: make([]byte, 64),
		Label:               "Test Keypair",
		IsPrimary:           true,
	}
	// Fill with random data to test BLOB round-trip
	rand.Read(data.EncryptedPrivateKey)
	rand.Read(data.Salt[:])
	rand.Read(data.Nonce[:])
	rand.Read(data.Tag[:])
	rand.Read(data.PasswordHash[:])
	return data
}

func TestInsertAndGetEncryptedKeypair(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	data := makeTestKeypairData("kp_addr_1")

	if err := db.InsertEncryptedKeypair(data); err != nil {
		t.Fatalf("InsertEncryptedKeypair: %v", err)
	}

	got, err := db.GetEncryptedKeypair("kp_addr_1")
	if err != nil {
		t.Fatalf("GetEncryptedKeypair: %v", err)
	}

	// Verify all fields
	if got.Address != data.Address {
		t.Errorf("Address = %q, want %q", got.Address, data.Address)
	}
	if !bytes.Equal(got.EncryptedPrivateKey, data.EncryptedPrivateKey) {
		t.Error("EncryptedPrivateKey mismatch")
	}
	if got.Salt != data.Salt {
		t.Error("Salt mismatch")
	}
	if got.Nonce != data.Nonce {
		t.Error("Nonce mismatch")
	}
	if got.Tag != data.Tag {
		t.Error("Tag mismatch")
	}
	if got.PasswordHash != data.PasswordHash {
		t.Error("PasswordHash mismatch")
	}
	if got.Label != data.Label {
		t.Errorf("Label = %q, want %q", got.Label, data.Label)
	}
	if got.IsPrimary != data.IsPrimary {
		t.Errorf("IsPrimary = %v, want %v", got.IsPrimary, data.IsPrimary)
	}
}

func TestGetEncryptedKeypairNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	_, err := db.GetEncryptedKeypair("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrKeyNotFound) {
		t.Errorf("error = %v, want ErrKeyNotFound", err)
	}
}

func TestUpdateEncryptedKeypair(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	data := makeTestKeypairData("kp_addr_1")
	if err := db.InsertEncryptedKeypair(data); err != nil {
		t.Fatalf("InsertEncryptedKeypair: %v", err)
	}

	// Update with new data
	data.Label = "Updated Label"
	data.IsPrimary = false
	rand.Read(data.EncryptedPrivateKey)
	rand.Read(data.Salt[:])

	if err := db.UpdateEncryptedKeypair(data); err != nil {
		t.Fatalf("UpdateEncryptedKeypair: %v", err)
	}

	got, err := db.GetEncryptedKeypair("kp_addr_1")
	if err != nil {
		t.Fatalf("GetEncryptedKeypair: %v", err)
	}

	if got.Label != "Updated Label" {
		t.Errorf("Label = %q, want %q", got.Label, "Updated Label")
	}
	if got.IsPrimary {
		t.Error("IsPrimary = true, want false")
	}
	if got.Salt != data.Salt {
		t.Error("Salt mismatch after update")
	}
}

func TestUpdateEncryptedKeypairNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	data := makeTestKeypairData("nonexistent")
	err := db.UpdateEncryptedKeypair(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrKeyNotFound) {
		t.Errorf("error = %v, want ErrKeyNotFound", err)
	}
}

func TestUpdateKeypairLastUsed(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	data := makeTestKeypairData("kp_addr_1")
	if err := db.InsertEncryptedKeypair(data); err != nil {
		t.Fatalf("InsertEncryptedKeypair: %v", err)
	}

	// Get initial last_used
	initial, err := db.GetEncryptedKeypair("kp_addr_1")
	if err != nil {
		t.Fatalf("GetEncryptedKeypair: %v", err)
	}

	if err := db.UpdateKeypairLastUsed("kp_addr_1"); err != nil {
		t.Fatalf("UpdateKeypairLastUsed: %v", err)
	}

	updated, err := db.GetEncryptedKeypair("kp_addr_1")
	if err != nil {
		t.Fatalf("GetEncryptedKeypair: %v", err)
	}

	if updated.LastUsed < initial.LastUsed {
		t.Errorf("LastUsed went backwards: %d < %d", updated.LastUsed, initial.LastUsed)
	}
}

func TestUpdateKeypairLastUsedNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	err := db.UpdateKeypairLastUsed("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrKeyNotFound) {
		t.Errorf("error = %v, want ErrKeyNotFound", err)
	}
}

func TestBlobFieldsRoundTrip(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	// Create data with specific known byte patterns
	data := EncryptedKeypairData{
		Address:             "blob_test",
		EncryptedPrivateKey: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		Label:               "Blob Test",
	}
	// Set specific patterns for fixed-size arrays
	for i := range data.Salt {
		data.Salt[i] = byte(i)
	}
	for i := range data.Nonce {
		data.Nonce[i] = byte(i + 100)
	}
	for i := range data.Tag {
		data.Tag[i] = byte(i + 200)
	}
	for i := range data.PasswordHash {
		data.PasswordHash[i] = byte(i + 50)
	}

	if err := db.InsertEncryptedKeypair(data); err != nil {
		t.Fatalf("InsertEncryptedKeypair: %v", err)
	}

	got, err := db.GetEncryptedKeypair("blob_test")
	if err != nil {
		t.Fatalf("GetEncryptedKeypair: %v", err)
	}

	// Verify exact byte-level match
	if !bytes.Equal(got.EncryptedPrivateKey, data.EncryptedPrivateKey) {
		t.Errorf("EncryptedPrivateKey = %x, want %x", got.EncryptedPrivateKey, data.EncryptedPrivateKey)
	}
	for i := range data.Salt {
		if got.Salt[i] != data.Salt[i] {
			t.Errorf("Salt[%d] = %d, want %d", i, got.Salt[i], data.Salt[i])
		}
	}
	for i := range data.Nonce {
		if got.Nonce[i] != data.Nonce[i] {
			t.Errorf("Nonce[%d] = %d, want %d", i, got.Nonce[i], data.Nonce[i])
		}
	}
	for i := range data.Tag {
		if got.Tag[i] != data.Tag[i] {
			t.Errorf("Tag[%d] = %d, want %d", i, got.Tag[i], data.Tag[i])
		}
	}
	for i := range data.PasswordHash {
		if got.PasswordHash[i] != data.PasswordHash[i] {
			t.Errorf("PasswordHash[%d] = %d, want %d", i, got.PasswordHash[i], data.PasswordHash[i])
		}
	}
}
