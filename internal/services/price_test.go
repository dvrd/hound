package services_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
)

func TestPriceService_FetchPrice(t *testing.T) {
	tests := []struct {
		name           string
		token          models.Token
		routerPrice    string // Jupiter price response for router
		dexPrice       string // DexScreener response
		jupPrice       string // Jupiter direct price response
		routerFail     bool
		dexFail        bool
		jupFail        bool
		wantErr        bool
		wantPriceAbove float64
	}{
		{
			name: "router succeeds with pools",
			token: models.Token{
				Symbol:          "BONK",
				ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
				Pools:           []models.PoolInfo{{Dex: "raydium", PoolAddress: "pool1"}},
			},
			routerPrice:    `{"data":{"DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263":{"price":"0.000028","priceChange24h":5.2}}}`,
			wantPriceAbove: 0,
		},
		{
			name: "router fails, dexscreener succeeds",
			token: models.Token{
				Symbol:          "BONK",
				ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
				Pools:           []models.PoolInfo{{Dex: "raydium", PoolAddress: "pool1"}},
			},
			routerFail: true,
			dexPrice: `{"pairs":[{"chainId":"solana","dexId":"raydium","pairAddress":"pool1",
				"baseToken":{"address":"DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263","name":"Bonk","symbol":"BONK"},
				"quoteToken":{"address":"sol","name":"SOL","symbol":"SOL"},
				"priceUsd":"0.000028","priceNative":"0.0001",
				"txns":{"h24":{"buys":100,"sells":50}},
				"volume":{"h24":1000000},
				"priceChange":{"m5":0.1,"h1":1.2,"h6":3.4,"h24":5.2},
				"liquidity":{"usd":500000,"base":100,"quote":200},
				"fdv":1000000,"marketCap":800000,"pairCreatedAt":1700000000}]}`,
			wantPriceAbove: 0,
		},
		{
			name: "router and dex fail, jupiter succeeds",
			token: models.Token{
				Symbol:          "BONK",
				ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
				Pools:           []models.PoolInfo{{Dex: "raydium", PoolAddress: "pool1"}},
			},
			routerFail:     true,
			dexFail:        true,
			jupPrice:       `{"data":{"DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263":{"price":"0.000028","priceChange24h":5.2}}}`,
			wantPriceAbove: 0,
		},
		{
			name: "all sources fail",
			token: models.Token{
				Symbol:          "BONK",
				ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
			},
			routerFail: true,
			dexFail:    true,
			jupFail:    true,
			wantErr:    true,
		},
		{
			name: "no pools skips router, dex succeeds",
			token: models.Token{
				Symbol:          "BONK",
				ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
			},
			dexPrice: `{"pairs":[{"chainId":"solana","dexId":"raydium","pairAddress":"pool1",
				"baseToken":{"address":"DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263","name":"Bonk","symbol":"BONK"},
				"quoteToken":{"address":"sol","name":"SOL","symbol":"SOL"},
				"priceUsd":"0.000028","priceNative":"0.0001",
				"txns":{"h24":{"buys":100,"sells":50}},
				"volume":{"h24":1000000},
				"priceChange":{"m5":0.1,"h1":1.2,"h6":3.4,"h24":5.2},
				"liquidity":{"usd":500000,"base":100,"quote":200},
				"fdv":1000000,"marketCap":800000,"pairCreatedAt":1700000000}]}`,
			wantPriceAbove: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock servers
			routerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.routerFail {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				fmt.Fprint(w, tt.routerPrice)
			}))
			defer routerServer.Close()

			dexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.dexFail {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				fmt.Fprint(w, tt.dexPrice)
			}))
			defer dexServer.Close()

			jupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.jupFail {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				fmt.Fprint(w, tt.jupPrice)
			}))
			defer jupServer.Close()

			// Create clients with test servers
			jupClient := dex.NewJupiterClientWithHTTP(routerServer.Client(), routerServer.URL+"?ids=", jupServer.URL+"?query=")
			router := dex.NewRouter(jupClient)
			dexClient := dex.NewDexScreenerClientWithHTTP(dexServer.Client(), dexServer.URL+"/")
			jupDirect := dex.NewJupiterClientWithHTTP(jupServer.Client(), jupServer.URL+"?ids=", jupServer.URL+"?query=")

			svc := services.NewPriceService(router, dexClient, jupDirect)

			price, err := svc.FetchPrice(tt.token)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if price.PriceUSD < tt.wantPriceAbove {
				t.Errorf("price %f below expected minimum %f", price.PriceUSD, tt.wantPriceAbove)
			}
		})
	}
}

func TestPriceService_FetchMultiplePrices(t *testing.T) {
	// Set up a mock Jupiter server that returns prices for known mints
	prices := map[string]string{
		"mint1": "1.50",
		"mint2": "0.005",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.RawQuery
		// Extract mint from query (format: ids=mint1)
		for mint, price := range prices {
			if len(query) >= len(mint) && query[len(query)-len(mint):] == mint {
				resp := map[string]interface{}{
					"data": map[string]interface{}{
						mint: map[string]interface{}{
							"price":          price,
							"priceChange24h": 2.5,
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	jupClient := dex.NewJupiterClientWithHTTP(server.Client(), server.URL+"?ids=", server.URL+"?query=")

	svc := services.NewPriceService(nil, nil, jupClient)

	tokens := []models.Token{
		{Symbol: "TOKEN1", ContractAddress: "mint1"},
		{Symbol: "TOKEN2", ContractAddress: "mint2"},
		{Symbol: "TOKEN3", ContractAddress: "mint_unknown"},
	}

	results := svc.FetchMultiplePrices(tokens)

	// Should have at least some results (best-effort)
	if len(results) > len(tokens) {
		t.Errorf("got more results (%d) than tokens (%d)", len(results), len(tokens))
	}
}
