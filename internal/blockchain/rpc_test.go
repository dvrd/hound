package blockchain_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	result, err := client.Call(context.Background(), "getBalance", []interface{}{"address123"})
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
	result, err := client.Call(context.Background(), "getBalance", []interface{}{"address123"})
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
	_, err := client.Call(context.Background(), "getBalance", []interface{}{"address123"})
	if err == nil {
		t.Fatal("expected error when all endpoints fail")
	}
	if !errors.Is(err, models.ErrRPCConnectionFailed) {
		t.Errorf("expected ErrRPCConnectionFailed, got: %v", err)
	}
}

// H10: RPC errors should NOT trigger endpoint rotation — return immediately.
func TestCallRPCErrorNoRotation(t *testing.T) {
	var hitCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount.Add(1)
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

	// Give it a backup to prove it does NOT rotate
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount.Add(1)
		t.Error("backup should NOT be hit for RPC errors")
		resp := blockchain.RPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer backup.Close()

	client := blockchain.NewRPCClient(server.URL, []string{backup.URL})
	_, err := client.Call(context.Background(), "invalidMethod", nil)
	if err == nil {
		t.Fatal("expected error for RPC error response")
	}

	// Should be the RPCError directly, not wrapped in ErrRPCConnectionFailed
	var rpcErr *blockchain.RPCError
	if !errors.As(err, &rpcErr) {
		t.Errorf("expected *RPCError, got: %T: %v", err, err)
	}

	// Only the primary should have been hit (no rotation)
	if hitCount.Load() != 1 {
		t.Errorf("expected 1 hit (no rotation), got %d", hitCount.Load())
	}
}

func TestRequestIDIncrements(t *testing.T) {
	var ids []int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		ids = append(ids, req.ID)
		mu.Unlock()

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
		_, err := client.Call(context.Background(), "test", nil)
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

// H1: Verify concurrent calls don't deadlock and complete independently.
func TestCallConcurrentNoDeadlock(t *testing.T) {
	var hitCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount.Add(1)
		// Simulate some latency
		time.Sleep(10 * time.Millisecond)
		resp := blockchain.RPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"value": 1}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)

	const numGoroutines = 10
	var wg sync.WaitGroup
	errs := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = client.Call(context.Background(), "test", nil)
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d failed: %v", i, err)
		}
	}

	if hitCount.Load() != numGoroutines {
		t.Errorf("expected %d hits, got %d", numGoroutines, hitCount.Load())
	}
}

// TestCall429_RotatesToBackupAfterDelay verifies that a 429 from the primary
// causes the client to wait (briefly) then rotate to the backup endpoint.
func TestCall429_RotatesToBackupAfterDelay(t *testing.T) {
	var primaryHits, backupHits atomic.Int32

	// Primary always returns 429 with a tiny Retry-After
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryHits.Add(1)
		w.Header().Set("Retry-After", "0") // 0 → parsed as 0, falls back to 1s default; use "" to test default
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer primary.Close()

	// Backup succeeds
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backupHits.Add(1)
		resp := blockchain.RPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"value": 42}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer backup.Close()

	client := blockchain.NewRPCClient(primary.URL, []string{backup.URL})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := client.Call(ctx, "getBalance", nil)
	if err != nil {
		t.Fatalf("expected success from backup, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result from backup")
	}
	if primaryHits.Load() == 0 {
		t.Error("primary should have been hit at least once")
	}
	if backupHits.Load() == 0 {
		t.Error("backup should have been hit after primary 429")
	}
}

// TestCall429_ContextCancelDuring429Wait verifies context cancellation is
// respected while waiting out a 429 Retry-After delay.
func TestCall429_ContextCancelDuring429Wait(t *testing.T) {
	// Primary always returns 429 with a long Retry-After
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60") // 60s — context should cancel first
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer primary.Close()

	client := blockchain.NewRPCClient(primary.URL, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Call(ctx, "getBalance", nil)
	if err == nil {
		t.Fatal("expected error from context cancellation during 429 wait")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("expected context error, got: %v", err)
	}
}

// M2: Verify context cancellation aborts in-flight request.
func TestCallContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow server
		time.Sleep(5 * time.Second)
		resp := blockchain.RPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Call(ctx, "test", nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
