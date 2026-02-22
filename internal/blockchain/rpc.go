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
