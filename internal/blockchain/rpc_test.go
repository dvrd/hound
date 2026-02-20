package blockchain_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/models"
)

func TestCallSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := blockchain.RPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"value": 1000000000}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	result, err := client.Call("getBalance", []interface{}{"address123"})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	var parsed struct {
		Value uint64 `json:"value"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if parsed.Value != 1000000000 {
		t.Errorf("expected value 1000000000, got %d", parsed.Value)
	}
}

func TestCallFailoverToBackup(t *testing.T) {
	var primaryHits atomic.Int32

	// Primary server always fails
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryHits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer primary.Close()

	// Backup server succeeds
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := blockchain.RPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"value": 42}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer backup.Close()

	client := blockchain.NewRPCClient(primary.URL, []string{backup.URL})
	result, err := client.Call("getBalance", []interface{}{"address123"})
	if err != nil {
		t.Fatalf("Call with failover failed: %v", err)
	}

	var parsed struct {
		Value uint64 `json:"value"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if parsed.Value != 42 {
		t.Errorf("expected value 42, got %d", parsed.Value)
	}

	if primaryHits.Load() != 1 {
		t.Errorf("expected primary to be hit once, got %d", primaryHits.Load())
	}
}

func TestCallAllEndpointsFail(t *testing.T) {
	// Primary fails
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer primary.Close()

	// Backup also fails
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer backup.Close()

	client := blockchain.NewRPCClient(primary.URL, []string{backup.URL})
	_, err := client.Call("getBalance", []interface{}{"address123"})
	if err == nil {
		t.Fatal("expected error when all endpoints fail")
	}
	if !errors.Is(err, models.ErrRPCConnectionFailed) {
		t.Errorf("expected ErrRPCConnectionFailed, got: %v", err)
	}
}

func TestCallRPCErrorDetected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := blockchain.RPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Error: &blockchain.RPCError{
				Code:    -32600,
				Message: "Invalid Request",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	_, err := client.Call("invalidMethod", nil)
	if err == nil {
		t.Fatal("expected error for RPC error response")
	}
	// With no backup, it should wrap as ErrRPCConnectionFailed
	if !errors.Is(err, models.ErrRPCConnectionFailed) {
		t.Errorf("expected ErrRPCConnectionFailed wrapping RPC error, got: %v", err)
	}
}

func TestRequestIDIncrements(t *testing.T) {
	var ids []int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		ids = append(ids, req.ID)

		resp := blockchain.RPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)

	for i := 0; i < 3; i++ {
		_, err := client.Call("test", nil)
		if err != nil {
			t.Fatalf("Call %d failed: %v", i, err)
		}
	}

	if len(ids) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(ids))
	}

	// IDs should be strictly increasing
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Errorf("request IDs not increasing: %v", ids)
			break
		}
	}
}
