package dex_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
)

func TestRouterFetchPriceJupiterFallback(t *testing.T) {
	mint := "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":{"%s":{"price":"0.000028","priceChange24h":-2.5}}}`, mint)
	}))
	defer server.Close()

	jupClient := dex.NewJupiterClientWithHTTP(server.Client(), server.URL+"?ids=", server.URL+"?query=")
	router := dex.NewRouter(jupClient)

	token := models.Token{
		Symbol:          "BONK",
		ContractAddress: mint,
		Pools: []models.PoolInfo{
			{Dex: "raydium", PoolType: "amm_v4", PoolAddress: "pool1"},
		},
	}

	price, err := router.FetchPrice(token)
	if err != nil {
		t.Fatalf("FetchPrice failed: %v", err)
	}

	if price.PriceUSD != 0.000028 {
		t.Errorf("expected price 0.000028, got %f", price.PriceUSD)
	}
	if price.Change24h != -2.5 {
		t.Errorf("expected change24h -2.5, got %f", price.Change24h)
	}
}

func TestRouterFetchPriceNoPools(t *testing.T) {
	mint := "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":{"%s":{"price":"0.000028","priceChange24h":1.0}}}`, mint)
	}))
	defer server.Close()

	jupClient := dex.NewJupiterClientWithHTTP(server.Client(), server.URL+"?ids=", server.URL+"?query=")
	router := dex.NewRouter(jupClient)

	token := models.Token{
		Symbol:          "BONK",
		ContractAddress: mint,
		Pools:           nil,
	}

	price, err := router.FetchPrice(token)
	if err != nil {
		t.Fatalf("FetchPrice with no pools failed: %v", err)
	}
	if price.PriceUSD != 0.000028 {
		t.Errorf("expected price 0.000028, got %f", price.PriceUSD)
	}
}
