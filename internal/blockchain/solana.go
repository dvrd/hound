package blockchain

import (
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
func GetBalance(client *RPCClient, address string) (uint64, error) {
	result, err := client.Call("getBalance", []interface{}{address})
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
func GetTokenAccountsByOwner(client *RPCClient, address string) ([]TokenAccount, error) {
	params := []interface{}{
		address,
		map[string]string{
			"programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
		},
		map[string]string{
			"encoding": "jsonParsed",
		},
	}

	result, err := client.Call("getTokenAccountsByOwner", params)
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
func GetAccountInfo(client *RPCClient, address string) ([]byte, error) {
	params := []interface{}{
		address,
		map[string]string{
			"encoding":   "base64",
			"commitment": "confirmed",
		},
	}

	result, err := client.Call("getAccountInfo", params)
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
func GetTokenAccountBalance(client *RPCClient, vaultAddr string) (amount uint64, decimals int, uiAmount float64, err error) {
	result, err := client.Call("getTokenAccountBalance", []interface{}{vaultAddr})
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
func GetTokenSupply(client *RPCClient, mintAddr string) (totalSupply uint64, decimals int, err error) {
	result, err := client.Call("getTokenSupply", []interface{}{mintAddr})
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
func GetTokenLargestAccounts(client *RPCClient, mintAddr string) ([]AccountBalance, error) {
	result, err := client.Call("getTokenLargestAccounts", []interface{}{mintAddr})
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
func GetLatestBlockhash(client *RPCClient) (string, uint64, error) {
	result, err := client.Call("getLatestBlockhash", []interface{}{
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
func SendTransaction(client *RPCClient, base64Tx string) (string, error) {
	result, err := client.Call("sendTransaction", []interface{}{
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
func GetSignaturesForAddress(client *RPCClient, address string, limit int, before string) ([]SignatureInfo, error) {
	opts := map[string]interface{}{"limit": limit}
	if before != "" {
		opts["before"] = before
	}

	result, err := client.Call("getSignaturesForAddress", []interface{}{address, opts})
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
func GetTransaction(client *RPCClient, signature string) (*TransactionDetail, error) {
	result, err := client.Call("getTransaction", []interface{}{
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
func GetMinimumBalanceForRentExemption(client *RPCClient, dataSize uint64) (uint64, error) {
	result, err := client.Call("getMinimumBalanceForRentExemption", []interface{}{dataSize})
	if err != nil {
		return 0, fmt.Errorf("getMinimumBalanceForRentExemption: %w", err)
	}

	var lamports uint64
	if err := json.Unmarshal(result, &lamports); err != nil {
		return 0, fmt.Errorf("getMinimumBalanceForRentExemption: parse result: %w", models.ErrRPCInvalidResponse)
	}

	return lamports, nil
}
