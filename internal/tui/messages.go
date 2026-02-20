package tui

import (
	"github.com/dvrd/hound/internal/models"
)

// NavigateMsg requests navigation to a different view.
type NavigateMsg struct {
	View string      // "wallet-list", "wallet-import", "wallet-status", "tokens", "history", "swap"
	Data interface{} // Optional data to pass to the view
}

// NavigateBackMsg requests going back to the previous view.
type NavigateBackMsg struct{}

// PortfolioRefreshedMsg is sent when a portfolio refresh completes.
type PortfolioRefreshedMsg struct {
	Portfolio models.PortfolioBalance
	Err       error
}

// WalletImportedMsg is sent when a wallet import completes.
type WalletImportedMsg struct {
	Address string
	Err     error
}

// ErrorMsg wraps an error for display.
type ErrorMsg struct {
	Err error
}

// StatusMsg displays a status message.
type StatusMsg struct {
	Message string
}
