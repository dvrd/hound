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
	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	pubKey := privKey.Public().(ed25519.PublicKey)

	// Build a valid transaction that passes ValidateSwapTransaction
	buildValidTx := func() []byte {
		var tx []byte
		tx = append(tx, 1)                   // 1 signature
		tx = append(tx, make([]byte, 64)...) // empty sig slot
		tx = append(tx, 1, 0, 1)             // header: 1 required sig, 0 readonly signed, 1 readonly unsigned
		tx = append(tx, 2)                   // 2 accounts
		tx = append(tx, pubKey...)           // account 0: signer
		tx = append(tx, make([]byte, 32)...) // account 1: system program (all zeros)
		tx = append(tx, make([]byte, 32)...) // blockhash
		tx = append(tx, 1)                   // 1 instruction
		tx = append(tx, 1)                   // programIdIndex = 1 (system program)
		tx = append(tx, 1)                   // 1 account index
		tx = append(tx, 0)                   // account index 0
		tx = append(tx, 4)                   // data length 4
		tx = append(tx, 0, 0, 0, 0)          // data
		return tx
	}

	tests := []struct {
		name    string
		txBytes []byte
		wantErr bool
		errIs   error
	}{
		{
			name:    "valid transaction",
			txBytes: buildValidTx(),
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
				tx := make([]byte, 1+128+32)
				tx[0] = 2
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

			signedBytes, err := base64.StdEncoding.DecodeString(signedBase64)
			if err != nil {
				t.Fatalf("decode signed tx: %v", err)
			}
			if len(signedBytes) != len(tt.txBytes) {
				t.Errorf("signed tx length %d != original %d", len(signedBytes), len(tt.txBytes))
			}
			if signedBytes[0] != 1 {
				t.Errorf("first byte changed: %d", signedBytes[0])
			}
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
			pubKeyBytes := privKey.Public().(ed25519.PublicKey)
			message := signedBytes[65:]
			sig := signedBytes[1:65]
			if !ed25519.Verify(pubKeyBytes, message, sig) {
				t.Error("signature verification failed")
			}
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

func TestValidateSwapTransaction(t *testing.T) {
	signerPubkey := make([]byte, 32)
	for i := range signerPubkey {
		signerPubkey[i] = byte(i + 1)
	}

	systemProgram := [32]byte{}

	computeBudget := [32]byte{3, 6, 70, 111, 229, 33, 23, 50, 255, 236, 173, 186, 114, 195, 155, 231, 188, 140, 229, 187, 197, 247, 18, 107, 44, 67, 155, 58, 64, 0, 0, 0}

	buildTx := func(feePayer []byte, programIdx uint8) []byte {
		var tx []byte
		tx = append(tx, 1)
		tx = append(tx, make([]byte, 64)...)
		tx = append(tx, 1, 0, 2)
		tx = append(tx, 3)
		tx = append(tx, feePayer...)
		tx = append(tx, systemProgram[:]...)
		tx = append(tx, computeBudget[:]...)
		tx = append(tx, make([]byte, 32)...)
		tx = append(tx, 1)
		tx = append(tx, programIdx)
		tx = append(tx, 1)
		tx = append(tx, 0)
		tx = append(tx, 4)
		tx = append(tx, 0, 0, 0, 0)
		return tx
	}

	t.Run("valid transaction passes", func(t *testing.T) {
		tx := buildTx(signerPubkey, 1)
		err := swap.ValidateSwapTransaction(tx, signerPubkey)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("wrong fee payer rejected", func(t *testing.T) {
		wrongSigner := make([]byte, 32)
		wrongSigner[0] = 0xFF
		tx := buildTx(signerPubkey, 1)
		err := swap.ValidateSwapTransaction(tx, wrongSigner)
		if err == nil {
			t.Fatal("expected error for wrong fee payer")
		}
		if !errors.Is(err, models.ErrUntrustedTransaction) {
			t.Errorf("expected ErrUntrustedTransaction, got: %v", err)
		}
	})

	t.Run("unknown program rejected", func(t *testing.T) {
		var tx []byte
		tx = append(tx, 1)
		tx = append(tx, make([]byte, 64)...)
		tx = append(tx, 1, 0, 3)
		tx = append(tx, 4)
		tx = append(tx, signerPubkey...)
		tx = append(tx, systemProgram[:]...)
		tx = append(tx, computeBudget[:]...)
		unknownProgram := make([]byte, 32)
		unknownProgram[0] = 0xDE
		unknownProgram[1] = 0xAD
		tx = append(tx, unknownProgram...)
		tx = append(tx, make([]byte, 32)...)
		tx = append(tx, 1)
		tx = append(tx, 3)
		tx = append(tx, 1)
		tx = append(tx, 0)
		tx = append(tx, 4)
		tx = append(tx, 0, 0, 0, 0)
		err := swap.ValidateSwapTransaction(tx, signerPubkey)
		if err == nil {
			t.Fatal("expected error for unknown program")
		}
		if !errors.Is(err, models.ErrUntrustedTransaction) {
			t.Errorf("expected ErrUntrustedTransaction, got: %v", err)
		}
	})

	t.Run("too short transaction", func(t *testing.T) {
		err := swap.ValidateSwapTransaction(make([]byte, 10), signerPubkey)
		if err == nil {
			t.Fatal("expected error for short tx")
		}
	})
}
