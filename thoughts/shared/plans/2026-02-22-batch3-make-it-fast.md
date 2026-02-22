# Batch 3 — "Make It Fast" Implementation Plan

**Goal:** Eliminate the global RPC mutex bottleneck, add context.Context plumbing for cancellation, parallelize GetTransaction fan-out, and batch PersistPortfolio DB writes in a single transaction.

**Architecture:** The RPC client's `Call()` method currently holds a mutex for the entire HTTP round-trip, serializing all concurrent RPC calls. We narrow the mutex to just ID increment + endpoint selection, add `context.Context` as first param to `Call()` and all 11 wrapper functions in `solana.go`, propagate context through all callers (passing `context.Background()` for now), fix endpoint rotation to only trigger on transport failures (not RPC errors), parallelize `GetTransaction` calls in `activity.go` with a semaphore, and wrap `PersistPortfolio` in a single DB transaction.

**Design:** `thoughts/shared/designs/2026-02-22-batch3-make-it-fast-design.md`

---

## Call-Site Analysis

Before implementation, here is the complete map of what calls what:

### blockchain package functions and their callers:

| Function | Callers |
|----------|---------|
| `RPCClient.Call()` | All 11 functions in `solana.go` |
| `GetBalance()` | `wallet/balance.go`, `services/transfer.go` (×2) |
| `GetTokenAccountsByOwner()` | `wallet/balance.go`, `services/transfer.go` |
| `GetAccountInfo()` | `services/transfer.go` |
| `GetTokenAccountBalance()` | Tests only |
| `GetTokenSupply()` | `services/token_info.go` |
| `GetTokenLargestAccounts()` | `services/token_info.go` |
| `GetLatestBlockhash()` | `services/transfer.go` (×2) |
| `SendTransaction()` | `services/transfer.go` (×2) |
| `GetSignaturesForAddress()` | `services/activity.go` |
| `GetTransaction()` | `services/activity.go` |
| `GetMinimumBalanceForRentExemption()` | Tests only |
| `GetSOLPriceCached()` | `wallet/balance.go`, `dex/router.go` (via func pointer) |

### Files that do NOT call blockchain directly (no changes needed):
- `services/keystore.go` — pure crypto, no RPC
- `services/swap.go` — uses `swap.SwapClient` (Jupiter HTTP), not blockchain
- `services/pool.go` — uses `dex.DexScreenerClient`, not blockchain
- `services/price.go` — uses `dex.Router`/`dex.DexScreenerClient`/`dex.JupiterClient`, not blockchain
- `internal/swap/*` — Jupiter HTTP API only
- `internal/tui/views/*` — call services, not blockchain directly
- `internal/dex/pool.go`, `dex/dexscreener.go`, `dex/jupiter.go` — HTTP clients, not blockchain

---

## Dependency Graph

```
Batch 1 (parallel): 1.1, 1.2 [foundation — database + RPC core]
Batch 2 (parallel): 2.1, 2.2 [blockchain wrappers — depends on batch 1]
Batch 3 (parallel): 3.1, 3.2, 3.3, 3.4, 3.5 [callers — depends on batch 2]
Batch 4 (parallel): 4.1, 4.2 [feature work — depends on batch 3]
```

---

## Batch 1: Foundation (parallel — 2 implementers)

All tasks in this batch have NO dependencies and run simultaneously.

### Task 1.1: Narrow RPC Mutex + Context + Transport-Only Rotation
**File:** `internal/blockchain/rpc.go`
**Test:** `internal/blockchain/rpc_test.go`
**Depends:** none

This is the core fix. Three changes in one file:
1. **H1:** Narrow mutex to just ID increment + endpoint copy
2. **H10:** Only rotate endpoints on transport failures, return RPC errors immediately
3. **M2:** Add `ctx context.Context` as first param to `Call()`

**Implementation — `internal/blockchain/rpc.go`:**

```go
package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/dvrd/hound/internal/models"
)

// RPCClient is a Solana JSON-RPC client with endpoint failover.
type RPCClient struct {
	endpoint        string
	backupEndpoints []string
	currentIndex    int
	requestID       int
	httpClient      *http.Client
	mu              sync.Mutex
}

// RPCRequest is a JSON-RPC 2.0 request.
type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// RPCResponse is a JSON-RPC 2.0 response.
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// NewRPCClient creates a new Solana RPC client.
func NewRPCClient(endpoint string, backupEndpoints []string) *RPCClient {
	return &RPCClient{
		endpoint:        endpoint,
		backupEndpoints: backupEndpoints,
		currentIndex:    0,
		requestID:       1,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Call makes a JSON-RPC call with endpoint failover.
// The mutex is only held for ID increment and endpoint selection, NOT during HTTP I/O.
// Endpoint rotation only happens on transport failures (HTTP errors, read errors, non-200 status,
// JSON unmarshal errors). RPC-level errors (rpcResp.Error) are returned immediately without rotation.
func (c *RPCClient) Call(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	maxAttempts := 1 + len(c.backupEndpoints)
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// === CRITICAL SECTION: only protect shared mutable state ===
		c.mu.Lock()
		endpoint := c.getEndpoint()
		reqID := c.requestID
		c.requestID++
		c.mu.Unlock()
		// === END CRITICAL SECTION ===

		// Build request (no shared state needed)
		req := RPCRequest{
			JSONRPC: "2.0",
			ID:      reqID,
			Method:  method,
			Params:  params,
		}

		body, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal RPC request: %w", err)
		}

		// Build HTTP request with context for cancellation
		httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create RPC request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		// HTTP POST (no lock held — concurrent calls proceed freely)
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("RPC POST to %s: %w", endpoint, err)
			c.rotateEndpointSafe(maxAttempts)
			continue
		}

		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read RPC response from %s: %w", endpoint, err)
			c.rotateEndpointSafe(maxAttempts)
			continue
		}

		// Check HTTP status (transport failure)
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("RPC HTTP %d from %s", resp.StatusCode, endpoint)
			c.rotateEndpointSafe(maxAttempts)
			continue
		}

		// Unmarshal response (transport failure if malformed)
		var rpcResp RPCResponse
		if err := json.Unmarshal(respBody, &rpcResp); err != nil {
			lastErr = fmt.Errorf("unmarshal RPC response from %s: %w", endpoint, err)
			c.rotateEndpointSafe(maxAttempts)
			continue
		}

		// Check for RPC error — this is an APPLICATION error, NOT a transport failure.
		// The endpoint is healthy; it just returned an error for this specific request.
		// Do NOT rotate, do NOT retry. Return immediately.
		if rpcResp.Error != nil {
			return nil, rpcResp.Error
		}

		// Success
		return rpcResp.Result, nil
	}

	return nil, fmt.Errorf("%w: %v", models.ErrRPCConnectionFailed, lastErr)
}

// getEndpoint returns the current endpoint based on currentIndex.
// Caller must hold c.mu.
func (c *RPCClient) getEndpoint() string {
	if c.currentIndex == 0 {
		return c.endpoint
	}
	return c.backupEndpoints[c.currentIndex-1]
}

// rotateEndpointSafe acquires the mutex and advances to the next endpoint.
func (c *RPCClient) rotateEndpointSafe(maxAttempts int) {
	c.mu.Lock()
	c.currentIndex = (c.currentIndex + 1) % maxAttempts
	c.mu.Unlock()
}
```

**Implementation — `internal/blockchain/rpc_test.go`:**

```go
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
```

**Verify:** `go test ./internal/blockchain/ -run 'TestCall|TestRequestID' -v`
**Commit:** `perf(rpc): narrow mutex to ID+endpoint, add context.Context, transport-only rotation`

---

### Task 1.2: Add BeginTx to Database
**File:** `internal/database/database.go`
**Test:** `internal/database/database_test.go`
**Depends:** none

Add a `BeginTx()` method for Fix 5 (batch PersistPortfolio). This is a thin wrapper.

**Implementation — add to `internal/database/database.go` (append after `IntegrityCheck` method, before `configurePragmas`):**

```go
// BeginTx starts a new database transaction.
// The caller must call tx.Commit() or tx.Rollback() when done.
func (d *Database) BeginTx() (*sql.Tx, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return tx, nil
}
```

**Implementation — add test to `internal/database/database_test.go`:**

The implementer should add this test function to the existing test file. Read the file first to understand the existing test patterns, then append:

```go
func TestBeginTxCommitAndRollback(t *testing.T) {
	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	defer db.Close()

	if err := db.CreateSchema(); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	// Test commit
	tx, err := db.BeginTx()
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Test rollback
	tx2, err := db.BeginTx()
	if err != nil {
		t.Fatalf("BeginTx (2) failed: %v", err)
	}
	if err := tx2.Rollback(); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
}
```

**Verify:** `go test ./internal/database/ -run TestBeginTx -v`
**Commit:** `feat(database): add BeginTx method for transaction support`

---

## Batch 2: Blockchain Wrappers (parallel — 2 implementers)

All tasks in this batch depend on Batch 1 completing (specifically Task 1.1).

### Task 2.1: Add context.Context to All solana.go Functions
**File:** `internal/blockchain/solana.go`
**Test:** `internal/blockchain/solana_test.go`
**Depends:** 1.1

Add `ctx context.Context` as the first parameter to all 11 functions, pass through to `client.Call(ctx, ...)`.

**Implementation — `internal/blockchain/solana.go`:**

Every function signature changes. The pattern is identical for all 11 functions:
- Add `ctx context.Context` as first param
- Change `client.Call("method", params)` → `client.Call(ctx, "method", params)`

```go
package blockchain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/dvrd/hound/internal/models"
)

// TokenAccount represents an SPL token account.
type TokenAccount struct {
	Pubkey   string
	Mint     string
	Owner    string
	Amount   uint64
	Decimals int
	UIAmount float64
}

// AccountBalance represents a token holder's balance.
type AccountBalance struct {
	Address  string
	Amount   uint64
	Decimals int
	UIAmount float64
}

// GetBalance returns the SOL balance in lamports for an address.
func GetBalance(ctx context.Context, client *RPCClient, address string) (uint64, error) {
	result, err := client.Call(ctx, "getBalance", []interface{}{address})
	if err != nil {
		return 0, fmt.Errorf("getBalance: %w", err)
	}

	var parsed struct {
		Value uint64 `json:"value"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return 0, fmt.Errorf("getBalance: parse result: %w", models.ErrRPCInvalidResponse)
	}

	return parsed.Value, nil
}

// GetTokenAccountsByOwner returns all SPL token accounts for an address.
func GetTokenAccountsByOwner(ctx context.Context, client *RPCClient, address string) ([]TokenAccount, error) {
	params := []interface{}{
		address,
		map[string]string{
			"programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
		},
		map[string]string{
			"encoding": "jsonParsed",
		},
	}

	result, err := client.Call(ctx, "getTokenAccountsByOwner", params)
	if err != nil {
		return nil, fmt.Errorf("getTokenAccountsByOwner: %w", err)
	}

	// Parse the deeply nested JSON response
	var parsed struct {
		Value []struct {
			Pubkey  string `json:"pubkey"`
			Account struct {
				Data struct {
					Parsed struct {
						Info struct {
							Mint        string `json:"mint"`
							Owner       string `json:"owner"`
							TokenAmount struct {
								Amount   string  `json:"amount"`
								Decimals int     `json:"decimals"`
								UIAmount float64 `json:"uiAmount"`
							} `json:"tokenAmount"`
						} `json:"info"`
					} `json:"parsed"`
				} `json:"data"`
			} `json:"account"`
		} `json:"value"`
	}

	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("getTokenAccountsByOwner: parse result: %w", models.ErrRPCInvalidResponse)
	}

	accounts := make([]TokenAccount, 0, len(parsed.Value))
	for _, v := range parsed.Value {
		amount, _ := strconv.ParseUint(v.Account.Data.Parsed.Info.TokenAmount.Amount, 10, 64)
		accounts = append(accounts, TokenAccount{
			Pubkey:   v.Pubkey,
			Mint:     v.Account.Data.Parsed.Info.Mint,
			Owner:    v.Account.Data.Parsed.Info.Owner,
			Amount:   amount,
			Decimals: v.Account.Data.Parsed.Info.TokenAmount.Decimals,
			UIAmount: v.Account.Data.Parsed.Info.TokenAmount.UIAmount,
		})
	}

	return accounts, nil
}

// GetAccountInfo returns raw account data (base64 decoded) for an address.
// Returns nil if the account does not exist (value is null).
func GetAccountInfo(ctx context.Context, client *RPCClient, address string) ([]byte, error) {
	params := []interface{}{
		address,
		map[string]string{
			"encoding":   "base64",
			"commitment": "confirmed",
		},
	}

	result, err := client.Call(ctx, "getAccountInfo", params)
	if err != nil {
		return nil, fmt.Errorf("getAccountInfo: %w", err)
	}

	// Check for null value (account doesn't exist)
	var parsed struct {
		Value *struct {
			Data []string `json:"data"`
		} `json:"value"`
	}

	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("getAccountInfo: parse result: %w", models.ErrRPCInvalidResponse)
	}

	if parsed.Value == nil {
		return nil, nil
	}

	if len(parsed.Value.Data) == 0 {
		return nil, fmt.Errorf("getAccountInfo: empty data array: %w", models.ErrRPCInvalidResponse)
	}

	decoded, err := base64.StdEncoding.DecodeString(parsed.Value.Data[0])
	if err != nil {
		return nil, fmt.Errorf("getAccountInfo: base64 decode: %w", models.ErrRPCInvalidResponse)
	}

	return decoded, nil
}

// GetTokenAccountBalance returns the balance of a specific token account.
func GetTokenAccountBalance(ctx context.Context, client *RPCClient, vaultAddr string) (amount uint64, decimals int, uiAmount float64, err error) {
	result, err := client.Call(ctx, "getTokenAccountBalance", []interface{}{vaultAddr})
	if err != nil {
		return 0, 0, 0, fmt.Errorf("getTokenAccountBalance: %w", err)
	}

	var parsed struct {
		Value struct {
			Amount   string  `json:"amount"`
			Decimals int     `json:"decimals"`
			UIAmount float64 `json:"uiAmount"`
		} `json:"value"`
	}

	if err := json.Unmarshal(result, &parsed); err != nil {
		return 0, 0, 0, fmt.Errorf("getTokenAccountBalance: parse result: %w", models.ErrRPCInvalidResponse)
	}

	amt, _ := strconv.ParseUint(parsed.Value.Amount, 10, 64)
	return amt, parsed.Value.Decimals, parsed.Value.UIAmount, nil
}

// GetTokenSupply returns the total supply of an SPL token.
func GetTokenSupply(ctx context.Context, client *RPCClient, mintAddr string) (totalSupply uint64, decimals int, err error) {
	result, err := client.Call(ctx, "getTokenSupply", []interface{}{mintAddr})
	if err != nil {
		return 0, 0, fmt.Errorf("getTokenSupply: %w", err)
	}

	var parsed struct {
		Value struct {
			Amount   string `json:"amount"`
			Decimals int    `json:"decimals"`
		} `json:"value"`
	}

	if err := json.Unmarshal(result, &parsed); err != nil {
		return 0, 0, fmt.Errorf("getTokenSupply: parse result: %w", models.ErrRPCInvalidResponse)
	}

	supply, _ := strconv.ParseUint(parsed.Value.Amount, 10, 64)
	return supply, parsed.Value.Decimals, nil
}

// GetTokenLargestAccounts returns the top holders of an SPL token.
func GetTokenLargestAccounts(ctx context.Context, client *RPCClient, mintAddr string) ([]AccountBalance, error) {
	result, err := client.Call(ctx, "getTokenLargestAccounts", []interface{}{mintAddr})
	if err != nil {
		return nil, fmt.Errorf("getTokenLargestAccounts: %w", err)
	}

	var parsed struct {
		Value []struct {
			Address  string  `json:"address"`
			Amount   string  `json:"amount"`
			Decimals int     `json:"decimals"`
			UIAmount float64 `json:"uiAmount"`
		} `json:"value"`
	}

	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("getTokenLargestAccounts: parse result: %w", models.ErrRPCInvalidResponse)
	}

	accounts := make([]AccountBalance, 0, len(parsed.Value))
	for _, v := range parsed.Value {
		amt, _ := strconv.ParseUint(v.Amount, 10, 64)
		accounts = append(accounts, AccountBalance{
			Address:  v.Address,
			Amount:   amt,
			Decimals: v.Decimals,
			UIAmount: v.UIAmount,
		})
	}

	return accounts, nil
}

// SignatureInfo represents a transaction signature from getSignaturesForAddress.
type SignatureInfo struct {
	Signature string      `json:"signature"`
	Slot      uint64      `json:"slot"`
	BlockTime *int64      `json:"blockTime"`
	Err       interface{} `json:"err"`
	Memo      *string     `json:"memo"`
}

// TransactionDetail represents parsed transaction data from getTransaction.
type TransactionDetail struct {
	Signature    string
	Slot         uint64
	BlockTime    *int64
	Fee          uint64
	Instructions []ParsedInstruction
	PreBalances  []uint64
	PostBalances []uint64
	Err          interface{}
}

// ParsedInstruction represents a parsed instruction from a transaction.
type ParsedInstruction struct {
	ProgramID string
	Program   string // "system", "spl-token", etc.
	Type      string // "transfer", "transferChecked", etc.
	Info      map[string]interface{}
}

// GetLatestBlockhash returns the latest blockhash and last valid block height.
func GetLatestBlockhash(ctx context.Context, client *RPCClient) (string, uint64, error) {
	result, err := client.Call(ctx, "getLatestBlockhash", []interface{}{
		map[string]string{"commitment": "finalized"},
	})
	if err != nil {
		return "", 0, fmt.Errorf("getLatestBlockhash: %w", err)
	}

	var parsed struct {
		Value struct {
			Blockhash            string `json:"blockhash"`
			LastValidBlockHeight uint64 `json:"lastValidBlockHeight"`
		} `json:"value"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return "", 0, fmt.Errorf("getLatestBlockhash: parse result: %w", models.ErrRPCInvalidResponse)
	}

	return parsed.Value.Blockhash, parsed.Value.LastValidBlockHeight, nil
}

// SendTransaction submits a signed transaction to the network.
func SendTransaction(ctx context.Context, client *RPCClient, base64Tx string) (string, error) {
	result, err := client.Call(ctx, "sendTransaction", []interface{}{
		base64Tx,
		map[string]interface{}{
			"encoding":            "base64",
			"skipPreflight":       false,
			"preflightCommitment": "confirmed",
		},
	})
	if err != nil {
		return "", fmt.Errorf("sendTransaction: %w", err)
	}

	var signature string
	if err := json.Unmarshal(result, &signature); err != nil {
		return "", fmt.Errorf("sendTransaction: parse result: %w", models.ErrRPCInvalidResponse)
	}

	return signature, nil
}

// GetSignaturesForAddress returns transaction signatures for an address.
func GetSignaturesForAddress(ctx context.Context, client *RPCClient, address string, limit int, before string) ([]SignatureInfo, error) {
	opts := map[string]interface{}{"limit": limit}
	if before != "" {
		opts["before"] = before
	}

	result, err := client.Call(ctx, "getSignaturesForAddress", []interface{}{address, opts})
	if err != nil {
		return nil, fmt.Errorf("getSignaturesForAddress: %w", err)
	}

	var sigs []SignatureInfo
	if err := json.Unmarshal(result, &sigs); err != nil {
		return nil, fmt.Errorf("getSignaturesForAddress: parse result: %w", models.ErrRPCInvalidResponse)
	}

	return sigs, nil
}

// GetTransaction returns parsed transaction details for a signature.
func GetTransaction(ctx context.Context, client *RPCClient, signature string) (*TransactionDetail, error) {
	result, err := client.Call(ctx, "getTransaction", []interface{}{
		signature,
		map[string]interface{}{
			"encoding":                       "jsonParsed",
			"maxSupportedTransactionVersion": 0,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getTransaction: %w", err)
	}

	// Check for null result (transaction not found)
	if string(result) == "null" {
		return nil, nil
	}

	var parsed struct {
		Slot      uint64 `json:"slot"`
		BlockTime *int64 `json:"blockTime"`
		Meta      struct {
			Fee          uint64      `json:"fee"`
			PreBalances  []uint64    `json:"preBalances"`
			PostBalances []uint64    `json:"postBalances"`
			Err          interface{} `json:"err"`
		} `json:"meta"`
		Transaction struct {
			Message struct {
				Instructions []struct {
					ProgramID string `json:"programId"`
					Program   string `json:"program"`
					Parsed    *struct {
						Type string                 `json:"type"`
						Info map[string]interface{} `json:"info"`
					} `json:"parsed,omitempty"`
				} `json:"instructions"`
			} `json:"message"`
		} `json:"transaction"`
	}

	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("getTransaction: parse result: %w", models.ErrRPCInvalidResponse)
	}

	detail := &TransactionDetail{
		Signature:    signature,
		Slot:         parsed.Slot,
		BlockTime:    parsed.BlockTime,
		Fee:          parsed.Meta.Fee,
		PreBalances:  parsed.Meta.PreBalances,
		PostBalances: parsed.Meta.PostBalances,
		Err:          parsed.Meta.Err,
	}

	for _, ix := range parsed.Transaction.Message.Instructions {
		pi := ParsedInstruction{
			ProgramID: ix.ProgramID,
			Program:   ix.Program,
		}
		if ix.Parsed != nil {
			pi.Type = ix.Parsed.Type
			pi.Info = ix.Parsed.Info
		}
		detail.Instructions = append(detail.Instructions, pi)
	}

	return detail, nil
}

// GetMinimumBalanceForRentExemption returns the minimum balance for rent exemption.
func GetMinimumBalanceForRentExemption(ctx context.Context, client *RPCClient, dataSize uint64) (uint64, error) {
	result, err := client.Call(ctx, "getMinimumBalanceForRentExemption", []interface{}{dataSize})
	if err != nil {
		return 0, fmt.Errorf("getMinimumBalanceForRentExemption: %w", err)
	}

	var lamports uint64
	if err := json.Unmarshal(result, &lamports); err != nil {
		return 0, fmt.Errorf("getMinimumBalanceForRentExemption: parse result: %w", models.ErrRPCInvalidResponse)
	}

	return lamports, nil
}
```

**Implementation — `internal/blockchain/solana_test.go`:**

Every test call needs `context.Background()` added. The `mockRPCServer` helper stays the same. The pattern for every test:
- `blockchain.GetBalance(client, addr)` → `blockchain.GetBalance(context.Background(), client, addr)`

```go
package blockchain_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dvrd/hound/internal/blockchain"
)

func mockRPCServer(t *testing.T, responses map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Logf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		respBody, ok := responses[req.Method]
		if !ok {
			t.Logf("unexpected method: %s", req.Method)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		resp := blockchain.RPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(respBody),
		}
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestGetBalance(t *testing.T) {
	server := mockRPCServer(t, map[string]string{
		"getBalance": `{"context":{"slot":123},"value":5000000000}`,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	balance, err := blockchain.GetBalance(context.Background(), client, "11111111111111111111111111111111")
	if err != nil {
		t.Fatalf("GetBalance failed: %v", err)
	}
	if balance != 5000000000 {
		t.Errorf("expected balance 5000000000, got %d", balance)
	}
}

func TestGetTokenAccountsByOwner(t *testing.T) {
	mockResponse := `{
		"context": {"slot": 123},
		"value": [
			{
				"pubkey": "tokenAcct1",
				"account": {
					"data": {
						"parsed": {
							"info": {
								"mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
								"owner": "ownerAddr",
								"tokenAmount": {
									"amount": "1000000",
									"decimals": 6,
									"uiAmount": 1.0
								}
							}
						},
						"program": "spl-token",
						"space": 165
					},
					"executable": false,
					"lamports": 2039280,
					"owner": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
				}
			},
			{
				"pubkey": "tokenAcct2",
				"account": {
					"data": {
						"parsed": {
							"info": {
								"mint": "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
								"owner": "ownerAddr",
								"tokenAmount": {
									"amount": "50000000000",
									"decimals": 5,
									"uiAmount": 500000.0
								}
							}
						},
						"program": "spl-token",
						"space": 165
					},
					"executable": false,
					"lamports": 2039280,
					"owner": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
				}
			}
		]
	}`

	server := mockRPCServer(t, map[string]string{
		"getTokenAccountsByOwner": mockResponse,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	accounts, err := blockchain.GetTokenAccountsByOwner(context.Background(), client, "ownerAddr")
	if err != nil {
		t.Fatalf("GetTokenAccountsByOwner failed: %v", err)
	}

	if len(accounts) != 2 {
		t.Fatalf("expected 2 token accounts, got %d", len(accounts))
	}

	if accounts[0].Pubkey != "tokenAcct1" {
		t.Errorf("account[0].Pubkey = %q, want %q", accounts[0].Pubkey, "tokenAcct1")
	}
	if accounts[0].Mint != "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" {
		t.Errorf("account[0].Mint = %q, want USDC mint", accounts[0].Mint)
	}
	if accounts[0].Amount != 1000000 {
		t.Errorf("account[0].Amount = %d, want 1000000", accounts[0].Amount)
	}
	if accounts[0].Decimals != 6 {
		t.Errorf("account[0].Decimals = %d, want 6", accounts[0].Decimals)
	}
	if accounts[1].Pubkey != "tokenAcct2" {
		t.Errorf("account[1].Pubkey = %q, want %q", accounts[1].Pubkey, "tokenAcct2")
	}
	if accounts[1].Amount != 50000000000 {
		t.Errorf("account[1].Amount = %d, want 50000000000", accounts[1].Amount)
	}
}

func TestGetAccountInfo(t *testing.T) {
	server := mockRPCServer(t, map[string]string{
		"getAccountInfo": `{"context":{"slot":123},"value":{"data":["SGVsbG8gV29ybGQ=","base64"],"executable":false,"lamports":1000000,"owner":"11111111111111111111111111111111"}}`,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	data, err := blockchain.GetAccountInfo(context.Background(), client, "someAddress")
	if err != nil {
		t.Fatalf("GetAccountInfo failed: %v", err)
	}

	expected := "Hello World"
	if string(data) != expected {
		t.Errorf("GetAccountInfo data = %q, want %q", string(data), expected)
	}
}

func TestGetAccountInfoNull(t *testing.T) {
	server := mockRPCServer(t, map[string]string{
		"getAccountInfo": `{"context":{"slot":123},"value":null}`,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	data, err := blockchain.GetAccountInfo(context.Background(), client, "nonExistentAddress")
	if err != nil {
		t.Fatalf("GetAccountInfo for null should not error: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data for null account, got %v", data)
	}
}

func TestGetTokenAccountBalance(t *testing.T) {
	server := mockRPCServer(t, map[string]string{
		"getTokenAccountBalance": `{"context":{"slot":123},"value":{"amount":"1000000","decimals":6,"uiAmount":1.0,"uiAmountString":"1"}}`,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	amount, decimals, uiAmount, err := blockchain.GetTokenAccountBalance(context.Background(), client, "vaultAddr")
	if err != nil {
		t.Fatalf("GetTokenAccountBalance failed: %v", err)
	}
	if amount != 1000000 {
		t.Errorf("amount = %d, want 1000000", amount)
	}
	if decimals != 6 {
		t.Errorf("decimals = %d, want 6", decimals)
	}
	if uiAmount != 1.0 {
		t.Errorf("uiAmount = %f, want 1.0", uiAmount)
	}
}

func TestGetTokenSupply(t *testing.T) {
	server := mockRPCServer(t, map[string]string{
		"getTokenSupply": `{"context":{"slot":123},"value":{"amount":"10000000000","decimals":6,"uiAmount":10000.0,"uiAmountString":"10000"}}`,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	supply, decimals, err := blockchain.GetTokenSupply(context.Background(), client, "mintAddr")
	if err != nil {
		t.Fatalf("GetTokenSupply failed: %v", err)
	}
	if supply != 10000000000 {
		t.Errorf("supply = %d, want 10000000000", supply)
	}
	if decimals != 6 {
		t.Errorf("decimals = %d, want 6", decimals)
	}
}

func TestGetTokenLargestAccounts(t *testing.T) {
	server := mockRPCServer(t, map[string]string{
		"getTokenLargestAccounts": `{"context":{"slot":123},"value":[{"address":"holder1","amount":"5000000000","decimals":6,"uiAmount":5000.0},{"address":"holder2","amount":"3000000000","decimals":6,"uiAmount":3000.0}]}`,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	accounts, err := blockchain.GetTokenLargestAccounts(context.Background(), client, "mintAddr")
	if err != nil {
		t.Fatalf("GetTokenLargestAccounts failed: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}
	if accounts[0].Address != "holder1" {
		t.Errorf("accounts[0].Address = %q, want %q", accounts[0].Address, "holder1")
	}
	if accounts[0].Amount != 5000000000 {
		t.Errorf("accounts[0].Amount = %d, want 5000000000", accounts[0].Amount)
	}
	if accounts[1].Address != "holder2" {
		t.Errorf("accounts[1].Address = %q, want %q", accounts[1].Address, "holder2")
	}
}

func TestGetLatestBlockhash(t *testing.T) {
	server := mockRPCServer(t, map[string]string{
		"getLatestBlockhash": `{"context":{"slot":123},"value":{"blockhash":"GHtXQBsoZHVnNFa9YevAzFr17DJjgHXk3ycTKD5xD3Zi","lastValidBlockHeight":12345}}`,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	blockhash, height, err := blockchain.GetLatestBlockhash(context.Background(), client)
	if err != nil {
		t.Fatalf("GetLatestBlockhash failed: %v", err)
	}
	if blockhash != "GHtXQBsoZHVnNFa9YevAzFr17DJjgHXk3ycTKD5xD3Zi" {
		t.Errorf("blockhash = %q, want GHtXQBsoZHVnNFa9YevAzFr17DJjgHXk3ycTKD5xD3Zi", blockhash)
	}
	if height != 12345 {
		t.Errorf("height = %d, want 12345", height)
	}
}

func TestSendTransaction(t *testing.T) {
	server := mockRPCServer(t, map[string]string{
		"sendTransaction": `"5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW"`,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	sig, err := blockchain.SendTransaction(context.Background(), client, "base64encodedtx")
	if err != nil {
		t.Fatalf("SendTransaction failed: %v", err)
	}
	if sig != "5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW" {
		t.Errorf("signature = %q", sig)
	}
}

func TestGetSignaturesForAddress(t *testing.T) {
	server := mockRPCServer(t, map[string]string{
		"getSignaturesForAddress": `[{"signature":"sig1","slot":100,"blockTime":1700000000,"err":null,"memo":null},{"signature":"sig2","slot":101,"blockTime":1700000001,"err":null,"memo":null}]`,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	sigs, err := blockchain.GetSignaturesForAddress(context.Background(), client, "someAddr", 10, "")
	if err != nil {
		t.Fatalf("GetSignaturesForAddress failed: %v", err)
	}
	if len(sigs) != 2 {
		t.Fatalf("got %d signatures, want 2", len(sigs))
	}
	if sigs[0].Signature != "sig1" {
		t.Errorf("sigs[0].Signature = %q, want sig1", sigs[0].Signature)
	}
	if sigs[0].Slot != 100 {
		t.Errorf("sigs[0].Slot = %d, want 100", sigs[0].Slot)
	}
}

func TestGetSignaturesForAddress_WithBefore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Params) < 2 {
			t.Error("expected 2 params")
		}
		opts, ok := req.Params[1].(map[string]interface{})
		if !ok {
			t.Error("second param should be a map")
		}
		if _, hasBefore := opts["before"]; !hasBefore {
			t.Error("opts should contain 'before'")
		}

		resp := blockchain.RPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`[]`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	_, err := blockchain.GetSignaturesForAddress(context.Background(), client, "addr", 5, "beforeSig")
	if err != nil {
		t.Fatalf("GetSignaturesForAddress with before failed: %v", err)
	}
}

func TestGetTransaction(t *testing.T) {
	mockResp := `{
		"slot": 200,
		"blockTime": 1700000000,
		"meta": {
			"fee": 5000,
			"preBalances": [10000000000, 0],
			"postBalances": [8999995000, 1000000000],
			"err": null
		},
		"transaction": {
			"message": {
				"instructions": [
					{
						"programId": "11111111111111111111111111111111",
						"program": "system",
						"parsed": {
							"type": "transfer",
							"info": {
								"source": "sender",
								"destination": "recipient",
								"lamports": 1000000000
							}
						}
					}
				]
			}
		}
	}`

	server := mockRPCServer(t, map[string]string{
		"getTransaction": mockResp,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	detail, err := blockchain.GetTransaction(context.Background(), client, "someSig")
	if err != nil {
		t.Fatalf("GetTransaction failed: %v", err)
	}
	if detail == nil {
		t.Fatal("GetTransaction returned nil")
	}
	if detail.Slot != 200 {
		t.Errorf("Slot = %d, want 200", detail.Slot)
	}
	if detail.Fee != 5000 {
		t.Errorf("Fee = %d, want 5000", detail.Fee)
	}
	if len(detail.Instructions) != 1 {
		t.Fatalf("Instructions count = %d, want 1", len(detail.Instructions))
	}
	if detail.Instructions[0].Program != "system" {
		t.Errorf("Program = %q, want system", detail.Instructions[0].Program)
	}
	if detail.Instructions[0].Type != "transfer" {
		t.Errorf("Type = %q, want transfer", detail.Instructions[0].Type)
	}
}

func TestGetTransaction_NotFound(t *testing.T) {
	server := mockRPCServer(t, map[string]string{
		"getTransaction": `null`,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	detail, err := blockchain.GetTransaction(context.Background(), client, "nonexistent")
	if err != nil {
		t.Fatalf("GetTransaction failed: %v", err)
	}
	if detail != nil {
		t.Error("expected nil for not-found transaction")
	}
}

func TestGetMinimumBalanceForRentExemption(t *testing.T) {
	server := mockRPCServer(t, map[string]string{
		"getMinimumBalanceForRentExemption": `2039280`,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	lamports, err := blockchain.GetMinimumBalanceForRentExemption(context.Background(), client, 165)
	if err != nil {
		t.Fatalf("GetMinimumBalanceForRentExemption failed: %v", err)
	}
	if lamports != 2039280 {
		t.Errorf("lamports = %d, want 2039280", lamports)
	}
}
```

**Verify:** `go test ./internal/blockchain/ -run 'TestGet|TestSend' -v`
**Commit:** `perf(blockchain): add context.Context to all solana.go RPC wrapper functions`

---

### Task 2.2: Add context.Context to Oracle + UpdateBalanceTx
**File:** `internal/blockchain/oracle.go` AND `internal/database/balances.go`
**Test:** `internal/blockchain/oracle_test.go` AND `internal/database/balances_test.go`
**Depends:** 1.1

Two small changes bundled because they're both foundation pieces with no cross-dependency:

**Part A: oracle.go** — Add `ctx context.Context` to `GetSOLPriceCached()` and internal fetch functions. Use `http.NewRequestWithContext`.

**Part B: balances.go** — Add `UpdateBalanceTx()` that accepts `*sql.Tx` for Fix 5.

**Implementation — `internal/blockchain/oracle.go`:**

The key change: `GetSOLPriceCached(ctx context.Context)` and internal functions use `http.NewRequestWithContext`.

```go
package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/dvrd/hound/internal/models"
)

// SOLMint is the mint address for native SOL (wrapped).
const SOLMint = "So11111111111111111111111111111111111111112"

const (
	solPriceTTL    = 30 * time.Second
	solPriceMinUSD = 1.0
	solPriceMaxUSD = 10000.0

	jupiterPriceURL   = "https://lite-api.jup.ag/price/v3?ids=" + SOLMint
	coingeckoPriceURL = "https://api.coingecko.com/api/v3/simple/price?ids=solana&vs_currencies=usd"
)

// solPriceCache holds the cached SOL/USD price.
var solPriceCache = struct {
	mu        sync.Mutex
	price     float64
	fetchedAt time.Time
}{}

// httpClientForOracle is the HTTP client used by the oracle.
// Exported via setter for testing.
var oracleHTTPClient = &http.Client{Timeout: 10 * time.Second}

// SetOracleHTTPClient replaces the HTTP client used by the oracle (for testing).
func SetOracleHTTPClient(c *http.Client) {
	oracleHTTPClient = c
}

// GetSOLPriceCached returns the cached SOL/USD price, refreshing if stale (30s TTL).
// Fallback chain: cache → Jupiter Price API → CoinGecko API.
// Validates price is in $1-$10000 range.
func GetSOLPriceCached(ctx context.Context) (float64, error) {
	solPriceCache.mu.Lock()
	defer solPriceCache.mu.Unlock()

	// Return cached value if fresh
	if solPriceCache.price > 0 && time.Since(solPriceCache.fetchedAt) < solPriceTTL {
		return solPriceCache.price, nil
	}

	// Try Jupiter first
	price, err := fetchSOLPriceJupiter(ctx)
	if err == nil {
		solPriceCache.price = price
		solPriceCache.fetchedAt = time.Now()
		return price, nil
	}

	// Fallback to CoinGecko
	price, err = fetchSOLPriceCoinGecko(ctx)
	if err == nil {
		solPriceCache.price = price
		solPriceCache.fetchedAt = time.Now()
		return price, nil
	}

	return 0, fmt.Errorf("all SOL price sources failed: %w", models.ErrOracleConnectionFailed)
}

// ResetSOLPriceCache clears the cached SOL price (for testing).
func ResetSOLPriceCache() {
	solPriceCache.mu.Lock()
	defer solPriceCache.mu.Unlock()
	solPriceCache.price = 0
	solPriceCache.fetchedAt = time.Time{}
}

// fetchSOLPriceJupiter fetches SOL/USD from Jupiter Price API v3.
func fetchSOLPriceJupiter(ctx context.Context) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", jupiterPriceURL, nil)
	if err != nil {
		return 0, fmt.Errorf("jupiter price create request: %w", err)
	}

	resp, err := oracleHTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("jupiter price request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("jupiter price HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("jupiter price read body: %w", err)
	}

	var parsed struct {
		Data map[string]struct {
			Price string `json:"price"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("jupiter price parse: %w", err)
	}

	entry, ok := parsed.Data[SOLMint]
	if !ok {
		return 0, fmt.Errorf("jupiter price: SOL mint not in response")
	}

	price, err := strconv.ParseFloat(entry.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("jupiter price parse float: %w", err)
	}

	if err := validateSOLPrice(price); err != nil {
		return 0, err
	}

	return price, nil
}

// fetchSOLPriceCoinGecko fetches SOL/USD from CoinGecko API.
func fetchSOLPriceCoinGecko(ctx context.Context) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", coingeckoPriceURL, nil)
	if err != nil {
		return 0, fmt.Errorf("coingecko price create request: %w", err)
	}

	resp, err := oracleHTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("coingecko price request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("coingecko price HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("coingecko price read body: %w", err)
	}

	var parsed struct {
		Solana struct {
			USD float64 `json:"usd"`
		} `json:"solana"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("coingecko price parse: %w", err)
	}

	price := parsed.Solana.USD
	if err := validateSOLPrice(price); err != nil {
		return 0, err
	}

	return price, nil
}

// validateSOLPrice checks that the price is within a reasonable range.
func validateSOLPrice(price float64) error {
	if price < solPriceMinUSD || price > solPriceMaxUSD {
		return fmt.Errorf("SOL price $%.2f outside valid range [$%.0f-$%.0f]: %w",
			price, solPriceMinUSD, solPriceMaxUSD, models.ErrOraclePriceInvalid)
	}
	return nil
}
```

**Implementation — `internal/blockchain/oracle_test.go`:**

Every call to `blockchain.GetSOLPriceCached()` becomes `blockchain.GetSOLPriceCached(context.Background())`. The rest of the test file stays identical. Add `"context"` to imports.

**Implementation — add to `internal/database/balances.go`:**

```go
import (
	"database/sql"
	// ... existing imports
)

// UpdateBalanceTx inserts or replaces a token balance within an existing transaction.
// Use this with BeginTx() for atomic batch writes.
func (d *Database) UpdateBalanceTx(tx *sql.Tx, walletAddr, mint, symbol string, amount, usdPrice, usdValue float64) error {
	now := time.Now().Unix()

	_, err := tx.Exec(
		`INSERT OR REPLACE INTO balances (wallet_address, mint, symbol, amount, usd_price, usd_value, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		walletAddr, mint, symbol, amount, usdPrice, usdValue, now,
	)
	if err != nil {
		return fmt.Errorf("updating balance (tx) for wallet %q mint %q: %w", walletAddr, mint, err)
	}
	return nil
}
```

**Implementation — add test to `internal/database/balances_test.go`:**

```go
func TestUpdateBalanceTx(t *testing.T) {
	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	// Insert a wallet first (foreign key)
	w := models.Wallet{Address: "txTestAddr", Label: "tx-test", IsPrimary: true}
	if err := db.InsertWallet(w); err != nil {
		t.Fatalf("insert wallet: %v", err)
	}

	// Test commit path
	tx, err := db.BeginTx()
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if err := db.UpdateBalanceTx(tx, "txTestAddr", "SOLmint", "SOL", 1.5, 150.0, 225.0); err != nil {
		tx.Rollback()
		t.Fatalf("UpdateBalanceTx: %v", err)
	}
	if err := db.UpdateBalanceTx(tx, "txTestAddr", "USDCmint", "USDC", 100.0, 1.0, 100.0); err != nil {
		tx.Rollback()
		t.Fatalf("UpdateBalanceTx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Verify both balances persisted
	balances, err := db.GetBalancesForWallet("txTestAddr")
	if err != nil {
		t.Fatalf("GetBalancesForWallet: %v", err)
	}
	if len(balances) != 2 {
		t.Fatalf("expected 2 balances, got %d", len(balances))
	}

	// Test rollback path
	tx2, err := db.BeginTx()
	if err != nil {
		t.Fatalf("BeginTx (2): %v", err)
	}
	if err := db.UpdateBalanceTx(tx2, "txTestAddr", "BONKmint", "BONK", 999.0, 0.001, 0.999); err != nil {
		tx2.Rollback()
		t.Fatalf("UpdateBalanceTx (rollback): %v", err)
	}
	tx2.Rollback()

	// Verify BONK was NOT persisted
	balances2, err := db.GetBalancesForWallet("txTestAddr")
	if err != nil {
		t.Fatalf("GetBalancesForWallet (2): %v", err)
	}
	if len(balances2) != 2 {
		t.Fatalf("expected 2 balances after rollback, got %d", len(balances2))
	}
}
```

**Verify:** `go test ./internal/blockchain/ -run TestGetSOLPrice -v && go test ./internal/database/ -run TestUpdateBalanceTx -v`
**Commit:** `perf(oracle,database): add context to oracle, add UpdateBalanceTx for batch writes`

---

## Batch 3: Caller Updates (parallel — 5 implementers)

All tasks in this batch depend on Batch 2 completing. Each task updates one file to pass `context.Background()` to the new context-aware blockchain functions.

### Task 3.1: Update wallet/balance.go
**File:** `internal/wallet/balance.go`
**Test:** `internal/wallet/balance_test.go`
**Depends:** 2.1, 2.2

Add `"context"` import. Change:
- `blockchain.GetBalance(f.rpcClient, address)` → `blockchain.GetBalance(context.Background(), f.rpcClient, address)`
- `blockchain.GetSOLPriceCached()` → `blockchain.GetSOLPriceCached(context.Background())`
- `blockchain.GetTokenAccountsByOwner(f.rpcClient, address)` → `blockchain.GetTokenAccountsByOwner(context.Background(), f.rpcClient, address)`

The implementer should read the existing file, add `"context"` to imports, and make these 3 substitutions. No other logic changes.

**Verify:** `go test ./internal/wallet/ -v`
**Commit:** `refactor(wallet): pass context.Background() to blockchain calls in balance.go`

---

### Task 3.2: Update services/transfer.go
**File:** `internal/services/transfer.go`
**Test:** `internal/services/transfer_test.go`
**Depends:** 2.1

Add `"context"` import. Change all blockchain calls to include `context.Background()`:
- `blockchain.GetBalance(rpcClient, fromAddr)` → `blockchain.GetBalance(context.Background(), rpcClient, fromAddr)` (2 occurrences)
- `blockchain.GetLatestBlockhash(rpcClient)` → `blockchain.GetLatestBlockhash(context.Background(), rpcClient)` (2 occurrences)
- `blockchain.SendTransaction(rpcClient, ...)` → `blockchain.SendTransaction(context.Background(), rpcClient, ...)` (2 occurrences)
- `blockchain.GetTokenAccountsByOwner(rpcClient, fromAddr)` → `blockchain.GetTokenAccountsByOwner(context.Background(), rpcClient, fromAddr)` (1 occurrence)
- `blockchain.GetAccountInfo(rpcClient, ...)` → `blockchain.GetAccountInfo(context.Background(), rpcClient, ...)` (1 occurrence)

**Verify:** `go test ./internal/services/ -run TestSend -v`
**Commit:** `refactor(transfer): pass context.Background() to blockchain calls`

---

### Task 3.3: Update services/activity.go
**File:** `internal/services/activity.go`
**Test:** `internal/services/activity_test.go`
**Depends:** 2.1

Add `"context"` import. Change:
- `blockchain.GetSignaturesForAddress(rpcClient, address, limit, before)` → `blockchain.GetSignaturesForAddress(context.Background(), rpcClient, address, limit, before)`
- `blockchain.GetTransaction(rpcClient, sig.Signature)` → `blockchain.GetTransaction(context.Background(), rpcClient, sig.Signature)`

**Note:** This is just the context plumbing. The parallelization (Fix 4) comes in Task 4.1.

**Verify:** `go test ./internal/services/ -run TestActivity -v`
**Commit:** `refactor(activity): pass context.Background() to blockchain calls`

---

### Task 3.4: Update services/token_info.go
**File:** `internal/services/token_info.go`
**Test:** `internal/services/token_info_test.go`
**Depends:** 2.1

Add `"context"` import. Change:
- `blockchain.GetTokenSupply(s.rpcClient, mint)` → `blockchain.GetTokenSupply(context.Background(), s.rpcClient, mint)`
- `blockchain.GetTokenLargestAccounts(s.rpcClient, mint)` → `blockchain.GetTokenLargestAccounts(context.Background(), s.rpcClient, mint)`

**Verify:** `go test ./internal/services/ -run TestTokenInfo -v`
**Commit:** `refactor(token_info): pass context.Background() to blockchain calls`

---

### Task 3.5: Update dex/router.go
**File:** `internal/dex/router.go`
**Test:** `internal/dex/router_test.go`
**Depends:** 2.2

The `Router` stores `getSOLPrice func() (float64, error)` as a function pointer. This needs to change to `func(context.Context) (float64, error)` to match the new `GetSOLPriceCached` signature.

Changes:
1. Add `"context"` import
2. Change field type: `getSOLPrice func() (float64, error)` → `getSOLPrice func(context.Context) (float64, error)`
3. Update `NewRouter`: `getSOLPrice: blockchain.GetSOLPriceCached` (already matches new signature)
4. Update `NewRouterWithSOLPrice` param type
5. Update `QuoteToUSD`: `r.getSOLPrice()` → `r.getSOLPrice(context.Background())`

```go
// Router routes price queries through multiple DEX sources.
type Router struct {
	rpcClient     *blockchain.RPCClient
	jupiterClient *JupiterClient
	getSOLPrice   func(context.Context) (float64, error)
}

// NewRouter creates a new price router.
func NewRouter(rpcClient *blockchain.RPCClient, jupiterClient *JupiterClient) *Router {
	return &Router{
		rpcClient:     rpcClient,
		jupiterClient: jupiterClient,
		getSOLPrice:   blockchain.GetSOLPriceCached,
	}
}

// NewRouterWithSOLPrice creates a router with a custom SOL price function (for testing).
func NewRouterWithSOLPrice(rpcClient *blockchain.RPCClient, jupiterClient *JupiterClient, getSOLPrice func(context.Context) (float64, error)) *Router {
	return &Router{
		rpcClient:     rpcClient,
		jupiterClient: jupiterClient,
		getSOLPrice:   getSOLPrice,
	}
}
```

And in `QuoteToUSD`:
```go
	case "sol", "wsol":
		solPrice, err := r.getSOLPrice(context.Background())
```

The test file `router_test.go` needs its mock function signature updated from `func() (float64, error)` to `func(context.Context) (float64, error)`. The implementer should read the test file, find `NewRouterWithSOLPrice` calls, and update the lambda signatures.

**Verify:** `go test ./internal/dex/ -run TestRouter -v`
**Commit:** `refactor(dex): update router SOL price function to accept context`

---

## Batch 4: Feature Work (parallel — 2 implementers)

These tasks implement the actual performance features. They depend on Batch 3 completing.

### Task 4.1: Parallelize GetTransaction in activity.go (Fix 4)
**File:** `internal/services/activity.go`
**Test:** `internal/services/activity_test.go`
**Depends:** 3.3

Replace the sequential `for` loop over signatures with a fan-out pattern using `sync.WaitGroup` + semaphore channel (capacity 5).

**Implementation — replace the `GetActivity` method in `internal/services/activity.go`:**

```go
package services

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/database"
)

// ... (ActivityItem, ActivityService, NewActivityService stay the same)

// maxConcurrentTxFetches limits parallel GetTransaction calls to avoid rate limiting.
const maxConcurrentTxFetches = 5

// GetActivity fetches on-chain activity for an address and merges with local swap history.
func (s *ActivityService) GetActivity(rpcClient *blockchain.RPCClient, address string, limit int, before string) ([]ActivityItem, error) {
	if rpcClient == nil {
		return nil, fmt.Errorf("get activity: RPC client is nil")
	}

	ctx := context.Background()

	// 1. Fetch signatures
	sigs, err := blockchain.GetSignaturesForAddress(ctx, rpcClient, address, limit, before)
	if err != nil {
		return nil, fmt.Errorf("get activity: %w", err)
	}

	if len(sigs) == 0 {
		return nil, nil
	}

	// 2. Fan-out: fetch transaction details concurrently with bounded parallelism
	type indexedResult struct {
		index int
		item  ActivityItem
		ok    bool
	}

	results := make([]indexedResult, len(sigs))
	sem := make(chan struct{}, maxConcurrentTxFetches)
	var wg sync.WaitGroup

	for i, sig := range sigs {
		wg.Add(1)
		go func(idx int, signature string) {
			defer wg.Done()

			sem <- struct{}{}        // acquire semaphore
			defer func() { <-sem }() // release semaphore

			detail, err := blockchain.GetTransaction(ctx, rpcClient, signature)
			if err != nil || detail == nil {
				// Skip transactions we can't fetch (same as before, but concurrent)
				return
			}

			item := classifyTransaction(detail, address)
			results[idx] = indexedResult{index: idx, item: item, ok: true}
		}(i, sig.Signature)
	}

	wg.Wait()

	// 3. Collect successful results (preserving order)
	items := make([]ActivityItem, 0, len(sigs))
	for _, r := range results {
		if r.ok {
			items = append(items, r.item)
		}
	}

	// 4. Merge with local swap history
	if s.db != nil {
		swapEntries, err := s.db.GetSwapHistory(address, limit)
		if err == nil {
			sigToSwap := make(map[string]bool, len(swapEntries))
			for _, entry := range swapEntries {
				sigToSwap[entry.Signature] = true
			}
			for i := range items {
				if sigToSwap[items[i].Signature] {
					items[i].Type = "swap"
				}
			}
		}
	}

	// 5. Sort by timestamp descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp > items[j].Timestamp
	})

	return items, nil
}

// ... (classifyTransaction, classifySOLTransfer, classifySPLTransfer,
//      classifyDirectionFromBalances, TruncateAddress, FormatLamports stay the same)
```

**Test additions for `internal/services/activity_test.go`:**

The implementer should read the existing test file and add a test that verifies:
1. Multiple transactions are fetched (mock server with delays)
2. Results maintain correct order
3. Failed individual fetches are skipped gracefully

```go
func TestGetActivityParallel(t *testing.T) {
	// Create a mock RPC server that handles both getSignaturesForAddress and getTransaction
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string        `json:"method"`
			ID     int           `json:"id"`
			Params []interface{} `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		var result json.RawMessage
		switch req.Method {
		case "getSignaturesForAddress":
			result = json.RawMessage(`[
				{"signature":"sig1","slot":100,"blockTime":1700000003},
				{"signature":"sig2","slot":101,"blockTime":1700000002},
				{"signature":"sig3","slot":102,"blockTime":1700000001}
			]`)
		case "getTransaction":
			// Simulate some latency to test concurrency
			time.Sleep(5 * time.Millisecond)
			sig := req.Params[0].(string)
			if sig == "sig2" {
				// Simulate a not-found transaction
				result = json.RawMessage(`null`)
			} else {
				bt := int64(1700000003)
				if sig == "sig3" {
					bt = 1700000001
				}
				result = json.RawMessage(fmt.Sprintf(`{
					"slot": 100,
					"blockTime": %d,
					"meta": {"fee": 5000, "preBalances": [100], "postBalances": [95], "err": null},
					"transaction": {"message": {"instructions": []}}
				}`, bt))
			}
		}

		resp := struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int             `json:"id"`
			Result  json.RawMessage `json:"result"`
		}{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	svc := services.NewActivityService(nil)

	items, err := svc.GetActivity(client, "testAddr", 10, "")
	if err != nil {
		t.Fatalf("GetActivity failed: %v", err)
	}

	// sig2 returns null, so we should get 2 items
	if len(items) != 2 {
		t.Fatalf("expected 2 items (sig2 is null), got %d", len(items))
	}

	// Should be sorted by timestamp descending
	if items[0].Timestamp < items[1].Timestamp {
		t.Errorf("items not sorted by timestamp descending: %d, %d", items[0].Timestamp, items[1].Timestamp)
	}
}
```

**Verify:** `go test ./internal/services/ -run TestActivity -v`
**Commit:** `perf(activity): parallelize GetTransaction with semaphore (max 5 concurrent)`

---

### Task 4.2: Batch PersistPortfolio in Single Transaction (Fix 5)
**File:** `internal/wallet/manager.go`
**Test:** `internal/wallet/manager_test.go`
**Depends:** 1.2, 2.2

Replace the sequential `db.UpdateBalance()` calls in `PersistPortfolio` with a single transaction using `BeginTx()` + `UpdateBalanceTx()`.

**Implementation — replace `PersistPortfolio` method in `internal/wallet/manager.go`:**

```go
// PersistPortfolio saves portfolio balances to the database atomically.
// All balance writes happen in a single transaction — all-or-nothing.
func (m *WalletManager) PersistPortfolio(portfolio models.PortfolioBalance) error {
	tx, err := m.db.BeginTx()
	if err != nil {
		return fmt.Errorf("persisting portfolio: begin tx: %w", err)
	}

	// Save SOL balance
	sol := portfolio.SOLBalance
	if err := m.db.UpdateBalanceTx(tx, portfolio.WalletAddress, sol.Mint, sol.Symbol,
		sol.Amount, sol.USDPrice, sol.USDValue,
	); err != nil {
		tx.Rollback()
		return fmt.Errorf("persisting SOL balance: %w", err)
	}

	// Save token balances
	for _, tb := range portfolio.TokenBalances {
		if err := m.db.UpdateBalanceTx(tx, portfolio.WalletAddress, tb.Mint, tb.Symbol,
			tb.Amount, tb.USDPrice, tb.USDValue,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("persisting balance for %s: %w", tb.Symbol, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("persisting portfolio: commit: %w", err)
	}

	return nil
}
```

**Test additions for `internal/wallet/manager_test.go`:**

The implementer should read the existing test file and add:

```go
func TestPersistPortfolioAtomic(t *testing.T) {
	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	// Insert wallet
	w := models.Wallet{Address: "persistTestAddr", Label: "persist-test", IsPrimary: true}
	if err := db.InsertWallet(w); err != nil {
		t.Fatalf("insert wallet: %v", err)
	}

	mgr := wallet.NewWalletManager(db, nil)

	portfolio := models.PortfolioBalance{
		WalletAddress: "persistTestAddr",
		SOLBalance: models.TokenBalance{
			Mint: "So11111111111111111111111111111111111111112", Symbol: "SOL",
			Amount: 5.0, USDPrice: 150.0, USDValue: 750.0,
		},
		TokenBalances: []models.TokenBalance{
			{Mint: "USDCmint", Symbol: "USDC", Amount: 100.0, USDPrice: 1.0, USDValue: 100.0},
			{Mint: "BONKmint", Symbol: "BONK", Amount: 1000000.0, USDPrice: 0.00001, USDValue: 10.0},
		},
		TotalUSD: 860.0,
	}

	if err := mgr.PersistPortfolio(portfolio); err != nil {
		t.Fatalf("PersistPortfolio failed: %v", err)
	}

	// Verify all 3 balances were written
	balances, err := db.GetBalancesForWallet("persistTestAddr")
	if err != nil {
		t.Fatalf("GetBalancesForWallet: %v", err)
	}
	if len(balances) != 3 {
		t.Fatalf("expected 3 balances, got %d", len(balances))
	}
}
```

**Verify:** `go test ./internal/wallet/ -run TestPersistPortfolio -v`
**Commit:** `perf(wallet): batch PersistPortfolio writes in single DB transaction`

---

## Final Verification

After all batches complete, run the full test suite:

```bash
go test ./... -count=1
```

All 24 packages must pass. The context.Context changes are purely additive (all callers pass `context.Background()`), so behavior is unchanged — only the plumbing is in place for future cancellation/timeout support.

---

## Summary of Changes by File

| File | Changes | Fix |
|------|---------|-----|
| `internal/blockchain/rpc.go` | Narrow mutex, context param, transport-only rotation | H1, H10, M2 |
| `internal/blockchain/rpc_test.go` | Add context.Background(), new concurrency + cancellation tests | H1, H10, M2 |
| `internal/blockchain/solana.go` | Add ctx param to all 11 functions | M2 |
| `internal/blockchain/solana_test.go` | Add context.Background() to all test calls | M2 |
| `internal/blockchain/oracle.go` | Add ctx param, use NewRequestWithContext | M2 |
| `internal/blockchain/oracle_test.go` | Add context.Background() to all test calls | M2 |
| `internal/database/database.go` | Add BeginTx() method | M12 |
| `internal/database/database_test.go` | Test BeginTx commit/rollback | M12 |
| `internal/database/balances.go` | Add UpdateBalanceTx() method | M12 |
| `internal/database/balances_test.go` | Test UpdateBalanceTx with commit/rollback | M12 |
| `internal/wallet/balance.go` | Pass context.Background() to 3 blockchain calls | M2 |
| `internal/wallet/manager.go` | Use BeginTx + UpdateBalanceTx in PersistPortfolio | M12 |
| `internal/wallet/manager_test.go` | Test atomic PersistPortfolio | M12 |
| `internal/services/transfer.go` | Pass context.Background() to 8 blockchain calls | M2 |
| `internal/services/activity.go` | Pass context + parallelize GetTransaction (semaphore=5) | M2, H6 |
| `internal/services/activity_test.go` | Test parallel GetTransaction | H6 |
| `internal/services/token_info.go` | Pass context.Background() to 2 blockchain calls | M2 |
| `internal/dex/router.go` | Update getSOLPrice func signature to accept context | M2 |
| `internal/dex/router_test.go` | Update mock function signatures | M2 |
