---
date: 2026-02-22
topic: "Batch 3 — Make It Fast (Performance)"
status: validated
---

# Batch 3 — "Make It Fast" Performance Optimization

## Problem Statement

The audit found 5 performance issues that limit concurrency and throughput. The most impactful: a global mutex in the RPC client serializes ALL HTTP calls — even independent requests block each other. Combined with N+1 sequential patterns in activity fetching and portfolio persistence, the app is far slower than it needs to be.

## Constraints

- No CGO — pure Go only
- All 24 test packages must continue passing
- context.Context plumbing is additive — existing callers pass context.Background() initially
- Semaphore concurrency for GetTransaction bounded to avoid rate limiting
- SQLite single-connection (SetMaxOpenConns(1)) means DB transaction wrapping is safe

## Fixes (5 total)

### Fix 1 — H1: Narrow RPC Mutex (HIGH)

**Problem:** `RPCClient.Call()` holds `c.mu.Lock()` for the entire HTTP round-trip (up to 10s × maxAttempts). All RPC calls are serialized globally.

**Solution:** The mutex only protects shared mutable state: `c.requestID` and `c.currentIndex`. Restructure `Call()`:
1. Lock → copy `requestID`, increment it, copy current `endpoint` → unlock
2. Build request, marshal JSON (no shared state needed)
3. HTTP POST (no lock held)
4. Read + parse response (no lock held)
5. On transport failure only: lock → `rotateEndpoint()` → unlock → retry loop

This allows concurrent RPC calls while protecting the ID counter and endpoint index.

**Files:** `internal/blockchain/rpc.go`

### Fix 2 — H10: Only Rotate on Transport Failures (HIGH)

**Problem:** `rotateEndpoint()` fires on RPC-level errors (`rpcResp.Error != nil`). These are application errors (e.g., "account not found", rate limits) — the endpoint itself is healthy.

**Solution:** Categorize errors into transport vs application:
- **Rotate (transport):** HTTP POST error, response body read error, non-200 HTTP status, JSON unmarshal error
- **Do NOT rotate (application):** `rpcResp.Error != nil` — return the error immediately, no retry

This is implemented naturally with Fix 1 since rotation only happens in the retry loop for transport failures.

**Files:** `internal/blockchain/rpc.go`

### Fix 3 — M2: Add context.Context to RPC Layer (MEDIUM)

**Problem:** No `context.Context` on any RPC function. No cancellation, no deadline propagation, no timeout control from callers.

**Solution:**
1. `RPCClient.Call()` gains `ctx context.Context` as first parameter
2. Replace `c.httpClient.Post(url, "application/json", body)` with `http.NewRequestWithContext(ctx, "POST", url, body)` + `c.httpClient.Do(req)`
3. All 11 functions in `solana.go` gain `ctx context.Context` as first parameter, pass through to `client.Call(ctx, ...)`
4. All callers updated to pass `context.Background()` — this is pure plumbing, no behavior change yet
5. Oracle functions (`oracle.go`) also gain context for consistency

**Files:** `internal/blockchain/rpc.go`, `internal/blockchain/solana.go`, `internal/blockchain/oracle.go`, all callers in `internal/services/`, `internal/wallet/`, `internal/swap/`, `internal/tui/views/`

### Fix 4 — H6: Parallelize GetTransaction Calls (HIGH)

**Problem:** `GetActivity()` calls `GetTransaction()` sequentially for each signature. With limit=20, that's 20 round-trips in series.

**Solution:** Fan-out with bounded concurrency using `sync.WaitGroup` + a semaphore channel:
1. Create results slice (pre-sized to len(signatures))
2. Launch goroutines for each signature, bounded by semaphore (capacity 5)
3. Each goroutine calls `GetTransaction(ctx, client, sig)`, stores result at its index
4. Wait for all to complete
5. Filter out nil results (errors/not-found), classify, continue as before

Semaphore of 5 balances throughput vs rate-limit risk. With H1 fixed, these 5 goroutines actually run concurrently.

**Files:** `internal/services/activity.go`

### Fix 5 — M12: Batch PersistPortfolio in Single Transaction (MEDIUM)

**Problem:** `PersistPortfolio()` does 1+N sequential `db.UpdateBalance()` calls. Each is an implicit SQLite transaction. Partial failures leave inconsistent state.

**Solution:**
1. Add `BeginTx() (*sql.Tx, error)` method to `database.DB` (thin wrapper around `db.conn.Begin()`)
2. Add `UpdateBalanceTx(tx *sql.Tx, ...)` that accepts a transaction handle
3. `PersistPortfolio()` calls `BeginTx()`, passes tx to all `UpdateBalanceTx()` calls, then `tx.Commit()`
4. On any error: `tx.Rollback()` — all-or-nothing semantics

**Files:** `internal/database/database.go`, `internal/database/balances.go` (or wherever UpdateBalance lives), `internal/wallet/manager.go`

## Data Flow (After Fixes)

```
GetActivity(ctx, limit=20):
  1. GetSignaturesForAddress(ctx, client, addr, 20)  → 1 RPC call
  2. Fan-out: 5 concurrent goroutines, each calls GetTransaction(ctx, client, sig)
     - Semaphore channel controls max concurrency
     - Each goroutine: lock→getID+endpoint→unlock → HTTP POST → parse → store result
     - Multiple goroutines in-flight simultaneously (H1 fix enables this)
  3. Collect results, filter nils, classify
  4. Merge with local swap history (1 DB query)
  5. Sort by timestamp, return

PersistPortfolio(portfolio):
  1. tx = db.BeginTx()
  2. UpdateBalanceTx(tx, SOL balance)
  3. for each token: UpdateBalanceTx(tx, token balance)
  4. tx.Commit()  // atomic — all or nothing
```

## Error Handling

- **RPC transport failure:** Rotate endpoint, retry with next endpoint (H10: only on transport errors)
- **RPC application error:** Return immediately, no rotation (e.g., "account not found")
- **Context cancellation:** HTTP request cancelled via context, error propagated to caller
- **GetTransaction fan-out:** Individual failures logged and skipped (same as current behavior, but concurrent)
- **PersistPortfolio rollback:** Any write failure rolls back entire transaction — no partial state

## Testing Strategy

- **RPC mutex narrowing:** Test concurrent `Call()` invocations complete without deadlock; verify requestID increments correctly under concurrency
- **Endpoint rotation:** Test that RPC errors don't trigger rotation; test that HTTP errors do
- **context.Context:** Test that cancelled context aborts in-flight request; verify all callers compile with new signatures
- **GetTransaction parallelism:** Test with mock RPC server returning varied delays; verify results maintain order; verify semaphore bounds concurrency
- **PersistPortfolio transaction:** Test rollback on mid-write failure; test all-or-nothing semantics

## Scope Note on context.Context (M2)

Adding context to every RPC function signature is a large mechanical change that touches many files. The approach:
1. Change `rpc.go` Call signature
2. Change all 11 `solana.go` functions
3. Change `oracle.go` functions
4. Update all direct callers (services, wallet manager, swap, TUI views)
5. Every caller passes `context.Background()` for now

This is pure plumbing — no behavior changes. The value comes later when we add per-request timeouts, TUI cancellation on navigation, etc.

## Open Questions

None — all decisions are made. Proceeding to implementation plan.
