package blockchain

import (
	"bytes"
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
func (c *RPCClient) Call(method string, params []interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	maxAttempts := 1 + len(c.backupEndpoints)
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Get current endpoint
		endpoint := c.getEndpoint()

		// Build request
		req := RPCRequest{
			JSONRPC: "2.0",
			ID:      c.requestID,
			Method:  method,
			Params:  params,
		}
		c.requestID++

		// Marshal to JSON
		body, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal RPC request: %w", err)
		}

		// HTTP POST
		resp, err := c.httpClient.Post(endpoint, "application/json", bytes.NewReader(body))
		if err != nil {
			lastErr = fmt.Errorf("RPC POST to %s: %w", endpoint, err)
			c.rotateEndpoint(maxAttempts)
			continue
		}

		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read RPC response from %s: %w", endpoint, err)
			c.rotateEndpoint(maxAttempts)
			continue
		}

		// Check HTTP status
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("RPC HTTP %d from %s", resp.StatusCode, endpoint)
			c.rotateEndpoint(maxAttempts)
			continue
		}

		// Unmarshal response
		var rpcResp RPCResponse
		if err := json.Unmarshal(respBody, &rpcResp); err != nil {
			lastErr = fmt.Errorf("unmarshal RPC response from %s: %w", endpoint, err)
			c.rotateEndpoint(maxAttempts)
			continue
		}

		// Check for RPC error
		if rpcResp.Error != nil {
			lastErr = rpcResp.Error
			c.rotateEndpoint(maxAttempts)
			continue
		}

		// Success
		return rpcResp.Result, nil
	}

	return nil, fmt.Errorf("%w: %v", models.ErrRPCConnectionFailed, lastErr)
}

// getEndpoint returns the current endpoint based on currentIndex.
func (c *RPCClient) getEndpoint() string {
	if c.currentIndex == 0 {
		return c.endpoint
	}
	return c.backupEndpoints[c.currentIndex-1]
}

// rotateEndpoint advances to the next endpoint.
func (c *RPCClient) rotateEndpoint(maxAttempts int) {
	c.currentIndex = (c.currentIndex + 1) % maxAttempts
}
