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
