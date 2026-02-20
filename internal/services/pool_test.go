package services_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
)

func setupPoolTestDB(t *testing.T) *database.Database {
	t.Helper()
	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}
	if err := db.MigrateSchema(); err != nil {
		t.Fatalf("MigrateSchema failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestPoolService_DiscoverAndStorePools(t *testing.T) {
	dexResponse := `{"pairs":[
		{"chainId":"solana","dexId":"raydium","pairAddress":"pool1",
		 "baseToken":{"address":"mint1","name":"Token1","symbol":"TK1"},
		 "quoteToken":{"address":"sol","name":"SOL","symbol":"SOL"},
		 "priceUsd":"1.50","priceNative":"0.01",
		 "txns":{"h24":{"buys":100,"sells":50}},
		 "volume":{"h24":500000},
		 "priceChange":{"m5":0.1,"h1":1.2,"h6":3.4,"h24":5.2},
		 "liquidity":{"usd":100000,"base":100,"quote":200},
		 "fdv":1000000,"marketCap":800000,"pairCreatedAt":1700000000},
		{"chainId":"solana","dexId":"orca","pairAddress":"pool2",
		 "baseToken":{"address":"mint1","name":"Token1","symbol":"TK1"},
		 "quoteToken":{"address":"usdc","name":"USDC","symbol":"USDC"},
		 "priceUsd":"1.51","priceNative":"1.51",
		 "txns":{"h24":{"buys":200,"sells":100}},
		 "volume":{"h24":800000},
		 "priceChange":{"m5":0.2,"h1":1.5,"h6":3.8,"h24":5.5},
		 "liquidity":{"usd":200000,"base":200,"quote":300},
		 "fdv":1000000,"marketCap":800000,"pairCreatedAt":1700000000}
	]}`

	tests := []struct {
		name         string
		token        models.Token
		forceRefresh bool
		serverResp   string
		serverStatus int
		wantErr      bool
		errContains  string
	}{
		{
			name: "discover pools successfully",
			token: models.Token{
				Symbol:          "TK1",
				Name:            "Token1",
				ContractAddress: "mint1",
				Chain:           "solana",
			},
			forceRefresh: false,
			serverResp:   dexResponse,
			serverStatus: http.StatusOK,
		},
		{
			name: "force refresh deletes old pools",
			token: models.Token{
				Symbol:          "TK1",
				Name:            "Token1",
				ContractAddress: "mint1",
				Chain:           "solana",
			},
			forceRefresh: true,
			serverResp:   dexResponse,
			serverStatus: http.StatusOK,
		},
		{
			name: "no pools found after filtering",
			token: models.Token{
				Symbol:          "TK2",
				Name:            "Token2",
				ContractAddress: "mint2",
				Chain:           "solana",
			},
			serverResp:   `{"pairs":[{"chainId":"ethereum","dexId":"uniswap","pairAddress":"pool3","baseToken":{"address":"mint2","name":"Token2","symbol":"TK2"},"quoteToken":{"address":"eth","name":"ETH","symbol":"ETH"},"priceUsd":"1.00","priceNative":"0.001","txns":{"h24":{"buys":10,"sells":5}},"volume":{"h24":100},"priceChange":{"m5":0,"h1":0,"h6":0,"h24":0},"liquidity":{"usd":500,"base":10,"quote":20},"fdv":10000,"marketCap":8000,"pairCreatedAt":1700000000}]}`,
			serverStatus: http.StatusOK,
			wantErr:      true,
		},
		{
			name: "dexscreener API failure",
			token: models.Token{
				Symbol:          "TK3",
				Name:            "Token3",
				ContractAddress: "mint3",
				Chain:           "solana",
			},
			serverStatus: http.StatusInternalServerError,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.serverStatus)
				if tt.serverResp != "" {
					fmt.Fprint(w, tt.serverResp)
				}
			}))
			defer server.Close()

			db := setupPoolTestDB(t)

			// Insert token into DB so InsertPool can find it
			if err := db.InsertToken(tt.token); err != nil {
				t.Fatalf("InsertToken failed: %v", err)
			}

			dexClient := dex.NewDexScreenerClientWithHTTP(server.Client(), server.URL+"/")
			svc := services.NewPoolService(dexClient, db)

			pool, err := svc.DiscoverAndStorePools(tt.token, tt.forceRefresh)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify pool was returned
			if pool.PoolAddress == "" {
				t.Error("expected non-empty pool address")
			}
			if pool.LiquidityUSD <= 0 {
				t.Error("expected positive liquidity")
			}

			// Verify pools were stored in DB
			stats, err := db.GetPoolStats(tt.token.Symbol)
			if err != nil {
				t.Fatalf("GetPoolStats failed: %v", err)
			}
			if stats.PoolCount == 0 {
				t.Error("expected pools to be stored in DB")
			}
		})
	}
}

func TestPoolService_DiscoverAndStorePools_EmptyPairs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"pairs":[]}`)
	}))
	defer server.Close()

	db := setupPoolTestDB(t)
	token := models.Token{
		Symbol:          "EMPTY",
		Name:            "Empty Token",
		ContractAddress: "emptymint",
		Chain:           "solana",
	}
	if err := db.InsertToken(token); err != nil {
		t.Fatalf("InsertToken failed: %v", err)
	}

	dexClient := dex.NewDexScreenerClientWithHTTP(server.Client(), server.URL+"/")
	svc := services.NewPoolService(dexClient, db)

	_, err := svc.DiscoverAndStorePools(token, false)
	if err == nil {
		t.Fatal("expected error for empty pairs")
	}
	if !errors.Is(err, models.ErrNoPoolsFound) {
		t.Errorf("expected ErrNoPoolsFound, got: %v", err)
	}
}
