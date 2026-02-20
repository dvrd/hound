package blockchain_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/models"
)

func setupOracleTest(t *testing.T) {
	t.Helper()
	blockchain.ResetSOLPriceCache()
	t.Cleanup(func() {
		blockchain.ResetSOLPriceCache()
		blockchain.SetOracleHTTPClient(http.DefaultClient)
	})
}

func TestGetSOLPriceCachedFromJupiter(t *testing.T) {
	setupOracleTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":{"%s":{"price":"145.32"}}}`, blockchain.SOLMint)
	}))
	defer server.Close()

	// Override the HTTP client to route to our test server
	transport := &rewriteTransport{
		base: http.DefaultTransport,
		rewrite: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = server.Listener.Addr().String()
		},
	}
	blockchain.SetOracleHTTPClient(&http.Client{Transport: transport})

	price, err := blockchain.GetSOLPriceCached()
	if err != nil {
		t.Fatalf("GetSOLPriceCached failed: %v", err)
	}
	if price != 145.32 {
		t.Errorf("expected price 145.32, got %f", price)
	}
}

func TestGetSOLPriceCachedReturnsCached(t *testing.T) {
	setupOracleTest(t)

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		fmt.Fprintf(w, `{"data":{"%s":{"price":"150.00"}}}`, blockchain.SOLMint)
	}))
	defer server.Close()

	transport := &rewriteTransport{
		base: http.DefaultTransport,
		rewrite: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = server.Listener.Addr().String()
		},
	}
	blockchain.SetOracleHTTPClient(&http.Client{Transport: transport})

	// First call fetches
	price1, err := blockchain.GetSOLPriceCached()
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call should use cache
	price2, err := blockchain.GetSOLPriceCached()
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if price1 != price2 {
		t.Errorf("cached price mismatch: %f vs %f", price1, price2)
	}

	if callCount.Load() != 1 {
		t.Errorf("expected 1 HTTP call (cached), got %d", callCount.Load())
	}
}

func TestGetSOLPriceCachedFallbackToCoinGecko(t *testing.T) {
	setupOracleTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Jupiter path fails, CoinGecko path succeeds
		if r.URL.Path == "/price/v3" || r.URL.Query().Get("ids") == blockchain.SOLMint {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// CoinGecko response
		fmt.Fprint(w, `{"solana":{"usd":142.50}}`)
	}))
	defer server.Close()

	transport := &rewriteTransport{
		base: http.DefaultTransport,
		rewrite: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = server.Listener.Addr().String()
		},
	}
	blockchain.SetOracleHTTPClient(&http.Client{Transport: transport})

	price, err := blockchain.GetSOLPriceCached()
	if err != nil {
		t.Fatalf("GetSOLPriceCached with CoinGecko fallback failed: %v", err)
	}
	if price != 142.50 {
		t.Errorf("expected price 142.50, got %f", price)
	}
}

func TestGetSOLPriceCachedAllSourcesFail(t *testing.T) {
	setupOracleTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	transport := &rewriteTransport{
		base: http.DefaultTransport,
		rewrite: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = server.Listener.Addr().String()
		},
	}
	blockchain.SetOracleHTTPClient(&http.Client{Transport: transport})

	_, err := blockchain.GetSOLPriceCached()
	if err == nil {
		t.Fatal("expected error when all sources fail")
	}
	if !errors.Is(err, models.ErrOracleConnectionFailed) {
		t.Errorf("expected ErrOracleConnectionFailed, got: %v", err)
	}
}

func TestGetSOLPriceCachedInvalidPrice(t *testing.T) {
	setupOracleTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a price outside valid range
		fmt.Fprintf(w, `{"data":{"%s":{"price":"0.50"}}}`, blockchain.SOLMint)
	}))
	defer server.Close()

	transport := &rewriteTransport{
		base: http.DefaultTransport,
		rewrite: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = server.Listener.Addr().String()
		},
	}
	blockchain.SetOracleHTTPClient(&http.Client{Transport: transport})

	_, err := blockchain.GetSOLPriceCached()
	if err == nil {
		t.Fatal("expected error for invalid price")
	}
	// Both Jupiter and CoinGecko return invalid prices, so all sources fail
	if !errors.Is(err, models.ErrOracleConnectionFailed) {
		t.Errorf("expected ErrOracleConnectionFailed, got: %v", err)
	}
}

// rewriteTransport rewrites request URLs for testing.
type rewriteTransport struct {
	base    http.RoundTripper
	rewrite func(req *http.Request)
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.rewrite(req)
	return t.base.RoundTrip(req)
}
