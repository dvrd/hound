package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
)

func setupTransferTest(t *testing.T) (*database.Database, *services.TransferService, string) {
	t.Helper()
	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	keystoreSvc := &services.KeystoreService{}
	transferSvc := services.NewTransferService(keystoreSvc, db)

	// Import a test wallet
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")
	password := "MyStr0ng!Pass#1"
	addr, err := keystoreSvc.ImportKeypair(db, words, password, "test-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportKeypair failed: %v", err)
	}

	return db, transferSvc, addr
}

func TestTransfer_SendSOL_InvalidRecipient(t *testing.T) {
	_, transferSvc, addr := setupTransferTest(t)
	client := blockchain.NewRPCClient("http://localhost:0", nil)

	_, err := transferSvc.SendSOL(client, addr, "not-valid-base58!!!", 100, "MyStr0ng!Pass#1")
	if err == nil {
		t.Fatal("expected error for invalid recipient")
	}
	if !errors.Is(err, models.ErrInvalidRecipient) {
		t.Errorf("expected ErrInvalidRecipient, got: %v", err)
	}
}

func TestTransfer_SendSOL_SendToSelf(t *testing.T) {
	_, transferSvc, addr := setupTransferTest(t)
	client := blockchain.NewRPCClient("http://localhost:0", nil)

	_, err := transferSvc.SendSOL(client, addr, addr, 100, "MyStr0ng!Pass#1")
	if err == nil {
		t.Fatal("expected error for send to self")
	}
	if !errors.Is(err, models.ErrSendToSelf) {
		t.Errorf("expected ErrSendToSelf, got: %v", err)
	}
}

func TestTransfer_SendSOL_WrongPassword(t *testing.T) {
	_, transferSvc, addr := setupTransferTest(t)

	// Need a valid recipient address
	recipient := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	// Mock server for blockhash (won't be reached due to password failure)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]interface{}{"value": map[string]interface{}{"blockhash": "GHtXQBsoZHVnNFa9YevAzFr17DJjgHXk3ycTKD5xD3Zi", "lastValidBlockHeight": 12345}},
		})
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	_, err := transferSvc.SendSOL(client, addr, recipient, 100, "Wr0ng!Password#9")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if !errors.Is(err, models.ErrCryptoFailed) {
		t.Errorf("expected ErrCryptoFailed, got: %v", err)
	}
}

func TestTransfer_SendSOL_Success(t *testing.T) {
	_, transferSvc, addr := setupTransferTest(t)

	recipient := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	expectedSig := "5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		var result interface{}
		switch req.Method {
		case "getBalance":
			result = map[string]interface{}{
				"value": 10_000_000_000, // 10 SOL — plenty for the transfer
			}
		case "getLatestBlockhash":
			result = map[string]interface{}{
				"value": map[string]interface{}{
					"blockhash":            "GHtXQBsoZHVnNFa9YevAzFr17DJjgHXk3ycTKD5xD3Zi",
					"lastValidBlockHeight": 12345,
				},
			}
		case "sendTransaction":
			result = expectedSig
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	sig, err := transferSvc.SendSOL(client, addr, recipient, 1_000_000_000, "MyStr0ng!Pass#1")
	if err != nil {
		t.Fatalf("SendSOL failed: %v", err)
	}
	if sig != expectedSig {
		t.Errorf("signature = %q, want %q", sig, expectedSig)
	}
}

func TestTransfer_SendSOL_InsufficientBalance(t *testing.T) {
	_, transferSvc, addr := setupTransferTest(t)

	recipient := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		var result interface{}
		switch req.Method {
		case "getBalance":
			result = map[string]interface{}{
				"value": 1000, // Only 1000 lamports — not enough
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	_, err := transferSvc.SendSOL(client, addr, recipient, 1_000_000_000, "MyStr0ng!Pass#1")
	if err == nil {
		t.Fatal("expected error for insufficient balance")
	}
	if !errors.Is(err, models.ErrInsufficientBalance) {
		t.Errorf("expected ErrInsufficientBalance, got: %v", err)
	}
}

func TestTransfer_EstimateFee(t *testing.T) {
	svc := services.NewTransferService(nil, nil)

	fee := svc.EstimateFee(false)
	if fee != 5000 {
		t.Errorf("EstimateFee(false) = %d, want 5000", fee)
	}

	feeWithATA := svc.EstimateFee(true)
	if feeWithATA != 5000+2_039_280 {
		t.Errorf("EstimateFee(true) = %d, want %d", feeWithATA, 5000+2_039_280)
	}
}

func TestAwaitConfirmation_Confirmed(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		count := callCount.Add(1)

		var status string
		if count >= 2 {
			status = `{"slot":100,"confirmationStatus":"confirmed","err":null}`
		} else {
			status = "null"
		}
		resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"value":[%s]}}`, req.ID, status)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	err := services.AwaitConfirmation(context.Background(), client, "test-sig", 10*time.Second)
	if err != nil {
		t.Fatalf("expected confirmation, got error: %v", err)
	}
}

func TestAwaitConfirmation_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"value":[{"slot":100,"confirmationStatus":"confirmed","err":{"InstructionError":[0,"Custom"]}}]}}`, req.ID)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	err := services.AwaitConfirmation(context.Background(), client, "test-sig", 10*time.Second)
	if err == nil {
		t.Fatal("expected error for failed tx")
	}
	if !errors.Is(err, models.ErrTransactionFailed) {
		t.Errorf("expected ErrTransactionFailed, got: %v", err)
	}
}

func TestAwaitConfirmation_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		// Always return null (not found)
		resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"value":[null]}}`, req.ID)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	err := services.AwaitConfirmation(context.Background(), client, "test-sig", 3*time.Second)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, models.ErrConfirmationTimeout) {
		t.Errorf("expected ErrConfirmationTimeout, got: %v", err)
	}
}
