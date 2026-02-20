package tui

import (
	"encoding/json"
	"os"

	"github.com/dvrd/hound/internal/models"
)

// WalletJSON is the JSON representation of a wallet for non-interactive output.
type WalletJSON struct {
	Address        string  `json:"address"`
	Label          string  `json:"label"`
	IsPrimary      bool    `json:"is_primary"`
	WalletType     string  `json:"wallet_type"`
	DerivationPath string  `json:"derivation_path"`
	TotalUSD       float64 `json:"total_usd,omitempty"`
}

// PortfolioJSON is the JSON representation of a portfolio for non-interactive output.
type PortfolioJSON struct {
	WalletAddress string      `json:"wallet_address"`
	TotalUSD      float64     `json:"total_usd"`
	SOL           TokenJSON   `json:"sol"`
	Tokens        []TokenJSON `json:"tokens"`
}

// TokenJSON is the JSON representation of a token balance for non-interactive output.
type TokenJSON struct {
	Mint      string  `json:"mint"`
	Symbol    string  `json:"symbol"`
	Amount    float64 `json:"amount"`
	USDPrice  float64 `json:"usd_price"`
	USDValue  float64 `json:"usd_value"`
	Change24h float64 `json:"change_24h"`
}

// TokenListJSON is the JSON representation of a tracked token for non-interactive output.
type TokenListJSON struct {
	Symbol          string  `json:"symbol"`
	Name            string  `json:"name"`
	ContractAddress string  `json:"contract_address"`
	PoolCount       int     `json:"pool_count"`
	USDPrice        float64 `json:"usd_price,omitempty"`
}

// SwapHistoryJSON is the JSON representation of a swap history entry for non-interactive output.
type SwapHistoryJSON struct {
	ID            int64   `json:"id"`
	WalletAddress string  `json:"wallet_address"`
	InputSymbol   string  `json:"input_symbol"`
	OutputSymbol  string  `json:"output_symbol"`
	InputAmount   float64 `json:"input_amount"`
	OutputAmount  float64 `json:"output_amount"`
	PriceImpact   float64 `json:"price_impact"`
	Status        string  `json:"status"`
	Dex           string  `json:"dex,omitempty"`
	Signature     string  `json:"signature,omitempty"`
	CreatedAt     int64   `json:"created_at"`
}

// WalletToJSON converts a Wallet model to its JSON representation.
func WalletToJSON(w models.Wallet, totalUSD float64) WalletJSON {
	return WalletJSON{
		Address:        w.Address,
		Label:          w.Label,
		IsPrimary:      w.IsPrimary,
		WalletType:     w.WalletType.String(),
		DerivationPath: w.DerivationPath,
		TotalUSD:       totalUSD,
	}
}

// PortfolioToJSON converts a PortfolioBalance model to its JSON representation.
func PortfolioToJSON(p models.PortfolioBalance) PortfolioJSON {
	tokens := make([]TokenJSON, 0, len(p.TokenBalances))
	for _, tb := range p.TokenBalances {
		tokens = append(tokens, TokenBalanceToJSON(tb))
	}

	return PortfolioJSON{
		WalletAddress: p.WalletAddress,
		TotalUSD:      p.TotalUSD,
		SOL:           TokenBalanceToJSON(p.SOLBalance),
		Tokens:        tokens,
	}
}

// TokenBalanceToJSON converts a TokenBalance model to its JSON representation.
func TokenBalanceToJSON(tb models.TokenBalance) TokenJSON {
	return TokenJSON{
		Mint:      tb.Mint,
		Symbol:    tb.Symbol,
		Amount:    tb.Amount,
		USDPrice:  tb.USDPrice,
		USDValue:  tb.USDValue,
		Change24h: tb.Change24h,
	}
}

// TokenToJSON converts a Token model to its JSON representation.
func TokenToJSON(t models.Token, poolCount int) TokenListJSON {
	return TokenListJSON{
		Symbol:          t.Symbol,
		Name:            t.Name,
		ContractAddress: t.ContractAddress,
		PoolCount:       poolCount,
		USDPrice:        t.USDPrice,
	}
}

// SwapHistoryToJSON converts a SwapHistoryEntry model to its JSON representation.
func SwapHistoryToJSON(e models.SwapHistoryEntry) SwapHistoryJSON {
	return SwapHistoryJSON{
		ID:            e.ID,
		WalletAddress: e.WalletAddress,
		InputSymbol:   e.InputSymbol,
		OutputSymbol:  e.OutputSymbol,
		InputAmount:   e.InputAmount,
		OutputAmount:  e.OutputAmount,
		PriceImpact:   e.PriceImpact,
		Status:        e.Status,
		Dex:           e.Dex,
		Signature:     e.Signature,
		CreatedAt:     e.CreatedAt,
	}
}

// PrintJSON writes any value as formatted JSON to stdout.
func PrintJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// PrintError writes an error as JSON to stderr.
func PrintError(err error) {
	json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck
}
