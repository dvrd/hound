package swapview_test

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/swapview"
)

func newTestModel() swapview.Model {
	return swapview.New("7xKXabc123", nil, nil, false)
}

func newDryRunModel() swapview.Model {
	return swapview.New("7xKXabc123", nil, nil, true)
}

func quotedModel() swapview.Model {
	m := newTestModel()
	quote := models.SwapQuote{
		InputMint:      "So11111111111111111111111111111111111111112",
		OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		InAmount:       "1000000000",
		OutAmount:      "150000000",
		Rate:           150.0,
		SlippageBps:    50,
		PriceImpactPct: 0.5,
		InputSymbol:    "SOL",
		OutputSymbol:   "USDC",
		RoutePlan: []models.RouteStep{
			{DexLabel: "Raydium", Percent: 100},
		},
		NetworkFee: 0.000005,
		FetchedAt:  time.Now(),
	}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	return updated.(swapview.Model)
}

func TestNew(t *testing.T) {
	m := newTestModel()
	if m.GetPhase() != swapview.PhaseInput {
		t.Errorf("initial phase = %d, want PhaseInput (0)", m.GetPhase())
	}
}

func TestNewDryRun(t *testing.T) {
	m := newDryRunModel()
	if !m.IsDryRun() {
		t.Error("dry run model should have dryRun = true")
	}
}

func TestViewContainsTitle(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Swap Tokens") {
		t.Errorf("View should contain 'Swap Tokens', got %q", view)
	}
}

func TestViewContainsPhaseIndicator(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Input") {
		t.Error("View should contain phase name 'Input'")
	}
}

func TestPhaseString(t *testing.T) {
	tests := []struct {
		phase swapview.SwapPhase
		want  string
	}{
		{swapview.PhaseInput, "Input"},
		{swapview.PhaseQuoting, "Getting Quote"},
		{swapview.PhaseReview, "Review Quote"},
		{swapview.PhasePassword, "Password"},
		{swapview.PhaseExecuting, "Executing"},
		{swapview.PhaseResult, "Result"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.phase.String()
			if got != tt.want {
				t.Errorf("SwapPhase(%d).String() = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}

func TestEscOnInputPhase_NavigatesBack(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc on input phase should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("esc on input phase should return NavigateBackMsg, got %T", msg)
	}
}

func TestEscOnReviewPhase_GoesBackToInput(t *testing.T) {
	m := quotedModel()
	if m.GetPhase() != swapview.PhaseReview {
		t.Fatalf("expected PhaseReview, got %d", m.GetPhase())
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model := updated.(swapview.Model)
	if model.GetPhase() != swapview.PhaseInput {
		t.Errorf("esc on review should go to PhaseInput, got %d", model.GetPhase())
	}
}

func TestQuoteFetchedMsg_Success(t *testing.T) {
	m := newTestModel()
	quote := models.SwapQuote{
		InputMint:  "SOL",
		OutputMint: "USDC",
		InAmount:   "1000000000",
		OutAmount:  "150000000",
		Rate:       150.0,
		FetchedAt:  time.Now(),
	}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	model := updated.(swapview.Model)
	if model.GetPhase() != swapview.PhaseReview {
		t.Errorf("phase after quote = %d, want PhaseReview", model.GetPhase())
	}
}

func TestQuoteFetchedMsg_Error(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Err: models.ErrConnectionFailed})
	model := updated.(swapview.Model)
	if model.GetPhase() != swapview.PhaseInput {
		t.Errorf("phase after quote error = %d, want PhaseInput", model.GetPhase())
	}
}

func TestReviewViewContainsQuoteDetails(t *testing.T) {
	m := quotedModel()
	view := m.View()
	if !strings.Contains(view, "Swap Quote") {
		t.Error("review view should contain 'Swap Quote'")
	}
	if !strings.Contains(view, "SOL") {
		t.Error("review view should contain input symbol 'SOL'")
	}
	if !strings.Contains(view, "USDC") {
		t.Error("review view should contain output symbol 'USDC'")
	}
	if !strings.Contains(view, "Raydium") {
		t.Error("review view should contain route DEX 'Raydium'")
	}
}

func TestReviewViewHighPriceImpact(t *testing.T) {
	m := newTestModel()
	quote := models.SwapQuote{
		InputMint:      "SOL",
		OutputMint:     "USDC",
		InAmount:       "1000000000",
		OutAmount:      "150000000",
		PriceImpactPct: 6.5,
		FetchedAt:      time.Now(),
	}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	model := updated.(swapview.Model)
	view := model.View()
	if !strings.Contains(view, "HIGH IMPACT") {
		t.Error("review view should show HIGH IMPACT warning for >5% price impact")
	}
}

func TestDryRunReview(t *testing.T) {
	m := newDryRunModel()
	quote := models.SwapQuote{
		InputMint:  "SOL",
		OutputMint: "USDC",
		InAmount:   "1000000000",
		OutAmount:  "150000000",
		FetchedAt:  time.Now(),
	}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	model := updated.(swapview.Model)
	view := model.View()
	if !strings.Contains(view, "DRY RUN") {
		t.Error("dry run review should show DRY RUN warning")
	}
}

func TestDryRunEnterSkipsPassword(t *testing.T) {
	m := newDryRunModel()
	quote := models.SwapQuote{
		InputMint:  "SOL",
		OutputMint: "USDC",
		InAmount:   "1000000000",
		OutAmount:  "150000000",
		FetchedAt:  time.Now(),
	}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	model := updated.(swapview.Model)

	// Press enter on review in dry-run mode
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(swapview.Model)
	if model.GetPhase() != swapview.PhaseResult {
		t.Errorf("dry run enter on review should go to PhaseResult, got %d", model.GetPhase())
	}
	if model.GetResult().Status != "dry-run" {
		t.Errorf("dry run result status = %q, want %q", model.GetResult().Status, "dry-run")
	}
}

func TestDryRunResultView(t *testing.T) {
	m := newDryRunModel()
	quote := models.SwapQuote{
		InputMint:  "SOL",
		OutputMint: "USDC",
		InAmount:   "1000000000",
		OutAmount:  "150000000",
		FetchedAt:  time.Now(),
	}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	model := updated.(swapview.Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(swapview.Model)
	view := model.View()
	if !strings.Contains(view, "Dry Run Complete") {
		t.Error("dry run result should show 'Dry Run Complete'")
	}
}

func TestSwapExecutedMsg_Success(t *testing.T) {
	m := newTestModel()
	result := models.SwapTransactionResult{
		Signature: "5abc123def456",
		Status:    "finalized",
		Dex:       "Raydium",
	}
	updated, _ := m.Update(swapview.SwapExecutedMsg{Result: result})
	model := updated.(swapview.Model)
	if model.GetPhase() != swapview.PhaseResult {
		t.Errorf("phase after swap = %d, want PhaseResult", model.GetPhase())
	}
	view := model.View()
	if !strings.Contains(view, "Successful") {
		t.Error("success result should contain 'Successful'")
	}
	if !strings.Contains(view, "5abc123def456") {
		t.Error("success result should contain signature")
	}
}

func TestSwapExecutedMsg_Error(t *testing.T) {
	m := newTestModel()
	result := models.SwapTransactionResult{
		Status:       "failed",
		ErrorMessage: "insufficient funds",
	}
	updated, _ := m.Update(swapview.SwapExecutedMsg{
		Result: result,
		Err:    models.ErrInsufficientBalance,
	})
	model := updated.(swapview.Model)
	if model.GetPhase() != swapview.PhaseResult {
		t.Errorf("phase after swap error = %d, want PhaseResult", model.GetPhase())
	}
	view := model.View()
	if !strings.Contains(view, "Failed") {
		t.Error("error result should contain 'Failed'")
	}
}

func TestResultPhase_AnyKeyNavigatesBack(t *testing.T) {
	m := newDryRunModel()
	quote := models.SwapQuote{FetchedAt: time.Now()}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	model := updated.(swapview.Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(swapview.Model)

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd == nil {
		t.Fatal("any key on result phase should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("any key on result should return NavigateBackMsg, got %T", msg)
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(swapview.Model)
	// Should not panic
}

func TestInputPhaseView(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Input Token") {
		t.Error("input phase should contain 'Input Token'")
	}
	if !strings.Contains(view, "Output Token") {
		t.Error("input phase should contain 'Output Token'")
	}
	if !strings.Contains(view, "Amount") {
		t.Error("input phase should contain 'Amount'")
	}
}

func TestSwap_ResponsiveInputWidths(t *testing.T) {
	m := swapview.New("addr123", nil, nil, false)

	model, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
}
