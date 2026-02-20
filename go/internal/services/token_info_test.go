package services_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
)

func TestTokenInfoService_FetchExtendedTokenInfo(t *testing.T) {
	dexResponse := `{"pairs":[
		{"chainId":"solana","dexId":"raydium","pairAddress":"pool1",
		 "baseToken":{"address":"mint1","name":"Token One","symbol":"TK1","decimals":9},
		 "quoteToken":{"address":"sol","name":"SOL","symbol":"SOL"},
		 "priceUsd":"1.50","priceNative":"0.01",
		 "txns":{"h24":{"buys":100,"sells":50}},
		 "volume":{"h24":500000},
		 "priceChange":{"m5":0.1,"h1":1.2,"h6":3.4,"h24":5.2},
		 "liquidity":{"usd":100000,"base":100,"quote":200},
		 "fdv":1000000,"marketCap":800000,"pairCreatedAt":1700000000},
		{"chainId":"solana","dexId":"orca","pairAddress":"pool2",
		 "baseToken":{"address":"mint1","name":"Token One","symbol":"TK1","decimals":9},
		 "quoteToken":{"address":"usdc","name":"USDC","symbol":"USDC"},
		 "priceUsd":"1.55","priceNative":"1.55",
		 "txns":{"h24":{"buys":200,"sells":100}},
		 "volume":{"h24":800000},
		 "priceChange":{"m5":0.2,"h1":1.5,"h6":3.8,"h24":5.5},
		 "liquidity":{"usd":200000,"base":200,"quote":300},
		 "fdv":1000000,"marketCap":800000,"pairCreatedAt":1700000000}
	]}`

	tests := []struct {
		name         string
		mintOrSymbol string
		dexResp      string
		dexStatus    int
		rpcResp      map[string]interface{} // method -> response
		wantErr      bool
		checkInfo    func(t *testing.T, info models.TokenExtendedInfo)
	}{
		{
			name:         "fetch by mint address",
			mintOrSymbol: "mint1",
			dexResp:      dexResponse,
			dexStatus:    http.StatusOK,
			checkInfo: func(t *testing.T, info models.TokenExtendedInfo) {
				if info.Name != "Token One" {
					t.Errorf("expected name 'Token One', got %q", info.Name)
				}
				if info.PriceUSD != 1.55 { // best price across pairs
					t.Errorf("expected price 1.55, got %f", info.PriceUSD)
				}
				if info.Volume24h != 1300000 { // 500000 + 800000
					t.Errorf("expected volume 1300000, got %f", info.Volume24h)
				}
				if info.LiquidityUSD != 300000 { // 100000 + 200000
					t.Errorf("expected liquidity 300000, got %f", info.LiquidityUSD)
				}
				if info.Txns24h != 450 { // 100+50+200+100
					t.Errorf("expected 450 txns, got %d", info.Txns24h)
				}
				if !info.IsActive {
					t.Error("expected IsActive to be true")
				}
			},
		},
		{
			name:         "fetch by symbol via DB",
			mintOrSymbol: "TK1",
			dexResp:      dexResponse,
			dexStatus:    http.StatusOK,
			checkInfo: func(t *testing.T, info models.TokenExtendedInfo) {
				if info.Name != "Token One" {
					t.Errorf("expected name 'Token One', got %q", info.Name)
				}
			},
		},
		{
			name:         "dexscreener failure",
			mintOrSymbol: "mint_bad",
			dexStatus:    http.StatusInternalServerError,
			wantErr:      true,
		},
		{
			name:         "no pairs found",
			mintOrSymbol: "mint_empty",
			dexResp:      `{"pairs":[]}`,
			dexStatus:    http.StatusOK,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// DexScreener mock
			dexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.dexStatus)
				if tt.dexResp != "" {
					fmt.Fprint(w, tt.dexResp)
				}
			}))
			defer dexServer.Close()

			// RPC mock (for token supply and holders)
			rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req blockchain.RPCRequest
				json.NewDecoder(r.Body).Decode(&req)

				switch req.Method {
				case "getTokenSupply":
					resp := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      req.ID,
						"result": map[string]interface{}{
							"value": map[string]interface{}{
								"amount":   "1000000000000",
								"decimals": 9,
							},
						},
					}
					json.NewEncoder(w).Encode(resp)
				case "getTokenLargestAccounts":
					resp := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      req.ID,
						"result": map[string]interface{}{
							"value": []map[string]interface{}{
								{"address": "holder1", "amount": "500000000000", "decimals": 9, "uiAmount": 500.0},
								{"address": "holder2", "amount": "200000000000", "decimals": 9, "uiAmount": 200.0},
							},
						},
					}
					json.NewEncoder(w).Encode(resp)
				default:
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer rpcServer.Close()

			// Set up DB with token
			db := setupPoolTestDB(t)
			token := models.Token{
				Symbol:          "TK1",
				Name:            "Token One",
				ContractAddress: "mint1",
				Chain:           "solana",
				Decimals:        9,
			}
			_ = db.InsertToken(token)

			dexClient := dex.NewDexScreenerClientWithHTTP(dexServer.Client(), dexServer.URL+"/")
			rpcClient := blockchain.NewRPCClient(rpcServer.URL, nil)

			svc := services.NewTokenInfoService(dexClient, rpcClient)

			info, err := svc.FetchExtendedTokenInfo(tt.mintOrSymbol, db)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkInfo != nil {
				tt.checkInfo(t, info)
			}
		})
	}
}

func TestTokenInfoService_FetchExtendedTokenInfo_NilDB(t *testing.T) {
	dexResponse := `{"pairs":[
		{"chainId":"solana","dexId":"raydium","pairAddress":"pool1",
		 "baseToken":{"address":"mint1","name":"Token One","symbol":"TK1","decimals":9},
		 "quoteToken":{"address":"sol","name":"SOL","symbol":"SOL"},
		 "priceUsd":"1.50","priceNative":"0.01",
		 "txns":{"h24":{"buys":100,"sells":50}},
		 "volume":{"h24":500000},
		 "priceChange":{"m5":0.1,"h1":1.2,"h6":3.4,"h24":5.2},
		 "liquidity":{"usd":100000,"base":100,"quote":200},
		 "fdv":1000000,"marketCap":800000,"pairCreatedAt":1700000000}
	]}`

	dexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, dexResponse)
	}))
	defer dexServer.Close()

	dexClient := dex.NewDexScreenerClientWithHTTP(dexServer.Client(), dexServer.URL+"/")
	svc := services.NewTokenInfoService(dexClient, nil)

	// Should work with nil DB, using mintOrSymbol as mint directly
	info, err := svc.FetchExtendedTokenInfo("mint1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "Token One" {
		t.Errorf("expected name 'Token One', got %q", info.Name)
	}
}

func TestTokenInfoService_TopHolders(t *testing.T) {
	dexResponse := `{"pairs":[
		{"chainId":"solana","dexId":"raydium","pairAddress":"pool1",
		 "baseToken":{"address":"mint1","name":"Token One","symbol":"TK1","decimals":9},
		 "quoteToken":{"address":"sol","name":"SOL","symbol":"SOL"},
		 "priceUsd":"1.50","priceNative":"0.01",
		 "txns":{"h24":{"buys":100,"sells":50}},
		 "volume":{"h24":500000},
		 "priceChange":{"m5":0.1,"h1":1.2,"h6":3.4,"h24":5.2},
		 "liquidity":{"usd":100000,"base":100,"quote":200},
		 "fdv":1000000,"marketCap":800000,"pairCreatedAt":1700000000}
	]}`

	dexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, dexResponse)
	}))
	defer dexServer.Close()

	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "getTokenSupply":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"value": map[string]interface{}{
						"amount":   "1000000000000",
						"decimals": 9,
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "getTokenLargestAccounts":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"value": []map[string]interface{}{
						{"address": "holder1", "amount": "500000000000", "decimals": 9, "uiAmount": 500.0},
						{"address": "holder2", "amount": "200000000000", "decimals": 9, "uiAmount": 200.0},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer rpcServer.Close()

	dexClient := dex.NewDexScreenerClientWithHTTP(dexServer.Client(), dexServer.URL+"/")
	rpcClient := blockchain.NewRPCClient(rpcServer.URL, nil)
	svc := services.NewTokenInfoService(dexClient, rpcClient)

	info, err := svc.FetchExtendedTokenInfo("mint1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.TotalSupply != 1000.0 {
		t.Errorf("expected total supply 1000.0, got %f", info.TotalSupply)
	}

	if len(info.TopHolders) != 2 {
		t.Fatalf("expected 2 top holders, got %d", len(info.TopHolders))
	}

	// holder1 has 500/1000 = 50%
	if info.TopHolders[0].OwnershipPct != 50.0 {
		t.Errorf("expected holder1 ownership 50%%, got %f%%", info.TopHolders[0].OwnershipPct)
	}

	// holder2 has 200/1000 = 20%
	if info.TopHolders[1].OwnershipPct != 20.0 {
		t.Errorf("expected holder2 ownership 20%%, got %f%%", info.TopHolders[1].OwnershipPct)
	}
}
