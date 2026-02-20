package blockchain_test

import (
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
	balance, err := blockchain.GetBalance(client, "11111111111111111111111111111111")
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
	accounts, err := blockchain.GetTokenAccountsByOwner(client, "ownerAddr")
	if err != nil {
		t.Fatalf("GetTokenAccountsByOwner failed: %v", err)
	}

	if len(accounts) != 2 {
		t.Fatalf("expected 2 token accounts, got %d", len(accounts))
	}

	// Check first account (USDC)
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

	// Check second account (BONK)
	if accounts[1].Pubkey != "tokenAcct2" {
		t.Errorf("account[1].Pubkey = %q, want %q", accounts[1].Pubkey, "tokenAcct2")
	}
	if accounts[1].Amount != 50000000000 {
		t.Errorf("account[1].Amount = %d, want 50000000000", accounts[1].Amount)
	}
}

func TestGetAccountInfo(t *testing.T) {
	// "SGVsbG8gV29ybGQ=" is base64 for "Hello World"
	server := mockRPCServer(t, map[string]string{
		"getAccountInfo": `{"context":{"slot":123},"value":{"data":["SGVsbG8gV29ybGQ=","base64"],"executable":false,"lamports":1000000,"owner":"11111111111111111111111111111111"}}`,
	})
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	data, err := blockchain.GetAccountInfo(client, "someAddress")
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
	data, err := blockchain.GetAccountInfo(client, "nonExistentAddress")
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
	amount, decimals, uiAmount, err := blockchain.GetTokenAccountBalance(client, "vaultAddr")
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
	supply, decimals, err := blockchain.GetTokenSupply(client, "mintAddr")
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
	accounts, err := blockchain.GetTokenLargestAccounts(client, "mintAddr")
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
