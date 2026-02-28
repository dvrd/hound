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

// --- New tests: RouteLabel, route display, price impact ---

func TestRouteLabel_SingleHop(t *testing.T) {
	q := models.SwapQuote{
		RoutePlan: []models.RouteStep{
			{DexLabel: "Raydium"},
		},
	}
	got := q.RouteLabel("SOL", "USDC")
	want := "SOL → Raydium → USDC"
	if got != want {
		t.Errorf("RouteLabel single hop = %q, want %q", got, want)
	}
}

func TestRouteLabel_MultiHop(t *testing.T) {
	q := models.SwapQuote{
		RoutePlan: []models.RouteStep{
			{DexLabel: "Raydium"},
			{DexLabel: "Orca"},
		},
	}
	got := q.RouteLabel("SOL", "BONK")
	if !strings.Contains(got, "2 hops") {
		t.Errorf("RouteLabel multi-hop should contain '2 hops', got %q", got)
	}
	if !strings.Contains(got, "Raydium") {
		t.Errorf("RouteLabel multi-hop should contain 'Raydium', got %q", got)
	}
	if !strings.Contains(got, "Orca") {
		t.Errorf("RouteLabel multi-hop should contain 'Orca', got %q", got)
	}
}

func TestRouteLabel_NoRoute(t *testing.T) {
	q := models.SwapQuote{RoutePlan: nil}
	got := q.RouteLabel("SOL", "USDC")
	if !strings.Contains(got, "direct") {
		t.Errorf("RouteLabel with no route plan should contain 'direct', got %q", got)
	}
}

func TestReviewViewContainsRouteLabel(t *testing.T) {
	m := quotedModel()
	view := m.View()
	if !strings.Contains(view, "Route") {
		t.Error("review view should contain 'Route' line")
	}
	if !strings.Contains(view, "Raydium") {
		t.Error("review view should contain DEX name 'Raydium' in route")
	}
}

func TestResultViewContainsRouteLabel(t *testing.T) {
	m := newTestModel()
	quote := models.SwapQuote{
		InputMint:    "So11111111111111111111111111111111111111112",
		OutputMint:   "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		InAmount:     "1000000000",
		OutAmount:    "150000000",
		InputSymbol:  "SOL",
		OutputSymbol: "USDC",
		RoutePlan: []models.RouteStep{
			{DexLabel: "Orca"},
		},
		FetchedAt: time.Now(),
	}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	m = updated.(swapview.Model)

	result := models.SwapTransactionResult{
		Signature: "abc123",
		Status:    "finalized",
		Dex:       "Orca",
	}
	updated, _ = m.Update(swapview.SwapExecutedMsg{Result: result})
	m = updated.(swapview.Model)

	view := m.View()
	if !strings.Contains(view, "Route") {
		t.Error("result view should contain 'Route' line")
	}
}

func TestPriceImpact_ModerateWarning(t *testing.T) {
	m := newTestModel()
	quote := models.SwapQuote{
		InputMint:      "SOL",
		OutputMint:     "USDC",
		InAmount:       "1000000000",
		OutAmount:      "150000000",
		PriceImpactPct: 2.5,
		FetchedAt:      time.Now(),
	}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	model := updated.(swapview.Model)
	view := model.View()
	if !strings.Contains(view, "2.50%") {
		t.Error("review view should show moderate price impact percentage")
	}
	if strings.Contains(view, "HIGH IMPACT") {
		t.Error("review view should not show HIGH IMPACT for 2.5% impact")
	}
}

func TestPriceImpact_LowNoWarning(t *testing.T) {
	m := newTestModel()
	quote := models.SwapQuote{
		InputMint:      "SOL",
		OutputMint:     "USDC",
		InAmount:       "1000000000",
		OutAmount:      "150000000",
		PriceImpactPct: 0.1,
		FetchedAt:      time.Now(),
	}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	model := updated.(swapview.Model)
	view := model.View()
	if strings.Contains(view, "HIGH IMPACT") {
		t.Error("review view should not show HIGH IMPACT for 0.1% impact")
	}
}

func TestPhaseInput_EnterWithEmptyFields_ShowsError(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(swapview.Model)
	if model.GetPhase() != swapview.PhaseInput {
		t.Errorf("phase after empty enter = %d, want PhaseInput", model.GetPhase())
	}
	view := model.View()
	if !strings.Contains(view, "Error") {
		t.Error("should show error when fields are empty")
	}
}

func TestEscOnResultPhase_NavigatesBack(t *testing.T) {
	m := newDryRunModel()
	quote := models.SwapQuote{FetchedAt: time.Now()}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	m = updated.(swapview.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(swapview.Model)
	if m.GetPhase() != swapview.PhaseResult {
		t.Fatalf("expected PhaseResult, got %d", m.GetPhase())
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc on result phase should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("esc on result should return NavigateBackMsg, got %T", msg)
	}
}

func TestEscOnPasswordPhase_GoesBackToReview(t *testing.T) {
	m := newTestModel()
	quote := models.SwapQuote{
		InputMint:  "SOL",
		OutputMint: "USDC",
		InAmount:   "1000000000",
		OutAmount:  "150000000",
		FetchedAt:  time.Now(),
	}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	m = updated.(swapview.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(swapview.Model)
	if m.GetPhase() != swapview.PhasePassword {
		t.Fatalf("expected PhasePassword, got %d", m.GetPhase())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(swapview.Model)
	if m.GetPhase() != swapview.PhaseReview {
		t.Errorf("esc on password should go to PhaseReview, got %d", m.GetPhase())
	}
}

// --- New tests: slippage input field, min received, slippage in review ---

func TestInputPhaseView_ContainsSlippageField(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Slippage") {
		t.Error("input phase should contain 'Slippage' field label")
	}
}

func TestInputPhaseView_SlippageHintShowsPercent(t *testing.T) {
	// Default value is "50" bps → hint should show "0.50%"
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "0.50%") {
		t.Errorf("input phase with default 50 bps should show '0.50%%' hint, got view:\n%s", view)
	}
}

func TestInputPhaseView_SlippageZeroShowsAuto(t *testing.T) {
	// Directly set slippage to "0" and verify the "Jupiter auto" hint renders.
	m := newTestModel()
	m.SetSlippageValue("0")
	view := m.View()
	if !strings.Contains(view, "Jupiter auto") {
		t.Errorf("slippage value '0' should show 'Jupiter auto' hint, got view:\n%s", view)
	}
}

func TestReviewViewContainsSlippage(t *testing.T) {
	// quotedModel() has SlippageBps: 50 → review should show "Slippage:" and "0.50%"
	m := quotedModel()
	view := m.View()
	if !strings.Contains(view, "Slippage:") {
		t.Error("review view should contain 'Slippage:' line when SlippageBps > 0")
	}
	if !strings.Contains(view, "0.50%") {
		t.Errorf("review view should show '0.50%%' for 50 bps slippage, got view:\n%s", view)
	}
}

func TestReviewViewContainsMinReceived(t *testing.T) {
	m := newTestModel()
	quote := models.SwapQuote{
		InputMint:    "So11111111111111111111111111111111111111112",
		OutputMint:   "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		InAmount:     "1000000000",
		OutAmount:    "150000000",
		Rate:         150.0,
		SlippageBps:  50,
		MinReceived:  149250000,
		InputSymbol:  "SOL",
		OutputSymbol: "USDC",
		RoutePlan:    []models.RouteStep{{DexLabel: "Raydium", Percent: 100}},
		NetworkFee:   0.000005,
		FetchedAt:    time.Now(),
	}
	updated, _ := m.Update(swapview.QuoteFetchedMsg{Quote: quote})
	m = updated.(swapview.Model)
	view := m.View()
	if !strings.Contains(view, "Min Received:") {
		t.Error("review view should contain 'Min Received:' line when MinReceived > 0")
	}
}

func TestInvalidSlippage_ShowsError(t *testing.T) {
	m := newTestModel()
	// Fill required fields so validation reaches slippage check
	for _, ch := range "So11111111111111111111111111111111111111112" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(swapview.Model)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(swapview.Model)
	for _, ch := range "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(swapview.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(swapview.Model)
	for _, ch := range "1000000000" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(swapview.Model)
	}
	// Tab to slippage field and set an out-of-range value
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(swapview.Model)
	// Clear default "50" first with backspace twice, then type 10001
	for i := 0; i < 5; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = updated.(swapview.Model)
	}
	for _, ch := range "10001" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(swapview.Model)
	}
	// Submit — should fail validation
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(swapview.Model)
	if m.GetPhase() != swapview.PhaseInput {
		t.Errorf("invalid slippage should keep PhaseInput, got %d", m.GetPhase())
	}
	view := m.View()
	if !strings.Contains(view, "Error") {
		t.Error("invalid slippage should show an error message")
	}
}

// ---------------------------------------------------------------------------
// Tab cycling: focus advances through all 4 fields and wraps around
// ---------------------------------------------------------------------------

func TestTabCyclesFocus(t *testing.T) {
	tab := tea.KeyMsg{Type: tea.KeyTab}
	shiftTab := tea.KeyMsg{Type: tea.KeyShiftTab}

	m := newTestModel()
	if m.GetFocusIndex() != 0 {
		t.Fatalf("initial focus should be 0 (input mint), got %d", m.GetFocusIndex())
	}

	// Tab forward through all 4 fields.
	for want := 1; want <= 4; want++ {
		updated, _ := m.Update(tab)
		m = updated.(swapview.Model)
		if m.GetFocusIndex() != want%4 {
			t.Errorf("after %d tab(s): focus = %d, want %d", want, m.GetFocusIndex(), want%4)
		}
	}

	// shift+tab goes backward.
	updated, _ := m.Update(shiftTab)
	m = updated.(swapview.Model)
	if m.GetFocusIndex() != 3 {
		t.Errorf("after shift+tab from 0: focus = %d, want 3", m.GetFocusIndex())
	}
}

// TestTabInputsAreIndependent verifies that typing after a tab lands in the
// correct field — i.e., focus state is actually applied to the model, not
// just tracked in focusIndex.
func TestTabInputsAreIndependent(t *testing.T) {
	tab := tea.KeyMsg{Type: tea.KeyTab}
	typeKey := func(m swapview.Model, ch rune) swapview.Model {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		return updated.(swapview.Model)
	}

	m := newTestModel()
	m = typeKey(m, 'A') // field 0: input mint

	updated, _ := m.Update(tab)
	m = updated.(swapview.Model)
	m = typeKey(m, 'B') // field 1: output mint

	updated, _ = m.Update(tab)
	m = updated.(swapview.Model)
	m = typeKey(m, 'C') // field 2: amount

	view := m.View()
	if !strings.Contains(view, "A") {
		t.Error("input mint field should contain 'A'")
	}
	if !strings.Contains(view, "B") {
		t.Error("output mint field should contain 'B'")
	}
	if !strings.Contains(view, "C") {
		t.Error("amount field should contain 'C'")
	}
}

// ── Token-search overlay tests ────────────────────────────────────────────────

// TestTokenPick_SKeyOnInputMintField opens the overlay when focused on input mint.
func TestTokenPick_SKeyOnInputMintField(t *testing.T) {
	m := newTestModel() // focusIndex=0 by default
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m2 := updated.(swapview.Model)

	if !m2.IsPickingToken() {
		t.Fatal("pressing 's' on input mint field should open token-pick overlay")
	}
	view := m2.View()
	if !strings.Contains(view, "Pick") {
		t.Error("overlay view should contain 'Pick' header")
	}
}

// TestTokenPick_SKeyOnAmountField does NOT open overlay when focused on amount.
func TestTokenPick_SKeyOnAmountField(t *testing.T) {
	m := newTestModel()
	// Tab twice to reach amount field (index 2)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3, _ := m2.(swapview.Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	m4, _ := m3.(swapview.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	result := m4.(swapview.Model)

	if result.IsPickingToken() {
		t.Error("pressing 's' on amount field should NOT open token-pick overlay")
	}
}

// TestTokenPick_EscClosesOverlay verifies esc closes the overlay without changing mint.
func TestTokenPick_EscClosesOverlay(t *testing.T) {
	m := newTestModel()
	// Open overlay
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if !m2.(swapview.Model).IsPickingToken() {
		t.Fatal("overlay should be open")
	}
	// Close with esc
	m3, _ := m2.(swapview.Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := m3.(swapview.Model)

	if result.IsPickingToken() {
		t.Error("esc should close the token-pick overlay")
	}
	if result.GetPhase() != swapview.PhaseInput {
		t.Error("phase should remain PhaseInput after closing overlay")
	}
}

// TestTokenPick_InjectResults verifies TokenSearchResultMsg populates results.
func TestTokenPick_InjectResults(t *testing.T) {
	m := newTestModel()
	// Open overlay so searchInput is focused and has empty value (matching query "")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	// Inject a result whose Query matches current searchInput value ("")
	msg := swapview.TokenSearchResultMsg{
		Query: "",
		Results: []swapview.TokenSearchResult{
			{Symbol: "SOL", Name: "Wrapped SOL", Address: "So111111111111111111111111111111111111111112"},
			{Symbol: "USDC", Name: "USD Coin", Address: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"},
		},
	}
	m3, _ := m2.(swapview.Model).Update(msg)
	result := m3.(swapview.Model)

	if result.SearchResultCount() != 2 {
		t.Errorf("expected 2 results, got %d", result.SearchResultCount())
	}
	view := result.View()
	if !strings.Contains(view, "SOL") {
		t.Error("overlay should display SOL result")
	}
	if !strings.Contains(view, "USDC") {
		t.Error("overlay should display USDC result")
	}
}

// TestTokenPick_JKNavigation verifies j/k move the cursor through results.
func TestTokenPick_JKNavigation(t *testing.T) {
	m := newTestModel()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	msg := swapview.TokenSearchResultMsg{
		Query: "",
		Results: []swapview.TokenSearchResult{
			{Symbol: "SOL", Name: "Wrapped SOL", Address: "addr1"},
			{Symbol: "USDC", Name: "USD Coin", Address: "addr2"},
			{Symbol: "BONK", Name: "Bonk", Address: "addr3"},
		},
	}
	m3, _ := m2.(swapview.Model).Update(msg)

	// Initial cursor = 0
	if m3.(swapview.Model).SearchCursor() != 0 {
		t.Errorf("initial cursor should be 0, got %d", m3.(swapview.Model).SearchCursor())
	}

	// Press j → cursor = 1
	m4, _ := m3.(swapview.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m4.(swapview.Model).SearchCursor() != 1 {
		t.Errorf("after j cursor should be 1, got %d", m4.(swapview.Model).SearchCursor())
	}

	// Press j again → cursor = 2
	m5, _ := m4.(swapview.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m5.(swapview.Model).SearchCursor() != 2 {
		t.Errorf("after jj cursor should be 2, got %d", m5.(swapview.Model).SearchCursor())
	}

	// Press j again — should not exceed last index
	m6, _ := m5.(swapview.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m6.(swapview.Model).SearchCursor() != 2 {
		t.Errorf("cursor should clamp at 2, got %d", m6.(swapview.Model).SearchCursor())
	}

	// Press k → cursor = 1
	m7, _ := m6.(swapview.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m7.(swapview.Model).SearchCursor() != 1 {
		t.Errorf("after k cursor should be 1, got %d", m7.(swapview.Model).SearchCursor())
	}
}

// TestTokenPick_EnterSelectsInputToken verifies enter writes the mint address to inputMint.
func TestTokenPick_EnterSelectsInputToken(t *testing.T) {
	m := newTestModel() // focusIndex=0 → pickingInput
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	msg := swapview.TokenSearchResultMsg{
		Query: "",
		Results: []swapview.TokenSearchResult{
			{Symbol: "SOL", Name: "Wrapped SOL", Address: "So111111111111111111111111111111111111111112"},
		},
	}
	m3, _ := m2.(swapview.Model).Update(msg)
	m4, _ := m3.(swapview.Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := m4.(swapview.Model)

	if result.IsPickingToken() {
		t.Error("overlay should close after enter")
	}
	if result.InputMintValue() != "So111111111111111111111111111111111111111112" {
		t.Errorf("inputMint should be set to SOL address, got %q", result.InputMintValue())
	}
	if result.InputSymbol() != "SOL" {
		t.Errorf("inputSymbol should be 'SOL', got %q", result.InputSymbol())
	}
}

// TestTokenPick_EnterSelectsOutputToken verifies enter writes to outputMint when overlay opened from index 1.
func TestTokenPick_EnterSelectsOutputToken(t *testing.T) {
	m := newTestModel()
	// Tab to outputMint (focusIndex=1)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3, _ := m2.(swapview.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if !m3.(swapview.Model).IsPickingToken() {
		t.Fatal("overlay should open on outputMint field")
	}
	msg := swapview.TokenSearchResultMsg{
		Query: "",
		Results: []swapview.TokenSearchResult{
			{Symbol: "USDC", Name: "USD Coin", Address: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"},
		},
	}
	m4, _ := m3.(swapview.Model).Update(msg)
	m5, _ := m4.(swapview.Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := m5.(swapview.Model)

	if result.IsPickingToken() {
		t.Error("overlay should close after enter")
	}
	if result.OutputMintValue() != "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" {
		t.Errorf("outputMint should be set to USDC address, got %q", result.OutputMintValue())
	}
	if result.OutputSymbol() != "USDC" {
		t.Errorf("outputSymbol should be 'USDC', got %q", result.OutputSymbol())
	}
}

// TestTokenPick_StaleResultIgnored verifies results with mismatched query are dropped.
func TestTokenPick_StaleResultIgnored(t *testing.T) {
	m := newTestModel()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	// Inject result with query "sol" but searchInput is still ""
	stale := swapview.TokenSearchResultMsg{
		Query: "sol",
		Results: []swapview.TokenSearchResult{
			{Symbol: "SOL", Name: "Wrapped SOL", Address: "addr1"},
		},
	}
	m3, _ := m2.(swapview.Model).Update(stale)
	result := m3.(swapview.Model)

	if result.SearchResultCount() != 0 {
		t.Errorf("stale result should be ignored, got %d results", result.SearchResultCount())
	}
}

// TestTokenPick_ViewShowsSearchHint verifies PhaseInput view shows 's' hint.
func TestTokenPick_ViewShowsSearchHint(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "press s to search") {
		t.Error("PhaseInput view should show 's to search' hint on mint fields")
	}
}

// TestTokenPick_FooterChangesInOverlay verifies footer shows overlay keys when picking.
func TestTokenPick_FooterChangesInOverlay(t *testing.T) {
	m := newTestModel()
	footerNormal := m.Footer()
	if !strings.Contains(footerNormal, "search token") {
		t.Error("normal footer should contain 'search token' hint")
	}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	footerOverlay := m2.(swapview.Model).Footer()
	if !strings.Contains(footerOverlay, "navigate") {
		t.Error("overlay footer should contain 'navigate' hint")
	}
	if strings.Contains(footerOverlay, "search token") {
		t.Error("overlay footer should NOT contain 'search token' hint")
	}
}
