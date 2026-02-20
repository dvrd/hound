package swap_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/swap"
)

func TestSignTransaction(t *testing.T) {
	// Generate a test keypair
	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	tests := []struct {
		name    string
		txBytes []byte
		wantErr bool
		errIs   error
	}{
		{
			name: "valid transaction",
			txBytes: func() []byte {
				// 1 byte (num sigs = 1) + 64 bytes (empty sig) + some message bytes
				tx := make([]byte, 1+64+32)
				tx[0] = 1 // 1 signature slot
				// Leave signature as zeros
				// Fill message with some data
				for i := 65; i < len(tx); i++ {
					tx[i] = byte(i)
				}
				return tx
			}(),
		},
		{
			name:    "too short",
			txBytes: make([]byte, 10),
			wantErr: true,
			errIs:   models.ErrInvalidTransaction,
		},
		{
			name: "wrong signature count",
			txBytes: func() []byte {
				tx := make([]byte, 1+128+32) // 2 sig slots
				tx[0] = 2                    // 2 signature slots
				return tx
			}(),
			wantErr: true,
			errIs:   models.ErrInvalidTransaction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txBase64 := base64.StdEncoding.EncodeToString(tt.txBytes)

			signedBase64, err := swap.SignTransaction(txBase64, privKey)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errIs != nil && !errors.Is(err, tt.errIs) {
					t.Errorf("expected error %v, got %v", tt.errIs, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Decode signed transaction
			signedBytes, err := base64.StdEncoding.DecodeString(signedBase64)
			if err != nil {
				t.Fatalf("decode signed tx: %v", err)
			}

			// Verify structure preserved
			if len(signedBytes) != len(tt.txBytes) {
				t.Errorf("signed tx length %d != original %d", len(signedBytes), len(tt.txBytes))
			}

			// Verify first byte unchanged
			if signedBytes[0] != 1 {
				t.Errorf("first byte changed: %d", signedBytes[0])
			}

			// Verify signature is not all zeros
			allZero := true
			for _, b := range signedBytes[1:65] {
				if b != 0 {
					allZero = false
					break
				}
			}
			if allZero {
				t.Error("signature is all zeros after signing")
			}

			// Verify signature is valid
			pubKey := privKey.Public().(ed25519.PublicKey)
			message := signedBytes[65:]
			sig := signedBytes[1:65]
			if !ed25519.Verify(pubKey, message, sig) {
				t.Error("signature verification failed")
			}

			// Verify message bytes unchanged
			for i := 65; i < len(signedBytes); i++ {
				if signedBytes[i] != tt.txBytes[i] {
					t.Errorf("message byte %d changed: %d != %d", i, signedBytes[i], tt.txBytes[i])
					break
				}
			}
		})
	}
}

func TestSignTransaction_InvalidBase64(t *testing.T) {
	_, privKey, _ := ed25519.GenerateKey(nil)
	_, err := swap.SignTransaction("not-valid-base64!!!", privKey)
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
	if !errors.Is(err, models.ErrInvalidTransaction) {
		t.Errorf("expected ErrInvalidTransaction, got: %v", err)
	}
}

func TestSubmitTransaction(t *testing.T) {
	tests := []struct {
		name         string
		serverResp   string
		serverStatus int
		wantErr      bool
		checkResult  func(t *testing.T, r models.SwapTransactionResult)
	}{
		{
			name:         "successful submission",
			serverStatus: http.StatusOK,
			serverResp: `{
				"signature": "5xGz...abc",
				"status": "confirmed",
				"slot": 123456789
			}`,
			checkResult: func(t *testing.T, r models.SwapTransactionResult) {
				if r.Signature != "5xGz...abc" {
					t.Errorf("unexpected signature: %s", r.Signature)
				}
				if r.Status != "confirmed" {
					t.Errorf("unexpected status: %s", r.Status)
				}
				if r.Slot != 123456789 {
					t.Errorf("unexpected slot: %d", r.Slot)
				}
			},
		},
		{
			name:         "server error",
			serverStatus: http.StatusInternalServerError,
			serverResp:   `{"error":"transaction failed"}`,
			wantErr:      true,
			checkResult: func(t *testing.T, r models.SwapTransactionResult) {
				if r.Status != "failed" {
					t.Errorf("expected failed status, got: %s", r.Status)
				}
			},
		},
		{
			name:         "empty status defaults to confirmed",
			serverStatus: http.StatusOK,
			serverResp: `{
				"signature": "sig123",
				"slot": 100
			}`,
			checkResult: func(t *testing.T, r models.SwapTransactionResult) {
				if r.Status != "confirmed" {
					t.Errorf("expected confirmed status, got: %s", r.Status)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.URL.Path != "/ultra/v1/execute" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("unexpected method: %s", r.Method)
				}

				// Verify request body
				var body struct {
					SignedTransaction string `json:"signedTransaction"`
					RequestID         string `json:"requestId"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				if body.SignedTransaction != "signed-tx-base64" {
					t.Errorf("unexpected signedTransaction: %s", body.SignedTransaction)
				}
				if body.RequestID != "req-123" {
					t.Errorf("unexpected requestId: %s", body.RequestID)
				}

				w.WriteHeader(tt.serverStatus)
				fmt.Fprint(w, tt.serverResp)
			}))
			defer server.Close()

			result, err := swap.SubmitTransaction(
				server.Client(),
				server.URL,
				"signed-tx-base64",
				"req-123",
			)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestSubmitTransaction_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{invalid json`)
	}))
	defer server.Close()

	_, err := swap.SubmitTransaction(server.Client(), server.URL, "tx", "req")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !errors.Is(err, models.ErrInvalidResponse) {
		t.Errorf("expected ErrInvalidResponse, got: %v", err)
	}
}
