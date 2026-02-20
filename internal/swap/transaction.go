package swap

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/dvrd/hound/internal/models"
)

// SignTransaction signs a base64-encoded Solana transaction with the given private key.
// The transaction must have exactly 1 signature slot (first byte == 1).
// Returns the signed transaction as base64.
func SignTransaction(txBase64 string, privateKey ed25519.PrivateKey) (string, error) {
	// Decode base64 transaction
	txBytes, err := base64.StdEncoding.DecodeString(txBase64)
	if err != nil {
		return "", fmt.Errorf("sign transaction: decode base64: %w", models.ErrInvalidTransaction)
	}

	// Validate minimum length: 1 byte (num sigs) + 64 bytes (signature slot) = 65
	if len(txBytes) < 65 {
		return "", fmt.Errorf("sign transaction: too short (%d bytes): %w", len(txBytes), models.ErrInvalidTransaction)
	}

	// Validate first byte is 1 (exactly 1 signature slot)
	if txBytes[0] != 1 {
		return "", fmt.Errorf("sign transaction: expected 1 signature slot, got %d: %w", txBytes[0], models.ErrInvalidTransaction)
	}

	// Extract message (everything after the signature slots)
	message := txBytes[65:]

	// Sign the message
	signature := ed25519.Sign(privateKey, message)

	// Copy 64-byte signature into bytes[1:65]
	copy(txBytes[1:65], signature)

	// Encode back to base64
	return base64.StdEncoding.EncodeToString(txBytes), nil
}

// SubmitTransaction submits a signed transaction to the Jupiter Ultra execute endpoint.
func SubmitTransaction(httpClient *http.Client, baseURL string, signedTx string, requestID string) (models.SwapTransactionResult, error) {
	// Build request body
	reqBody := struct {
		SignedTransaction string `json:"signedTransaction"`
		RequestID         string `json:"requestId"`
	}{
		SignedTransaction: signedTx,
		RequestID:         requestID,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return models.SwapTransactionResult{}, fmt.Errorf("submit transaction: marshal body: %w", err)
	}

	// POST to execute endpoint
	url := baseURL + "/ultra/v1/execute"
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return models.SwapTransactionResult{}, fmt.Errorf("submit transaction: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.SwapTransactionResult{}, fmt.Errorf("submit transaction: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return models.SwapTransactionResult{
			Status:       "failed",
			ErrorMessage: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
		}, fmt.Errorf("submit transaction HTTP %d: %w", resp.StatusCode, models.ErrConnectionFailed)
	}

	// Parse response
	var raw struct {
		Signature string `json:"signature"`
		Status    string `json:"status"`
		Slot      int64  `json:"slot"`
		Error     string `json:"error"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return models.SwapTransactionResult{}, fmt.Errorf("submit transaction: parse response: %w", models.ErrInvalidResponse)
	}

	result := models.SwapTransactionResult{
		Signature:    raw.Signature,
		Slot:         raw.Slot,
		Status:       raw.Status,
		ErrorMessage: raw.Error,
	}

	if result.Status == "" {
		result.Status = "confirmed"
	}

	return result, nil
}
