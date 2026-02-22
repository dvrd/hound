package swap_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/swap"
)

const jupiterUltraResponse = `{
	"requestId": "req-123",
	"inputMint": "So11111111111111111111111111111111111111112",
	"outputMint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
	"inAmount": "1000000000",
	"outAmount": "150000000",
	"swapMode": "ExactIn",
	"slippageBps": 50,
	"priceImpactPct": "0.01",
	"routePlan": [
		{
			"swapInfo": {
				"ammKey": "amm-key-1",
				"label": "Raydium",
				"inputMint": "So11111111111111111111111111111111111111112",
				"outputMint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"inAmount": "1000000000",
				"outAmount": "150000000",
				"feeAmount": "25000",
				"feeMint": "So11111111111111111111111111111111111111112"
			},
			"percent": 100
		}
	],
	"transaction": "AQAAAA==",
	"prioritizationFeeLamports": 5000
}`

func TestSwapClient_GetQuote(t *testing.T) {
	tests := []struct {
		name         string
		serverResp   string
		serverStatus int
		wantErr      bool
		checkQuote   func(t *testing.T, q models.SwapQuote)
	}{
		{
			name:         "successful quote",
			serverResp:   jupiterUltraResponse,
			serverStatus: http.StatusOK,
			checkQuote: func(t *testing.T, q models.SwapQuote) {
				if q.InputMint != "So11111111111111111111111111111111111111112" {
					t.Errorf("unexpected input mint: %s", q.InputMint)
				}
				if q.OutputMint != "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" {
					t.Errorf("unexpected output mint: %s", q.OutputMint)
				}
				if q.InAmount != "1000000000" {
					t.Errorf("unexpected in amount: %s", q.InAmount)
				}
				if q.OutAmount != "150000000" {
					t.Errorf("unexpected out amount: %s", q.OutAmount)
				}
				if q.SlippageBps != 50 {
					t.Errorf("unexpected slippage: %d", q.SlippageBps)
				}
				if q.PriceImpactPct != 0.01 {
					t.Errorf("unexpected price impact: %f", q.PriceImpactPct)
				}
				if len(q.RoutePlan) != 1 {
					t.Fatalf("expected 1 route step, got %d", len(q.RoutePlan))
				}
				if q.RoutePlan[0].DexLabel != "Raydium" {
					t.Errorf("unexpected dex label: %s", q.RoutePlan[0].DexLabel)
				}
				if q.RoutePlan[0].Percent != 100 {
					t.Errorf("unexpected percent: %d", q.RoutePlan[0].Percent)
				}
				if q.Rate <= 0 {
					t.Error("expected positive rate")
				}
				if q.FetchedAt.IsZero() {
					t.Error("expected non-zero FetchedAt")
				}
				if q.RawResponse == nil {
					t.Error("expected non-nil RawResponse")
				}
			},
		},
		{
			name:         "server error",
			serverStatus: http.StatusInternalServerError,
			serverResp:   `{"error":"internal error"}`,
			wantErr:      true,
		},
		{
			name:         "invalid JSON response",
			serverStatus: http.StatusOK,
			serverResp:   `{invalid json`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request path
				if r.URL.Path != "/ultra/v1/order" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.serverStatus)
				fmt.Fprint(w, tt.serverResp)
			}))
			defer server.Close()

			client := swap.NewSwapClientWithHTTP(server.Client(), server.URL)

			quote, err := client.GetQuote(
				"So11111111111111111111111111111111111111112",
				"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"1000000000",
				"taker-address",
			)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkQuote != nil {
				tt.checkQuote(t, quote)
			}
		})
	}
}

func TestSwapClient_GetQuote_Caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		fmt.Fprint(w, jupiterUltraResponse)
	}))
	defer server.Close()

	client := swap.NewSwapClientWithHTTP(server.Client(), server.URL)

	// First call
	_, err := client.GetQuote("mint1", "mint2", "1000", "taker")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call should be cached
	_, err = client.GetQuote("mint1", "mint2", "1000", "taker")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 server call (cached), got %d", callCount)
	}

	// Different amount should not be cached
	_, err = client.GetQuote("mint1", "mint2", "2000", "taker")
	if err != nil {
		t.Fatalf("third call failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 server calls (different amount), got %d", callCount)
	}
}

func TestSwapClient_GetQuote_RequestParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("inputMint") != "inputMint1" {
			t.Errorf("unexpected inputMint: %s", q.Get("inputMint"))
		}
		if q.Get("outputMint") != "outputMint1" {
			t.Errorf("unexpected outputMint: %s", q.Get("outputMint"))
		}
		if q.Get("amount") != "5000" {
			t.Errorf("unexpected amount: %s", q.Get("amount"))
		}
		if q.Get("taker") != "myTaker" {
			t.Errorf("unexpected taker: %s", q.Get("taker"))
		}
		fmt.Fprint(w, jupiterUltraResponse)
	}))
	defer server.Close()

	client := swap.NewSwapClientWithHTTP(server.Client(), server.URL)
	_, err := client.GetQuote("inputMint1", "outputMint1", "5000", "myTaker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSwapQuote_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		fetchedAt time.Time
		want      bool
	}{
		{
			name:      "fresh quote",
			fetchedAt: time.Now(),
			want:      false,
		},
		{
			name:      "expired quote",
			fetchedAt: time.Now().Add(-2 * models.QuoteTTL),
			want:      true,
		},
		{
			name:      "just expired",
			fetchedAt: time.Now().Add(-91 * time.Second),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &models.SwapQuote{FetchedAt: tt.fetchedAt}
			if got := q.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSwapClient_GetQuote_MultiRouteStep(t *testing.T) {
	multiRouteResp := `{
		"requestId": "req-456",
		"inputMint": "mintA",
		"outputMint": "mintC",
		"inAmount": "1000000",
		"outAmount": "500000",
		"swapMode": "ExactIn",
		"slippageBps": 100,
		"priceImpactPct": "0.5",
		"routePlan": [
			{
				"swapInfo": {
					"ammKey": "amm1",
					"label": "Raydium",
					"inputMint": "mintA",
					"outputMint": "mintB",
					"inAmount": "1000000",
					"outAmount": "750000",
					"feeAmount": "1000",
					"feeMint": "mintA"
				},
				"percent": 100
			},
			{
				"swapInfo": {
					"ammKey": "amm2",
					"label": "Orca",
					"inputMint": "mintB",
					"outputMint": "mintC",
					"inAmount": "750000",
					"outAmount": "500000",
					"feeAmount": "500",
					"feeMint": "mintB"
				},
				"percent": 100
			}
		],
		"transaction": "AQAAAA==",
		"prioritizationFeeLamports": 10000
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, multiRouteResp)
	}))
	defer server.Close()

	client := swap.NewSwapClientWithHTTP(server.Client(), server.URL)
	quote, err := client.GetQuote("mintA", "mintC", "1000000", "taker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(quote.RoutePlan) != 2 {
		t.Fatalf("expected 2 route steps, got %d", len(quote.RoutePlan))
	}

	if quote.RoutePlan[0].DexLabel != "Raydium" {
		t.Errorf("step 0: expected Raydium, got %s", quote.RoutePlan[0].DexLabel)
	}
	if quote.RoutePlan[1].DexLabel != "Orca" {
		t.Errorf("step 1: expected Orca, got %s", quote.RoutePlan[1].DexLabel)
	}

	// Verify raw response is preserved
	var raw map[string]interface{}
	if err := json.Unmarshal(quote.RawResponse, &raw); err != nil {
		t.Fatalf("failed to unmarshal raw response: %v", err)
	}
	if raw["requestId"] != "req-456" {
		t.Errorf("expected requestId 'req-456' in raw response")
	}
}

func TestSwapClient_GetQuote_CacheIncludesTaker(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		fmt.Fprint(w, jupiterUltraResponse)
	}))
	defer server.Close()

	client := swap.NewSwapClientWithHTTP(server.Client(), server.URL)

	_, err := client.GetQuote("mint1", "mint2", "1000", "takerA")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	_, err = client.GetQuote("mint1", "mint2", "1000", "takerB")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 server calls (different takers), got %d", callCount)
	}

	_, err = client.GetQuote("mint1", "mint2", "1000", "takerA")
	if err != nil {
		t.Fatalf("third call failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 server calls (taker A cached), got %d", callCount)
	}
}

func TestSwapClient_GetQuote_InvalidAmounts(t *testing.T) {
	badResp := `{
		"requestId": "req-bad",
		"inputMint": "mint1",
		"outputMint": "mint2",
		"inAmount": "not-a-number",
		"outAmount": "150000000",
		"swapMode": "ExactIn",
		"slippageBps": 50,
		"priceImpactPct": "0.01",
		"routePlan": [],
		"transaction": "AQAAAA==",
		"prioritizationFeeLamports": 5000
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, badResp)
	}))
	defer server.Close()

	client := swap.NewSwapClientWithHTTP(server.Client(), server.URL)
	_, err := client.GetQuote("mint1", "mint2", "1000", "taker")
	if err == nil {
		t.Fatal("expected error for invalid inAmount")
	}
}
