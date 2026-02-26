package send_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/send"
)

func testPortfolio() models.PortfolioBalance {
	return models.PortfolioBalance{
		WalletAddress: "SenderAddr111111111111111111111111111111111",
		SOLBalance: models.TokenBalance{
			Mint:     "So11111111111111111111111111111111111111112",
			Symbol:   "SOL",
			Amount:   5.0,
			Decimals: 9,
			USDPrice: 150.0,
			USDValue: 750.0,
		},
		TokenBalances: []models.TokenBalance{
			{
				Mint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				Symbol:   "USDC",
				Amount:   100.0,
				Decimals: 6,
				USDPrice: 1.0,
				USDValue: 100.0,
			},
			{
				Mint:     "ZeroMint11111111111111111111111111111111111",
				Symbol:   "ZERO",
				Amount:   0,
				Decimals: 6,
			},
			{
				Mint:     "DustMint11111111111111111111111111111111111",
				Symbol:   "DUST",
				Amount:   1000.0,
				Decimals: 6,
				USDPrice: 0.0005,
				USDValue: 0.50, // below $1 threshold — must be filtered out
			},
		},
		TotalUSD: 850.0,
	}
}

func newTestModel() send.Model {
	return send.New(
		"SenderAddr111111111111111111111111111111111",
		nil, // no transfer service in tests
		nil, // no rpc client in tests
		testPortfolio(),
	)
}

func TestNew(t *testing.T) {
	m := newTestModel()
	if m.CurrentStep() != send.StepSelectToken {
		t.Errorf("initial step = %d, want StepSelectToken (0)", m.CurrentStep())
	}
}

func TestDustTokensFilteredFromTokenList(t *testing.T) {
	m := newTestModel()
	view := m.View()

	// SOL ($750) and USDC ($100) must appear
	if !strings.Contains(view, "SOL") {
		t.Error("SOL should appear in token list")
	}
	if !strings.Contains(view, "USDC") {
		t.Error("USDC ($100) should appear in token list")
	}

	// DUST ($0.50) has balance but USD value < $1 — must be hidden
	if strings.Contains(view, "DUST") {
		t.Error("DUST ($0.50) should be hidden from token list (below $1 threshold)")
	}

	// ZERO (zero balance) must also be hidden
	if strings.Contains(view, "ZERO") {
		t.Error("ZERO (zero balance) should be hidden from token list")
	}
}

func TestSelectToken(t *testing.T) {
	m := newTestModel()

	// Initial cursor at 0
	if m.TokenCursor() != 0 {
		t.Errorf("initial cursor = %d, want 0", m.TokenCursor())
	}

	// Move down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(send.Model)
	if m.TokenCursor() != 1 {
		t.Errorf("cursor after j = %d, want 1", m.TokenCursor())
	}

	// Move up
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(send.Model)
	if m.TokenCursor() != 0 {
		t.Errorf("cursor after k = %d, want 0", m.TokenCursor())
	}

	// Can't go above 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(send.Model)
	if m.TokenCursor() != 0 {
		t.Errorf("cursor should stay at 0, got %d", m.TokenCursor())
	}
}

func TestSelectTokenEnter(t *testing.T) {
	m := newTestModel()

	// Select SOL (cursor at 0)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepRecipient {
		t.Errorf("step after enter = %d, want StepRecipient", m.CurrentStep())
	}
}

func TestSelectTokenFiltersZeroBalance(t *testing.T) {
	m := newTestModel()
	view := m.View()
	// Should contain SOL and USDC but not ZERO
	if !strings.Contains(view, "SOL") {
		t.Error("view should contain SOL")
	}
	if !strings.Contains(view, "USDC") {
		t.Error("view should contain USDC")
	}
	if strings.Contains(view, "ZERO") {
		t.Error("view should not contain ZERO (zero balance)")
	}
}

func TestRecipientValidation(t *testing.T) {
	m := newTestModel()

	// Advance to recipient step
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Try empty address
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepRecipient {
		t.Error("should stay on recipient step with empty address")
	}
	view := m.View()
	if !strings.Contains(view, "Error") {
		t.Error("should show error for empty address")
	}
}

func TestRecipientSelfSend(t *testing.T) {
	m := newTestModel()

	// Advance to recipient step
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Type own address
	selfAddr := "SenderAddr111111111111111111111111111111111"
	for _, r := range selfAddr {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}

	// Try to advance
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepRecipient {
		t.Error("should stay on recipient step when sending to self")
	}
	view := m.View()
	if !strings.Contains(view, "own address") {
		t.Error("should show self-send error")
	}
}

func TestAmountValidation(t *testing.T) {
	m := newTestModel()

	// Advance to recipient step
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Type a valid recipient
	recipient := "RecipientAddr111111111111111111111111111111"
	for _, r := range recipient {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepAmount {
		t.Fatalf("should be at amount step, got %d", m.CurrentStep())
	}

	// Try empty amount
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepAmount {
		t.Error("should stay on amount step with empty amount")
	}
}

func TestAmountMAX(t *testing.T) {
	m := newTestModel()

	// Advance to amount step
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select token
	m = updated.(send.Model)

	recipient := "RecipientAddr111111111111111111111111111111"
	for _, r := range recipient {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm recipient
	m = updated.(send.Model)

	// Type MAX
	for _, r := range "MAX" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Should advance to review
	if m.CurrentStep() != send.StepReview {
		t.Errorf("step after MAX = %d, want StepReview", m.CurrentStep())
	}
}

func TestReviewDisplay(t *testing.T) {
	m := newTestModel()

	// Advance through steps to review
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select SOL
	m = updated.(send.Model)

	recipient := "RecipientAddr111111111111111111111111111111"
	for _, r := range recipient {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm recipient
	m = updated.(send.Model)

	// Type amount
	for _, r := range "1.5" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm amount
	m = updated.(send.Model)

	if m.CurrentStep() != send.StepReview {
		t.Fatalf("should be at review step, got %d", m.CurrentStep())
	}

	view := m.View()
	if !strings.Contains(view, "SOL") {
		t.Error("review should contain token symbol")
	}
	if !strings.Contains(view, "1.5") {
		t.Error("review should contain amount")
	}
	if !strings.Contains(view, "Reci") {
		t.Error("review should contain truncated recipient")
	}
	if !strings.Contains(view, "Fee") {
		t.Error("review should contain fee")
	}
}

func TestEscNavigatesBack(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc on first step should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("esc on first step should return NavigateBackMsg, got %T", msg)
	}
}

func TestEscGoesBackOneStep(t *testing.T) {
	m := newTestModel()

	// Advance to recipient step
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepRecipient {
		t.Fatalf("should be at recipient step, got %d", m.CurrentStep())
	}

	// Esc should go back to select token
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepSelectToken {
		t.Errorf("esc should go back to select token, got step %d", m.CurrentStep())
	}
}

func TestTransferSentMsg_Success(t *testing.T) {
	m := newTestModel()

	// Send TransferSentMsg with signature
	updated, _ := m.Update(tui.TransferSentMsg{Signature: "5abc123def456"})
	m = updated.(send.Model)

	if m.CurrentStep() != send.StepConfirming {
		t.Errorf("step after TransferSentMsg = %d, want StepConfirming", m.CurrentStep())
	}

	view := m.View()
	if !strings.Contains(view, "Confirming") {
		t.Error("confirming step should show confirming text")
	}
}

func TestTransferSentMsg_Error(t *testing.T) {
	m := newTestModel()

	// Send TransferSentMsg with error
	updated, _ := m.Update(tui.TransferSentMsg{Err: tui.ErrorMsg{Err: nil}.Err})
	// nil error means success path — goes to confirming
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepConfirming {
		t.Errorf("step = %d, want StepConfirming", m.CurrentStep())
	}
}

func TestTransferSentMsg_WithError(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(tui.TransferSentMsg{Err: fmt.Errorf("insufficient funds")})
	m = updated.(send.Model)

	if m.CurrentStep() != send.StepResult {
		t.Errorf("step = %d, want StepResult", m.CurrentStep())
	}

	view := m.View()
	if !strings.Contains(view, "Failed") {
		t.Error("result should indicate failure")
	}
	if !strings.Contains(view, "insufficient funds") {
		t.Error("result should contain error message")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(send.Model)
	// Should not panic
}

func TestStepName(t *testing.T) {
	tests := []struct {
		step send.Step
		want string
	}{
		{send.StepSelectToken, "Select Token"},
		{send.StepRecipient, "Recipient"},
		{send.StepAmount, "Amount"},
		{send.StepReview, "Review"},
		{send.StepPassword, "Password"},
		{send.StepSending, "Sending"},
		{send.StepConfirming, "Confirming"},
		{send.StepResult, "Result"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.step.Name()
			if got != tt.want {
				t.Errorf("Step(%d).Name() = %q, want %q", tt.step, got, tt.want)
			}
		})
	}
}

func TestResultStep_AnyKeyNavigatesBack(t *testing.T) {
	m := newTestModel()

	// Go to confirming step via TransferSentMsg
	updated, _ := m.Update(tui.TransferSentMsg{Signature: "abc"})
	m = updated.(send.Model)

	// Go to result step via TransferConfirmedMsg
	updated, _ = m.Update(tui.TransferConfirmedMsg{Signature: "abc", Confirmed: true})
	m = updated.(send.Model)

	// Any key should navigate back
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd == nil {
		t.Fatal("any key on result step should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("any key on result should return NavigateBackMsg, got %T", msg)
	}
}

func TestTransferConfirmedMsg_Success(t *testing.T) {
	m := newTestModel()

	// Go to confirming step via TransferSentMsg
	updated, _ := m.Update(tui.TransferSentMsg{Signature: "5abc123def456"})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepConfirming {
		t.Fatalf("step = %d, want StepConfirming", m.CurrentStep())
	}

	// Send TransferConfirmedMsg
	updated, _ = m.Update(tui.TransferConfirmedMsg{Signature: "5abc123def456", Confirmed: true})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepResult {
		t.Errorf("step after TransferConfirmedMsg = %d, want StepResult", m.CurrentStep())
	}

	view := m.View()
	if !strings.Contains(view, "Confirmed") {
		t.Error("result should indicate confirmation")
	}
	if !strings.Contains(view, "5abc123def456") {
		t.Error("result should contain signature")
	}
	if !strings.Contains(view, "solscan.io") {
		t.Error("result should contain explorer link")
	}
}

func TestSend_ResponsiveInputWidths(t *testing.T) {
	portfolio := models.PortfolioBalance{
		SOLBalance: models.TokenBalance{Symbol: "SOL", Amount: 1.0, Decimals: 9},
	}
	m := send.New("addr123", nil, nil, portfolio)

	model, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	_ = model

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
}

func TestSend_ResponsiveInputWidths_Wide(t *testing.T) {
	portfolio := models.PortfolioBalance{
		SOLBalance: models.TokenBalance{Symbol: "SOL", Amount: 1.0, Decimals: 9},
	}
	m := send.New("addr123", nil, nil, portfolio)

	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = model

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at wide width")
	}
}

// --- New tests: SOL vs SPL, MAX amount, initial state ---

func TestInitialState_ShowsTokenSelection(t *testing.T) {
	m := newTestModel()
	if m.CurrentStep() != send.StepSelectToken {
		t.Errorf("initial step = %d, want StepSelectToken", m.CurrentStep())
	}
	view := m.View()
	if !strings.Contains(view, "Select token") {
		t.Error("initial view should show token selection prompt")
	}
}

// advanceToReviewSOL drives the model through all steps to StepReview with SOL selected.
func advanceToReviewSOL(t *testing.T) send.Model {
	t.Helper()
	m := newTestModel()

	// Select SOL (cursor at 0).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepRecipient {
		t.Fatalf("expected StepRecipient, got %d", m.CurrentStep())
	}

	// Type a valid recipient.
	recipient := "RecipientAddr111111111111111111111111111111"
	for _, r := range recipient {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepAmount {
		t.Fatalf("expected StepAmount, got %d", m.CurrentStep())
	}

	// Type amount.
	for _, r := range "1.0" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepReview {
		t.Fatalf("expected StepReview, got %d", m.CurrentStep())
	}
	return m
}

// advanceToReviewSPL drives the model to StepReview with USDC (SPL) selected.
func advanceToReviewSPL(t *testing.T) send.Model {
	t.Helper()
	m := newTestModel()

	// Move cursor to USDC (index 1).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(send.Model)

	// Select USDC.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepRecipient {
		t.Fatalf("expected StepRecipient, got %d", m.CurrentStep())
	}

	// Type a valid recipient.
	recipient := "RecipientAddr111111111111111111111111111111"
	for _, r := range recipient {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepAmount {
		t.Fatalf("expected StepAmount, got %d", m.CurrentStep())
	}

	// Type amount.
	for _, r := range "10.0" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepReview {
		t.Fatalf("expected StepReview, got %d", m.CurrentStep())
	}
	return m
}

func TestSOLSend_ReviewShowsFee(t *testing.T) {
	m := advanceToReviewSOL(t)
	view := m.View()
	// Review must show fee line.
	if !strings.Contains(view, "Fee") {
		t.Error("SOL send review should show fee line")
	}
	if !strings.Contains(view, "SOL") {
		t.Error("SOL send review should mention SOL")
	}
}

func TestSPLSend_ReviewShowsFee(t *testing.T) {
	m := advanceToReviewSPL(t)
	view := m.View()
	if !strings.Contains(view, "Fee") {
		t.Error("SPL send review should show fee line")
	}
	if !strings.Contains(view, "USDC") {
		t.Error("SPL send review should mention USDC")
	}
}

func TestMAXAmount_SOL_AdvancesToReview(t *testing.T) {
	m := newTestModel()

	// Select SOL.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Type recipient.
	recipient := "RecipientAddr111111111111111111111111111111"
	for _, r := range recipient {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Type MAX.
	for _, r := range "MAX" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	if m.CurrentStep() != send.StepReview {
		t.Errorf("MAX amount should advance to StepReview, got step %d", m.CurrentStep())
	}
}

func TestMAXAmount_SPL_AdvancesToReview(t *testing.T) {
	m := newTestModel()

	// Select USDC (index 1).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(send.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Type recipient.
	recipient := "RecipientAddr111111111111111111111111111111"
	for _, r := range recipient {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Type MAX.
	for _, r := range "MAX" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	if m.CurrentStep() != send.StepReview {
		t.Errorf("MAX amount for SPL should advance to StepReview, got step %d", m.CurrentStep())
	}
}

func TestSelfSend_ShowsError(t *testing.T) {
	m := newTestModel()

	// Select SOL.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Type own address.
	selfAddr := "SenderAddr111111111111111111111111111111111"
	for _, r := range selfAddr {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	if m.CurrentStep() != send.StepRecipient {
		t.Error("self-send should stay on recipient step")
	}
	view := m.View()
	if !strings.Contains(view, "own address") {
		t.Error("self-send should show 'own address' error")
	}
}

func TestAmountExceedsBalance_ShowsError(t *testing.T) {
	m := newTestModel()

	// Select SOL (5.0 SOL balance).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Type recipient.
	recipient := "RecipientAddr111111111111111111111111111111"
	for _, r := range recipient {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Type amount exceeding balance.
	for _, r := range "999.0" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	if m.CurrentStep() != send.StepAmount {
		t.Error("exceeding balance should stay on amount step")
	}
	view := m.View()
	if !strings.Contains(view, "Error") {
		t.Error("exceeding balance should show error")
	}
}

func TestAmountNegative_ShowsError(t *testing.T) {
	m := newTestModel()

	// Select SOL.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Type recipient.
	recipient := "RecipientAddr111111111111111111111111111111"
	for _, r := range recipient {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	// Type negative amount.
	for _, r := range "-1.0" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(send.Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	if m.CurrentStep() != send.StepAmount {
		t.Error("negative amount should stay on amount step")
	}
	view := m.View()
	if !strings.Contains(view, "Error") {
		t.Error("negative amount should show error")
	}
}

func TestPasswordStep_EmptyPassword_ShowsError(t *testing.T) {
	m := advanceToReviewSOL(t)

	// Advance to password step.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepPassword {
		t.Fatalf("expected StepPassword, got %d", m.CurrentStep())
	}

	// Press enter without typing password.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(send.Model)

	if m.CurrentStep() != send.StepPassword {
		t.Error("empty password should stay on password step")
	}
	view := m.View()
	if !strings.Contains(view, "Error") {
		t.Error("empty password should show error")
	}
}

func TestConfirmingStep_View(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tui.TransferSentMsg{Signature: "sig123abc"})
	m = updated.(send.Model)
	if m.CurrentStep() != send.StepConfirming {
		t.Fatalf("expected StepConfirming, got %d", m.CurrentStep())
	}
	view := m.View()
	if !strings.Contains(view, "sig123abc") {
		t.Error("confirming view should show signature")
	}
}
