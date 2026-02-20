package dex_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
)

const testMint = "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263"

func TestJupiterFetchPrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":{"%s":{"price":"0.000028","priceChange24h":-2.5}}}`, testMint)
	}))
	defer server.Close()

	client := dex.NewJupiterClientWithHTTP(server.Client(), server.URL+"?ids=", server.URL+"?query=")
	price, err := client.FetchPrice(testMint)
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

func TestJupiterFetchPriceCacheHit(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		fmt.Fprintf(w, `{"data":{"%s":{"price":"0.000028","priceChange24h":-2.5}}}`, testMint)
	}))
	defer server.Close()

	client := dex.NewJupiterClientWithHTTP(server.Client(), server.URL+"?ids=", server.URL+"?query=")

	_, err := client.FetchPrice(testMint)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	_, err = client.FetchPrice(testMint)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if callCount.Load() != 1 {
		t.Errorf("expected 1 HTTP call (cached), got %d", callCount.Load())
	}
}

func TestJupiterFetchPriceNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{}}`)
	}))
	defer server.Close()

	client := dex.NewJupiterClientWithHTTP(server.Client(), server.URL+"?ids=", server.URL+"?query=")
	_, err := client.FetchPrice("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing token")
	}
	if !errors.Is(err, models.ErrTokenNotFound) {
		t.Errorf("expected ErrTokenNotFound, got: %v", err)
	}
}

func TestJupiterFetchPriceServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := dex.NewJupiterClientWithHTTP(server.Client(), server.URL+"?ids=", server.URL+"?query=")
	_, err := client.FetchPrice(testMint)
	if err == nil {
		t.Fatal("expected error for server error")
	}
	if !errors.Is(err, models.ErrConnectionFailed) {
		t.Errorf("expected ErrConnectionFailed, got: %v", err)
	}
}

func TestJupiterLookupTokenMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `[{"id":"%s","name":"Bonk","symbol":"BONK","decimals":5}]`, testMint)
	}))
	defer server.Close()

	client := dex.NewJupiterClientWithHTTP(server.Client(), server.URL+"?ids=", server.URL+"?query=")
	meta, err := client.LookupTokenMetadata(testMint)
	if err != nil {
		t.Fatalf("LookupTokenMetadata failed: %v", err)
	}

	if meta.Address != testMint {
		t.Errorf("expected address %s, got %s", testMint, meta.Address)
	}
	if meta.Symbol != "BONK" {
		t.Errorf("expected symbol BONK, got %s", meta.Symbol)
	}
	if meta.Name != "Bonk" {
		t.Errorf("expected name Bonk, got %s", meta.Name)
	}
	if meta.Decimals != 5 {
		t.Errorf("expected decimals 5, got %d", meta.Decimals)
	}
}

func TestJupiterLookupTokenMetadataNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	}))
	defer server.Close()

	client := dex.NewJupiterClientWithHTTP(server.Client(), server.URL+"?ids=", server.URL+"?query=")
	_, err := client.LookupTokenMetadata("nonexistent")
	if err == nil {
		t.Fatal("expected error for not found token")
	}
	if !errors.Is(err, models.ErrTokenNotFound) {
		t.Errorf("expected ErrTokenNotFound, got: %v", err)
	}
}
