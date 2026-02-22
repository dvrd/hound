package services

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/database"
)

// maxConcurrentTxFetches limits parallel GetTransaction calls to avoid rate limiting.
const maxConcurrentTxFetches = 5

// ActivityItem represents a unified transaction history entry.
type ActivityItem struct {
	Signature    string
	Type         string // "sol_transfer", "spl_transfer", "swap", "program_interaction", "unknown"
	Direction    string // "sent", "received", "self"
	Amount       string // human-readable: "1.5 SOL", "100 USDC"
	Counterparty string // the other address (truncated for display)
	Fee          uint64 // fee in lamports
	Timestamp    int64  // unix timestamp
	Status       string // "confirmed", "failed"
	Slot         uint64
}

// ActivityService fetches and merges on-chain transaction history with local swap records.
type ActivityService struct {
	db *database.Database
}

// NewActivityService creates a new ActivityService.
func NewActivityService(db *database.Database) *ActivityService {
	return &ActivityService{db: db}
}

// GetActivity fetches on-chain activity for an address and merges with local swap history.
func (s *ActivityService) GetActivity(rpcClient *blockchain.RPCClient, address string, limit int, before string) ([]ActivityItem, error) {
	if rpcClient == nil {
		return nil, fmt.Errorf("get activity: RPC client is nil")
	}

	ctx := context.Background()

	// 1. Fetch signatures
	sigs, err := blockchain.GetSignaturesForAddress(ctx, rpcClient, address, limit, before)
	if err != nil {
		return nil, fmt.Errorf("get activity: %w", err)
	}

	if len(sigs) == 0 {
		return nil, nil
	}

	// 2. Fan-out: fetch transaction details concurrently with bounded parallelism
	type indexedResult struct {
		index int
		item  ActivityItem
		ok    bool
	}

	results := make([]indexedResult, len(sigs))
	sem := make(chan struct{}, maxConcurrentTxFetches)
	var wg sync.WaitGroup

	for i, sig := range sigs {
		wg.Add(1)
		go func(idx int, signature string) {
			defer wg.Done()

			sem <- struct{}{}        // acquire semaphore
			defer func() { <-sem }() // release semaphore

			detail, err := blockchain.GetTransaction(ctx, rpcClient, signature)
			if err != nil || detail == nil {
				// Skip transactions we can't fetch (same as before, but concurrent)
				return
			}

			item := classifyTransaction(detail, address)
			results[idx] = indexedResult{index: idx, item: item, ok: true}
		}(i, sig.Signature)
	}

	wg.Wait()

	// 3. Collect successful results (preserving order)
	items := make([]ActivityItem, 0, len(sigs))
	for _, r := range results {
		if r.ok {
			items = append(items, r.item)
		}
	}

	// 4. Merge with local swap history
	if s.db != nil {
		swapEntries, err := s.db.GetSwapHistory(address, limit)
		if err == nil {
			sigToSwap := make(map[string]bool, len(swapEntries))
			for _, entry := range swapEntries {
				sigToSwap[entry.Signature] = true
			}
			for i := range items {
				if sigToSwap[items[i].Signature] {
					items[i].Type = "swap"
				}
			}
		}
	}

	// 5. Sort by timestamp descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp > items[j].Timestamp
	})

	return items, nil
}

// classifyTransaction analyzes a transaction and returns an ActivityItem.
func classifyTransaction(detail *blockchain.TransactionDetail, address string) ActivityItem {
	item := ActivityItem{
		Signature: detail.Signature,
		Slot:      detail.Slot,
		Fee:       detail.Fee,
		Status:    "confirmed",
		Type:      "unknown",
		Direction: "self",
	}

	if detail.BlockTime != nil {
		item.Timestamp = *detail.BlockTime
	}

	if detail.Err != nil {
		item.Status = "failed"
	}

	// Classify based on instructions
	for _, ix := range detail.Instructions {
		switch {
		case ix.Program == "system" && ix.Type == "transfer":
			item.Type = "sol_transfer"
			classifySOLTransfer(&item, ix, address, detail)
			return item
		case ix.Program == "spl-token" && (ix.Type == "transfer" || ix.Type == "transferChecked"):
			item.Type = "spl_transfer"
			classifySPLTransfer(&item, ix, address)
			return item
		}
	}

	// If we have instructions but none matched known types
	if len(detail.Instructions) > 0 {
		item.Type = "program_interaction"
	}

	// Determine direction from balance changes
	classifyDirectionFromBalances(&item, detail, address)

	return item
}

// classifySOLTransfer determines direction and amount for a SOL transfer.
func classifySOLTransfer(item *ActivityItem, ix blockchain.ParsedInstruction, address string, detail *blockchain.TransactionDetail) {
	source, _ := ix.Info["source"].(string)
	destination, _ := ix.Info["destination"].(string)
	lamportsVal, _ := ix.Info["lamports"].(float64)
	lamports := uint64(lamportsVal)

	if source == address {
		item.Direction = "sent"
		item.Counterparty = TruncateAddress(destination)
	} else if destination == address {
		item.Direction = "received"
		item.Counterparty = TruncateAddress(source)
	}

	item.Amount = FormatLamports(lamports)
}

// classifySPLTransfer determines direction and amount for an SPL transfer.
func classifySPLTransfer(item *ActivityItem, ix blockchain.ParsedInstruction, address string) {
	authority, _ := ix.Info["authority"].(string)
	source, _ := ix.Info["source"].(string)
	destination, _ := ix.Info["destination"].(string)

	if authority == address || source == address {
		item.Direction = "sent"
		item.Counterparty = TruncateAddress(destination)
	} else {
		item.Direction = "received"
		item.Counterparty = TruncateAddress(source)
	}

	// Try to get amount from tokenAmount or amount
	if tokenAmount, ok := ix.Info["tokenAmount"].(map[string]interface{}); ok {
		if uiAmountStr, ok := tokenAmount["uiAmountString"].(string); ok {
			item.Amount = uiAmountStr
		}
	} else if amountStr, ok := ix.Info["amount"].(string); ok {
		item.Amount = amountStr
	}
}

// classifyDirectionFromBalances determines direction from pre/post balance changes.
func classifyDirectionFromBalances(item *ActivityItem, detail *blockchain.TransactionDetail, address string) {
	if len(detail.PreBalances) > 0 && len(detail.PostBalances) > 0 {
		// The first account is typically the fee payer
		pre := detail.PreBalances[0]
		post := detail.PostBalances[0]
		if post < pre {
			item.Direction = "sent"
		} else if post > pre {
			item.Direction = "received"
		}
	}
}

// TruncateAddress returns "xxxx...xxxx" format.
func TruncateAddress(addr string) string {
	if len(addr) <= 8 {
		return addr
	}
	return addr[:4] + "..." + addr[len(addr)-4:]
}

// FormatLamports converts lamports to a human-readable SOL string.
func FormatLamports(lamports uint64) string {
	sol := float64(lamports) / 1e9
	return fmt.Sprintf("%g SOL", sol)
}
