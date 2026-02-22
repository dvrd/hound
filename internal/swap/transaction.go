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
	"github.com/dvrd/hound/internal/transaction"
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

	// SECURITY-6: Validate transaction before signing
	signerPubkey := privateKey.Public().(ed25519.PublicKey)
	if err := ValidateSwapTransaction(txBytes, signerPubkey); err != nil {
		return "", fmt.Errorf("sign transaction: %w", err)
	}

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

// allowedPrograms is the set of program IDs permitted in swap transactions.
// Any transaction containing a program not in this list is rejected.
var allowedPrograms = map[[32]byte]string{
	// System Program: 11111111111111111111111111111111
	{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}: "System Program",
	// SPL Token: TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA
	{6, 221, 246, 225, 215, 101, 161, 147, 217, 203, 225, 70, 206, 235, 121, 172, 28, 180, 133, 237, 95, 91, 55, 145, 58, 140, 245, 133, 126, 255, 0, 169}: "SPL Token",
	// SPL Token 2022: TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb
	{6, 221, 246, 225, 238, 117, 143, 122, 174, 212, 176, 214, 175, 5, 218, 193, 72, 153, 181, 89, 75, 97, 16, 4, 103, 185, 56, 234, 174, 61, 63, 214}: "SPL Token 2022",
	// Associated Token Account: ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL
	{140, 151, 37, 143, 78, 36, 137, 241, 187, 61, 16, 41, 20, 142, 13, 131, 11, 90, 19, 153, 218, 255, 16, 132, 4, 142, 123, 216, 219, 233, 248, 89}: "Associated Token Account",
	// Compute Budget: ComputeBudget111111111111111111111111111111
	{3, 6, 70, 111, 229, 33, 23, 50, 255, 236, 173, 186, 114, 195, 155, 231, 188, 140, 229, 187, 197, 247, 18, 107, 44, 67, 155, 58, 64, 0, 0, 0}: "Compute Budget",
	// Jupiter v6: JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4
	{4, 121, 213, 225, 45, 20, 76, 32, 194, 10, 198, 190, 3, 34, 174, 111, 31, 89, 100, 61, 174, 213, 235, 144, 143, 155, 127, 15, 52, 1, 56, 195}: "Jupiter v6",
	// Jupiter DCA: DCA265Vj8a9CEuX1eb1LWRnDT7uK6q1xMipnNyatn23M
	{186, 142, 191, 47, 227, 237, 190, 233, 230, 42, 10, 241, 45, 51, 250, 9, 25, 56, 68, 185, 133, 246, 186, 46, 141, 178, 28, 91, 132, 62, 78, 72}: "Jupiter DCA",
}

// ValidateSwapTransaction parses a Solana transaction and validates:
// 1. The fee payer (first account key) matches signerPubkey
// 2. All program IDs referenced by instructions are in the allowlist
// Returns ErrUntrustedTransaction if validation fails.
func ValidateSwapTransaction(txBytes []byte, signerPubkey []byte) error {
	if len(txBytes) < 134 {
		return fmt.Errorf("validate swap tx: too short (%d bytes): %w", len(txBytes), models.ErrUntrustedTransaction)
	}

	numSigs := int(txBytes[0])
	if numSigs < 1 || numSigs > 4 {
		return fmt.Errorf("validate swap tx: unexpected signature count %d: %w", numSigs, models.ErrUntrustedTransaction)
	}
	msgStart := 1 + 64*numSigs
	if msgStart >= len(txBytes) {
		return fmt.Errorf("validate swap tx: message start beyond tx bounds: %w", models.ErrUntrustedTransaction)
	}
	msg := txBytes[msgStart:]

	if len(msg) < 3 {
		return fmt.Errorf("validate swap tx: message too short for header: %w", models.ErrUntrustedTransaction)
	}
	msg = msg[3:]

	numAccounts, consumed, err := transaction.DecodeCompactU16(msg)
	if err != nil {
		return fmt.Errorf("validate swap tx: decode num accounts: %w", models.ErrUntrustedTransaction)
	}
	msg = msg[consumed:]

	accountKeysSize := int(numAccounts) * 32
	if len(msg) < accountKeysSize {
		return fmt.Errorf("validate swap tx: not enough bytes for %d accounts: %w", numAccounts, models.ErrUntrustedTransaction)
	}

	accountKeys := make([][32]byte, numAccounts)
	for i := 0; i < int(numAccounts); i++ {
		copy(accountKeys[i][:], msg[i*32:(i+1)*32])
	}
	msg = msg[accountKeysSize:]

	if len(signerPubkey) != 32 {
		return fmt.Errorf("validate swap tx: signer pubkey must be 32 bytes: %w", models.ErrUntrustedTransaction)
	}
	if !bytes.Equal(accountKeys[0][:], signerPubkey) {
		return fmt.Errorf("validate swap tx: fee payer mismatch: %w", models.ErrUntrustedTransaction)
	}

	if len(msg) < 32 {
		return fmt.Errorf("validate swap tx: not enough bytes for blockhash: %w", models.ErrUntrustedTransaction)
	}
	msg = msg[32:]

	numInstructions, consumed, err := transaction.DecodeCompactU16(msg)
	if err != nil {
		return fmt.Errorf("validate swap tx: decode num instructions: %w", models.ErrUntrustedTransaction)
	}
	msg = msg[consumed:]

	for i := 0; i < int(numInstructions); i++ {
		if len(msg) < 1 {
			return fmt.Errorf("validate swap tx: instruction %d: unexpected end: %w", i, models.ErrUntrustedTransaction)
		}

		programIdx := int(msg[0])
		msg = msg[1:]

		if programIdx >= int(numAccounts) {
			return fmt.Errorf("validate swap tx: instruction %d: program index %d out of range: %w", i, programIdx, models.ErrUntrustedTransaction)
		}

		if _, ok := allowedPrograms[accountKeys[programIdx]]; !ok {
			return fmt.Errorf("validate swap tx: instruction %d: untrusted program at index %d: %w", i, programIdx, models.ErrUntrustedTransaction)
		}

		numAccountIndices, consumed, err := transaction.DecodeCompactU16(msg)
		if err != nil {
			return fmt.Errorf("validate swap tx: instruction %d: decode account indices count: %w", i, models.ErrUntrustedTransaction)
		}
		msg = msg[consumed:]
		if len(msg) < int(numAccountIndices) {
			return fmt.Errorf("validate swap tx: instruction %d: not enough bytes for account indices: %w", i, models.ErrUntrustedTransaction)
		}
		msg = msg[numAccountIndices:]

		dataLen, consumed, err := transaction.DecodeCompactU16(msg)
		if err != nil {
			return fmt.Errorf("validate swap tx: instruction %d: decode data length: %w", i, models.ErrUntrustedTransaction)
		}
		msg = msg[consumed:]
		if len(msg) < int(dataLen) {
			return fmt.Errorf("validate swap tx: instruction %d: not enough bytes for data: %w", i, models.ErrUntrustedTransaction)
		}
		msg = msg[dataLen:]
	}

	return nil
}
