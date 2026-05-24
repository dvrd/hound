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
const maxConcurrentTxFetches = 10

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

// ActivityResult holds the result of a single page fetch.
type ActivityResult struct {
	Items   []ActivityItem
	LastSig string // last RPC signature — use as cursor for next page
	HasMore bool   // true when the RPC returned a full page (more may exist)
}

// GetActivity fetches one page of on-chain activity starting after `before`.
// Pass before="" for the first page. Use result.LastSig as `before` for subsequent pages.
func (s *ActivityService) GetActivity(ctx context.Context, rpcClient *blockchain.RPCClient, address string, limit int, before string) (ActivityResult, error) {
	if rpcClient == nil {
		return ActivityResult{}, fmt.Errorf("get activity: RPC client is nil")
	}

	// 1. Fetch signatures for this page.
	sigs, err := blockchain.GetSignaturesForAddress(ctx, rpcClient, address, limit, before)
	if err != nil {
		return ActivityResult{}, fmt.Errorf("get activity: %w", err)
	}
	if len(sigs) == 0 {
		return ActivityResult{}, nil
	}

	lastSig := sigs[len(sigs)-1].Signature
	hasMore := len(sigs) == limit

	// 2. Fan-out: fetch transaction details concurrently with bounded parallelism.
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
			sem <- struct{}{}
			defer func() { <-sem }()

			detail, err := blockchain.GetTransaction(ctx, rpcClient, signature)
			if err != nil || detail == nil {
				return
			}
			item := classifyTransaction(detail, address)
			results[idx] = indexedResult{index: idx, item: item, ok: true}
		}(i, sig.Signature)
	}
	wg.Wait()

	// 3. Collect successful results (preserving order).
	items := make([]ActivityItem, 0, len(sigs))
	for _, r := range results {
		if r.ok {
			items = append(items, r.item)
		}
	}

	// 4. Merge with local swap history.
	// Check only the signatures from this page against the swap_history table.
	if s.db != nil && len(items) > 0 {
		sigs := make([]string, len(items))
		for i, item := range items {
			sigs[i] = item.Signature
		}
		swapSigs := s.db.GetSwapSignatures(sigs)
		for i := range items {
			if swapSigs[items[i].Signature] {
				items[i].Type = "swap"
			}
		}
	}

	// 5. Sort by timestamp descending.
	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp > items[j].Timestamp
	})

	return ActivityResult{Items: items, LastSig: lastSig, HasMore: hasMore}, nil
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

	// Classify based on top-level instructions
	for _, ix := range detail.Instructions {
		if classified := classifyInstruction(&item, ix, address, detail); classified {
			return item
		}
	}

	// M3: Also check inner instructions (CPI transfers)
	for _, innerSet := range detail.InnerInstructions {
		for _, ix := range innerSet.Instructions {
			if classified := classifyInstruction(&item, ix, address, detail); classified {
				return item
			}
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

// classifyInstruction attempts to classify a single instruction. Returns true if classified.
func classifyInstruction(item *ActivityItem, ix blockchain.ParsedInstruction, address string, detail *blockchain.TransactionDetail) bool {
	switch {
	case ix.Program == "system" && ix.Type == "transfer":
		item.Type = "sol_transfer"
		classifySOLTransfer(item, ix, address, detail)
		return true
	case ix.Program == "spl-token" && (ix.Type == "transfer" || ix.Type == "transferChecked"):
		item.Type = "spl_transfer"
		classifySPLTransfer(item, ix, address, detail)
		return true
	}
	return false
}

// classifySOLTransfer determines direction and amount for a SOL transfer.
func classifySOLTransfer(item *ActivityItem, ix blockchain.ParsedInstruction, address string, detail *blockchain.TransactionDetail) {
	source, _ := ix.Info["source"].(string)
	destination, _ := ix.Info["destination"].(string)
	lamportsVal, _ := ix.Info["lamports"].(float64)
	lamports := uint64(lamportsVal)

	if source == address {
		item.Direction = "sent"
		item.Counterparty = truncateAddress(destination)
	} else if destination == address {
		item.Direction = "received"
		item.Counterparty = truncateAddress(source)
	}

	item.Amount = formatLamports(lamports)
}

// classifySPLTransfer determines direction and amount for an SPL transfer.
func classifySPLTransfer(item *ActivityItem, ix blockchain.ParsedInstruction, address string, detail *blockchain.TransactionDetail) {
	authority, _ := ix.Info["authority"].(string)
	source, _ := ix.Info["source"].(string)
	destination, _ := ix.Info["destination"].(string)

	// Fix 7: Use authority field for direction detection.
	// For SPL transfers, source/destination are ATA addresses, not wallet addresses.
	// The authority field tells us who initiated the transfer.
	if authority == address {
		item.Direction = "sent"
		item.Counterparty = truncateAddress(destination)
	} else {
		// Not the authority — likely a receive. Fall through to balance-based
		// classification for accuracy, but set counterparty from source.
		item.Direction = "received"
		item.Counterparty = truncateAddress(source)
		// Double-check with balance-based classification
		classifyDirectionFromBalances(item, detail, address)
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
	if len(detail.PreBalances) == 0 || len(detail.PostBalances) == 0 {
		return
	}

	// Fix 6: Find the wallet's index in account keys instead of always using index 0
	idx := -1
	for i, key := range detail.AccountKeys {
		if key == address {
			idx = i
			break
		}
	}

	if idx < 0 || idx >= len(detail.PreBalances) || idx >= len(detail.PostBalances) {
		return // address not found in account keys
	}

	pre := detail.PreBalances[idx]
	post := detail.PostBalances[idx]
	if post < pre {
		item.Direction = "sent"
	} else if post > pre {
		item.Direction = "received"
	}
}

// truncateAddress returns "xxxx...xxxx" format.
func truncateAddress(addr string) string {
	if len(addr) <= 8 {
		return addr
	}
	return addr[:4] + "..." + addr[len(addr)-4:]
}

// formatLamports converts lamports to a human-readable SOL string.
func formatLamports(lamports uint64) string {
	sol := float64(lamports) / 1e9
	return fmt.Sprintf("%g SOL", sol)
}
