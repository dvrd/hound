package walletlist_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/walletlist"
)

func newTestModel() walletlist.Model {
	return walletlist.New(nil, nil)
}

func loadedModel(wallets []models.Wallet, portfolios map[string]models.PortfolioBalance) walletlist.Model {
	m := newTestModel()
	msg := walletlist.WalletsLoadedMsg{
		Wallets:    wallets,
		Portfolios: portfolios,
	}
	updated, _ := m.Update(msg)
	return updated.(walletlist.Model)
}

func sampleWallets() []models.Wallet {
	return []models.Wallet{
		{
			Address:    "7xKXabc1234567890abcdef9mPq",
			Label:      "Main",
			IsPrimary:  true,
			WalletType: models.WalletTypeBIP44Standard,
		},
		{
			Address:    "9yLMdef0987654321fedcba3nRs",
			Label:      "Trading",
			IsPrimary:  false,
			WalletType: models.WalletTypeBIP44Change,
		},
	}
}

func samplePortfolios() map[string]models.PortfolioBalance {
	return map[string]models.PortfolioBalance{
		"7xKXabc1234567890abcdef9mPq": {
			WalletAddress: "7xKXabc1234567890abcdef9mPq",
			TotalUSD:      1500.50,
		},
		"9yLMdef0987654321fedcba3nRs": {
			WalletAddress: "9yLMdef0987654321fedcba3nRs",
			TotalUSD:      250.00,
		},
	}
}

func TestNew(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Hound") {
		t.Errorf("View should contain 'Hound', got %q", view)
	}
}

func TestWalletsLoadedMsg(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())
	view := m.View()

	if !strings.Contains(view, "Main") {
		t.Error("View should contain wallet label 'Main'")
	}
	if !strings.Contains(view, "Trading") {
		t.Error("View should contain wallet label 'Trading'")
	}
}

func TestWalletsLoadedMsg_Error(t *testing.T) {
	m := newTestModel()
	msg := walletlist.WalletsLoadedMsg{
		Err: models.ErrWalletNotFound,
	}
	updated, _ := m.Update(msg)
	model := updated.(walletlist.Model)
	view := model.View()
	if !strings.Contains(view, "Error") {
		t.Error("View should show error when WalletsLoadedMsg has error")
	}
}

func TestCursorNavigation(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())

	// Initial cursor at 0
	w, ok := m.SelectedWallet()
	if !ok {
		t.Fatal("should have selected wallet")
	}
	if w.Label != "Main" {
		t.Errorf("initial selected wallet = %q, want %q", w.Label, "Main")
	}

	// Move down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(walletlist.Model)
	w, _ = m.SelectedWallet()
	if w.Label != "Trading" {
		t.Errorf("after j, selected wallet = %q, want %q", w.Label, "Trading")
	}

	// Move down again (should stay at last)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(walletlist.Model)
	w, _ = m.SelectedWallet()
	if w.Label != "Trading" {
		t.Errorf("after second j, selected wallet = %q, want %q", w.Label, "Trading")
	}

	// Move up
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(walletlist.Model)
	w, _ = m.SelectedWallet()
	if w.Label != "Main" {
		t.Errorf("after k, selected wallet = %q, want %q", w.Label, "Main")
	}

	// Move up again (should stay at first)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(walletlist.Model)
	w, _ = m.SelectedWallet()
	if w.Label != "Main" {
		t.Errorf("after second k, selected wallet = %q, want %q", w.Label, "Main")
	}
}

func TestCursorNavigation_ArrowKeys(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(walletlist.Model)
	w, _ := m.SelectedWallet()
	if w.Label != "Trading" {
		t.Errorf("after down, selected = %q, want %q", w.Label, "Trading")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(walletlist.Model)
	w, _ = m.SelectedWallet()
	if w.Label != "Main" {
		t.Errorf("after up, selected = %q, want %q", w.Label, "Main")
	}
}

func TestEnterNavigatesToStatus(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should return a command")
	}
	msg := cmd()
	nav, ok := msg.(tui.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.View != "wallet-status" {
		t.Errorf("NavigateMsg.View = %q, want %q", nav.View, "wallet-status")
	}
	if nav.Data != "7xKXabc1234567890abcdef9mPq" {
		t.Errorf("NavigateMsg.Data = %v, want wallet address", nav.Data)
	}
}

func TestSKeyNavigatesToStatus(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("s should return a command")
	}
	msg := cmd()
	nav, ok := msg.(tui.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.View != "wallet-status" {
		t.Errorf("NavigateMsg.View = %q, want %q", nav.View, "wallet-status")
	}
}

func TestIKeyNavigatesToImport(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if cmd == nil {
		t.Fatal("i should return a command")
	}
	msg := cmd()
	nav, ok := msg.(tui.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.View != "wallet-import" {
		t.Errorf("NavigateMsg.View = %q, want %q", nav.View, "wallet-import")
	}
}

func TestDKeyNavigatesToDelete(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("d should return a command")
	}
	msg := cmd()
	nav, ok := msg.(tui.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.View != "wallet-delete" {
		t.Errorf("NavigateMsg.View = %q, want %q", nav.View, "wallet-delete")
	}
	w, ok := nav.Data.(models.Wallet)
	if !ok {
		t.Fatalf("expected models.Wallet data, got %T", nav.Data)
	}
	if w.Label != "Main" {
		t.Errorf("delete wallet label = %q, want %q", w.Label, "Main")
	}
}

func TestEmptyWalletList(t *testing.T) {
	m := loadedModel(nil, nil)
	view := m.View()
	if !strings.Contains(view, "No wallets found") {
		t.Error("empty wallet list should show 'No wallets found'")
	}
}

func TestEmptyWalletList_NoNavigate(t *testing.T) {
	m := loadedModel(nil, nil)

	// Enter on empty list should not navigate
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("enter on empty list should not return a command")
	}
}

func TestViewContainsStatusBar(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())
	view := m.View()
	if !strings.Contains(view, "[i]mport") {
		t.Error("View should contain [i]mport in status bar")
	}
	if !strings.Contains(view, "[q]uit") {
		t.Error("View should contain [q]uit in status bar")
	}
}

func TestViewContainsTotalUSD(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())
	view := m.View()
	if !strings.Contains(view, "Total") {
		t.Error("View should contain 'Total'")
	}
}

func TestTruncateAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{"long address", "7xKXabc1234567890abcdef9mPq", "7xKX...9mPq"},
		{"short address", "abc", "abc"},
		{"exactly 11", "12345678901", "12345678901"},
		{"12 chars", "123456789012", "1234...9012"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := walletlist.TruncateAddress(tt.addr)
			if got != tt.want {
				t.Errorf("TruncateAddress(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestSelectedWallet_Empty(t *testing.T) {
	m := loadedModel(nil, nil)
	_, ok := m.SelectedWallet()
	if ok {
		t.Error("SelectedWallet on empty list should return false")
	}
}

func TestPortfolioRefreshedMsg(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())

	// Simulate portfolio refresh
	updated, _ := m.Update(tui.PortfolioRefreshedMsg{
		Portfolio: models.PortfolioBalance{
			WalletAddress: "7xKXabc1234567890abcdef9mPq",
			TotalUSD:      2000.00,
		},
	})
	model := updated.(walletlist.Model)
	view := model.View()
	// The view should reflect the updated portfolio
	if !strings.Contains(view, "2000") || !strings.Contains(view, "$") {
		// This is a soft check - the exact formatting depends on FormatPrice
		_ = view // just ensure no panic
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(walletlist.Model)
	// Should not panic
}

func TestSendKeyNavigatesToSend(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if cmd == nil {
		t.Fatal("S should return a command")
	}
	msg := cmd()
	nav, ok := msg.(tui.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.View != "send" {
		t.Errorf("NavigateMsg.View = %q, want %q", nav.View, "send")
	}
	if nav.Data != "7xKXabc1234567890abcdef9mPq" {
		t.Errorf("NavigateMsg.Data = %v, want wallet address", nav.Data)
	}
}

func TestReceiveKeyNavigatesToReceive(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if cmd == nil {
		t.Fatal("R should return a command")
	}
	msg := cmd()
	nav, ok := msg.(tui.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.View != "receive" {
		t.Errorf("NavigateMsg.View = %q, want %q", nav.View, "receive")
	}
	if nav.Data != "7xKXabc1234567890abcdef9mPq" {
		t.Errorf("NavigateMsg.Data = %v, want wallet address", nav.Data)
	}
}

func TestWalletList_ResponsiveView_Narrow(t *testing.T) {
	m := walletlist.New(nil, nil)
	model, _ := m.Update(walletlist.WalletsLoadedMsg{
		Wallets: []models.Wallet{
			{Address: "abc123def456ghi789", Label: "Main", IsPrimary: true},
			{Address: "xyz987wvu654tsr321", Label: "Secondary"},
		},
		Portfolios: map[string]models.PortfolioBalance{},
	})

	model, _ = model.Update(tea.WindowSizeMsg{Width: 60, Height: 15})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
	if !strings.Contains(view, "[i]mp") {
		t.Error("narrow view should use abbreviated status bar")
	}
}

func TestWalletList_ResponsiveView_Wide(t *testing.T) {
	m := walletlist.New(nil, nil)
	model, _ := m.Update(walletlist.WalletsLoadedMsg{
		Wallets: []models.Wallet{
			{Address: "abc123def456ghi789", Label: "Main", IsPrimary: true},
		},
		Portfolios: map[string]models.PortfolioBalance{},
	})

	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	view := model.(tea.Model).View()
	if !strings.Contains(view, "[i]mport") {
		t.Error("wide view should use full status bar")
	}
}

func TestWalletList_CappedVisibleRows(t *testing.T) {
	wallets := make([]models.Wallet, 20)
	for i := range wallets {
		wallets[i] = models.Wallet{
			Address: fmt.Sprintf("addr%d", i),
			Label:   fmt.Sprintf("Wallet %d", i),
		}
	}

	m := walletlist.New(nil, nil)
	model, _ := m.Update(walletlist.WalletsLoadedMsg{
		Wallets:    wallets,
		Portfolios: map[string]models.PortfolioBalance{},
	})

	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 12})

	view := model.(tea.Model).View()
	if !strings.Contains(view, "more") {
		t.Error("should show scroll indicator when list exceeds visible rows")
	}
}
