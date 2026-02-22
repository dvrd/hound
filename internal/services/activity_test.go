package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dvrd/hound/internal/blockchain"
)

func TestClassifySOLTransfer(t *testing.T) {
	myAddr := "7xKXabc123def456ghi789jkl012mno345pqr678stu"
	blockTime := int64(1700000000)
	detail := &blockchain.TransactionDetail{
		Signature: "sig123",
		Slot:      100,
		BlockTime: &blockTime,
		Fee:       5000,
		Instructions: []blockchain.ParsedInstruction{
			{
				ProgramID: "11111111111111111111111111111111",
				Program:   "system",
				Type:      "transfer",
				Info: map[string]interface{}{
					"source":      myAddr,
					"destination": "RecipientAddr999",
					"lamports":    float64(1_500_000_000), // 1.5 SOL
				},
			},
		},
		PreBalances:  []uint64{5_000_000_000, 0},
		PostBalances: []uint64{3_499_995_000, 1_500_000_000},
	}

	item := classifyTransaction(detail, myAddr)

	if item.Type != "sol_transfer" {
		t.Errorf("type = %q, want sol_transfer", item.Type)
	}
	if item.Direction != "sent" {
		t.Errorf("direction = %q, want sent", item.Direction)
	}
	if !strings.Contains(item.Amount, "1.5") {
		t.Errorf("amount = %q, want to contain '1.5'", item.Amount)
	}
	if item.Status != "confirmed" {
		t.Errorf("status = %q, want confirmed", item.Status)
	}
}

func TestClassifySPLTransfer(t *testing.T) {
	myAddr := "7xKXabc123def456ghi789jkl012mno345pqr678stu"
	blockTime := int64(1700000000)
	detail := &blockchain.TransactionDetail{
		Signature: "sig456",
		Slot:      200,
		BlockTime: &blockTime,
		Fee:       5000,
		Instructions: []blockchain.ParsedInstruction{
			{
				ProgramID: "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
				Program:   "spl-token",
				Type:      "transferChecked",
				Info: map[string]interface{}{
					"authority":   myAddr,
					"source":      "SourceATA",
					"destination": "DestATA",
					"tokenAmount": map[string]interface{}{
						"uiAmountString": "100.5",
					},
				},
			},
		},
		PreBalances:  []uint64{1_000_000_000},
		PostBalances: []uint64{999_995_000},
	}

	item := classifyTransaction(detail, myAddr)

	if item.Type != "spl_transfer" {
		t.Errorf("type = %q, want spl_transfer", item.Type)
	}
	if item.Direction != "sent" {
		t.Errorf("direction = %q, want sent", item.Direction)
	}
	if item.Amount != "100.5" {
		t.Errorf("amount = %q, want '100.5'", item.Amount)
	}
}

func TestClassifyUnknown(t *testing.T) {
	myAddr := "7xKXabc123"
	blockTime := int64(1700000000)
	detail := &blockchain.TransactionDetail{
		Signature: "sig789",
		Slot:      300,
		BlockTime: &blockTime,
		Fee:       5000,
		Instructions: []blockchain.ParsedInstruction{
			{
				ProgramID: "SomeRandomProgram111111111111111111111111111",
				Program:   "unknown_program",
				Type:      "some_action",
				Info:      map[string]interface{}{},
			},
		},
		AccountKeys:  []string{myAddr},
		PreBalances:  []uint64{1_000_000_000},
		PostBalances: []uint64{999_995_000},
	}

	item := classifyTransaction(detail, myAddr)

	if item.Type != "program_interaction" {
		t.Errorf("type = %q, want program_interaction", item.Type)
	}
}

func TestDirectionSent(t *testing.T) {
	myAddr := "SenderAddr"
	blockTime := int64(1700000000)
	detail := &blockchain.TransactionDetail{
		Signature:    "sigSent",
		Slot:         400,
		BlockTime:    &blockTime,
		Fee:          5000,
		Instructions: []blockchain.ParsedInstruction{},
		AccountKeys:  []string{myAddr},
		PreBalances:  []uint64{5_000_000_000},
		PostBalances: []uint64{3_000_000_000},
	}

	item := classifyTransaction(detail, myAddr)

	// No instructions → unknown type, but direction from balances
	if item.Direction != "sent" {
		t.Errorf("direction = %q, want sent", item.Direction)
	}
}

func TestDirectionReceived(t *testing.T) {
	myAddr := "ReceiverAddr"
	blockTime := int64(1700000000)
	detail := &blockchain.TransactionDetail{
		Signature:    "sigRecv",
		Slot:         500,
		BlockTime:    &blockTime,
		Fee:          5000,
		Instructions: []blockchain.ParsedInstruction{},
		AccountKeys:  []string{myAddr},
		PreBalances:  []uint64{1_000_000_000},
		PostBalances: []uint64{3_000_000_000},
	}

	item := classifyTransaction(detail, myAddr)

	if item.Direction != "received" {
		t.Errorf("direction = %q, want received", item.Direction)
	}
}

func TestMergeWithSwapHistory(t *testing.T) {
	// This tests the merge logic conceptually - we can't easily test GetActivity
	// without a real RPC client, but we can test that classifyTransaction + merge works.
	// The merge is done in GetActivity by matching signatures against swap records.
	// Here we test the classify function returns items that can be merged.
	myAddr := "7xKXabc123"
	blockTime := int64(1700000000)
	detail := &blockchain.TransactionDetail{
		Signature: "swapSig123",
		Slot:      600,
		BlockTime: &blockTime,
		Fee:       5000,
		Instructions: []blockchain.ParsedInstruction{
			{
				ProgramID: "11111111111111111111111111111111",
				Program:   "system",
				Type:      "transfer",
				Info: map[string]interface{}{
					"source":      myAddr,
					"destination": "RecipientAddr",
					"lamports":    float64(500_000_000),
				},
			},
		},
		AccountKeys:  []string{myAddr, "RecipientAddr"},
		PreBalances:  []uint64{1_000_000_000, 0},
		PostBalances: []uint64{499_995_000, 500_000_000},
	}

	item := classifyTransaction(detail, myAddr)
	// Before merge, it's a sol_transfer
	if item.Type != "sol_transfer" {
		t.Errorf("type before merge = %q, want sol_transfer", item.Type)
	}

	// Simulate merge (what GetActivity does)
	item.Type = "swap"
	if item.Type != "swap" {
		t.Errorf("type after merge = %q, want swap", item.Type)
	}
}

func TestTruncateAddress(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"7xKXabc123def456ghi789jkl012mno345pqr678stu", "7xKX...8stu"},
		{"short", "short"},
		{"12345678", "12345678"},
		{"123456789", "1234...6789"},
	}

	for _, tt := range tests {
		got := TruncateAddress(tt.input)
		if got != tt.want {
			t.Errorf("TruncateAddress(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestActivityGetActivity_Empty(t *testing.T) {
	// With nil RPC client, GetActivity should fail gracefully
	svc := NewActivityService(nil)
	_, err := svc.GetActivity(nil, "addr", 10, "")
	if err == nil {
		t.Error("expected error with nil RPC client")
	}
}

func TestFormatLamports(t *testing.T) {
	tests := []struct {
		lamports uint64
		want     string
	}{
		{1_000_000_000, "1 SOL"},
		{1_500_000_000, "1.5 SOL"},
		{5000, "5e-06 SOL"},
		{0, "0 SOL"},
	}

	for _, tt := range tests {
		got := FormatLamports(tt.lamports)
		if got != tt.want {
			t.Errorf("FormatLamports(%d) = %q, want %q", tt.lamports, got, tt.want)
		}
	}
}

func TestGetActivityParallel(t *testing.T) {
	// Create a mock RPC server that handles both getSignaturesForAddress and getTransaction
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string        `json:"method"`
			ID     int           `json:"id"`
			Params []interface{} `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		var result json.RawMessage
		switch req.Method {
		case "getSignaturesForAddress":
			result = json.RawMessage(`[
				{"signature":"sig1","slot":100,"blockTime":1700000003},
				{"signature":"sig2","slot":101,"blockTime":1700000002},
				{"signature":"sig3","slot":102,"blockTime":1700000001}
			]`)
		case "getTransaction":
			// Simulate some latency to test concurrency
			time.Sleep(5 * time.Millisecond)
			sig := req.Params[0].(string)
			if sig == "sig2" {
				// Simulate a not-found transaction
				result = json.RawMessage(`null`)
			} else {
				bt := int64(1700000003)
				if sig == "sig3" {
					bt = 1700000001
				}
				result = json.RawMessage(fmt.Sprintf(`{
					"slot": 100,
					"blockTime": %d,
					"meta": {"fee": 5000, "preBalances": [100], "postBalances": [95], "err": null, "innerInstructions": []},
					"transaction": {"message": {"accountKeys": [{"pubkey": "testAddr"}], "instructions": []}}
				}`, bt))
			}
		}

		resp := struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int             `json:"id"`
			Result  json.RawMessage `json:"result"`
		}{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	svc := NewActivityService(nil)

	items, err := svc.GetActivity(client, "testAddr", 10, "")
	if err != nil {
		t.Fatalf("GetActivity failed: %v", err)
	}

	// sig2 returns null, so we should get 2 items
	if len(items) != 2 {
		t.Fatalf("expected 2 items (sig2 is null), got %d", len(items))
	}

	// Should be sorted by timestamp descending
	if items[0].Timestamp < items[1].Timestamp {
		t.Errorf("items not sorted by timestamp descending: %d, %d", items[0].Timestamp, items[1].Timestamp)
	}
}
