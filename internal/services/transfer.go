package services

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/transaction"
)

// TransferService orchestrates SOL and SPL token transfers.
type TransferService struct {
	keystoreService *KeystoreService
	db              *database.Database
}

// NewTransferService creates a new TransferService.
func NewTransferService(keystoreService *KeystoreService, db *database.Database) *TransferService {
	return &TransferService{
		keystoreService: keystoreService,
		db:              db,
	}
}

// SendSOL sends SOL from one address to another.
// Returns the transaction signature.
func (s *TransferService) SendSOL(rpcClient *blockchain.RPCClient, fromAddr, toAddr string, lamports uint64, password string) (string, error) {
	// 1. Validate recipient
	_, err := transaction.PubkeyFromBase58(toAddr)
	if err != nil {
		return "", fmt.Errorf("send SOL: %w", models.ErrInvalidRecipient)
	}

	// 2. Validate not self
	if fromAddr == toAddr {
		return "", fmt.Errorf("send SOL: %w", models.ErrSendToSelf)
	}

	// 3. Unlock keypair
	privKey, err := s.keystoreService.UnlockKeypair(s.db, fromAddr, password)
	if err != nil {
		return "", fmt.Errorf("send SOL: %w", err)
	}
	defer func() {
		for i := range privKey {
			privKey[i] = 0
		}
	}()

	// 4. Check balance
	balance, err := blockchain.GetBalance(context.Background(), rpcClient, fromAddr)
	if err != nil {
		return "", fmt.Errorf("send SOL: get balance: %w", err)
	}
	requiredBalance := lamports + 5000 // amount + base fee
	if balance < requiredBalance {
		return "", fmt.Errorf("send SOL: %w", models.ErrInsufficientBalance)
	}

	// 5. Get latest blockhash
	blockhashStr, _, err := blockchain.GetLatestBlockhash(context.Background(), rpcClient)
	if err != nil {
		return "", fmt.Errorf("send SOL: %w", err)
	}

	// 6. Parse pubkeys
	fromPubkey, _ := transaction.PubkeyFromBase58(fromAddr)
	toPubkey, _ := transaction.PubkeyFromBase58(toAddr)
	blockhash, err := transaction.PubkeyFromBase58(blockhashStr)
	if err != nil {
		return "", fmt.Errorf("send SOL: parse blockhash: %w", err)
	}

	// 7. Build instruction
	transferIx := transaction.SystemTransfer(fromPubkey, toPubkey, lamports)

	// 8. Build message
	msg := transaction.NewMessage(fromPubkey, []transaction.Instruction{transferIx}, blockhash)

	// 9. Sign
	tx, err := transaction.NewTransaction(msg, []ed25519.PrivateKey{privKey})
	if err != nil {
		return "", fmt.Errorf("send SOL: sign: %w", err)
	}

	// 10. Submit
	sig, err := blockchain.SendTransaction(context.Background(), rpcClient, tx.ToBase64())
	if err != nil {
		return "", fmt.Errorf("send SOL: submit: %w", err)
	}

	return sig, nil
}

// SendSPL sends SPL tokens from one address to another.
// Creates recipient's ATA if it doesn't exist (sender pays rent).
func (s *TransferService) SendSPL(rpcClient *blockchain.RPCClient, fromAddr, toAddr, mint string, amount uint64, decimals uint8, password string) (string, error) {
	// 1. Validate recipient
	_, err := transaction.PubkeyFromBase58(toAddr)
	if err != nil {
		return "", fmt.Errorf("send SPL: %w", models.ErrInvalidRecipient)
	}

	// 2. Validate not self
	if fromAddr == toAddr {
		return "", fmt.Errorf("send SPL: %w", models.ErrSendToSelf)
	}

	// 3. Unlock keypair
	privKey, err := s.keystoreService.UnlockKeypair(s.db, fromAddr, password)
	if err != nil {
		return "", fmt.Errorf("send SPL: %w", err)
	}
	defer func() {
		for i := range privKey {
			privKey[i] = 0
		}
	}()

	// 4. Check SOL balance for fees
	solBalance, err := blockchain.GetBalance(context.Background(), rpcClient, fromAddr)
	if err != nil {
		return "", fmt.Errorf("send SPL: get SOL balance: %w", err)
	}
	// Need at least base fee (5000) + potential ATA rent (2_039_280)
	minSOL := uint64(5000 + 2_039_280)
	if solBalance < minSOL {
		return "", fmt.Errorf("send SPL: insufficient SOL for fees: %w", models.ErrInsufficientBalance)
	}

	// 5. Check token balance
	tokenAccounts, err := blockchain.GetTokenAccountsByOwner(context.Background(), rpcClient, fromAddr)
	if err != nil {
		return "", fmt.Errorf("send SPL: get token accounts: %w", err)
	}
	var tokenBalance uint64
	for _, ta := range tokenAccounts {
		if ta.Mint == mint {
			tokenBalance = ta.Amount
			break
		}
	}
	if tokenBalance < amount {
		return "", fmt.Errorf("send SPL: %w", models.ErrInsufficientBalance)
	}

	// 6. Parse pubkeys
	fromPubkey, _ := transaction.PubkeyFromBase58(fromAddr)
	toPubkey, _ := transaction.PubkeyFromBase58(toAddr)
	mintPubkey, err := transaction.PubkeyFromBase58(mint)
	if err != nil {
		return "", fmt.Errorf("send SPL: invalid mint: %w", err)
	}

	// 7. Derive ATAs
	senderATA, err := transaction.DeriveATA(fromPubkey, mintPubkey)
	if err != nil {
		return "", fmt.Errorf("send SPL: derive sender ATA: %w", err)
	}
	recipientATA, err := transaction.DeriveATA(toPubkey, mintPubkey)
	if err != nil {
		return "", fmt.Errorf("send SPL: derive recipient ATA: %w", err)
	}

	// 8. Check if recipient ATA exists
	accountData, err := blockchain.GetAccountInfo(context.Background(), rpcClient, recipientATA.String())
	if err != nil {
		return "", fmt.Errorf("send SPL: check recipient ATA: %w", err)
	}

	// 9. Build instructions
	var instructions []transaction.Instruction
	if accountData == nil {
		// ATA doesn't exist, create it
		createIx, err := transaction.CreateATAInstruction(fromPubkey, toPubkey, mintPubkey)
		if err != nil {
			return "", fmt.Errorf("send SPL: create ATA instruction: %w", err)
		}
		instructions = append(instructions, createIx)
	}

	transferIx := transaction.TokenTransferChecked(senderATA, mintPubkey, recipientATA, fromPubkey, amount, decimals)
	instructions = append(instructions, transferIx)

	// 10. Get blockhash, build message, sign, submit
	blockhashStr, _, err := blockchain.GetLatestBlockhash(context.Background(), rpcClient)
	if err != nil {
		return "", fmt.Errorf("send SPL: %w", err)
	}
	blockhash, err := transaction.PubkeyFromBase58(blockhashStr)
	if err != nil {
		return "", fmt.Errorf("send SPL: parse blockhash: %w", err)
	}

	msg := transaction.NewMessage(fromPubkey, instructions, blockhash)
	tx, err := transaction.NewTransaction(msg, []ed25519.PrivateKey{privKey})
	if err != nil {
		return "", fmt.Errorf("send SPL: sign: %w", err)
	}

	sig, err := blockchain.SendTransaction(context.Background(), rpcClient, tx.ToBase64())
	if err != nil {
		return "", fmt.Errorf("send SPL: submit: %w", err)
	}

	return sig, nil
}

// EstimateFee returns the estimated fee in lamports for a transfer.
// baseFee is 5000 lamports per signature. If createATA is true, adds rent exemption cost.
func (s *TransferService) EstimateFee(createATA bool) uint64 {
	baseFee := uint64(5000)
	if createATA {
		baseFee += 2_039_280 // rent exemption for 165-byte token account
	}
	return baseFee
}

// AwaitConfirmation polls GetSignatureStatuses until the transaction is confirmed/finalized
// or the timeout is reached. Returns nil on confirmation, ErrTransactionFailed if the tx
// failed on-chain, or ErrConfirmationTimeout if polling times out.
func AwaitConfirmation(ctx context.Context, rpcClient *blockchain.RPCClient, signature string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("await confirmation: %w", models.ErrConfirmationTimeout)
		}

		statuses, err := blockchain.GetSignatureStatuses(ctx, rpcClient, []string{signature})
		if err != nil {
			// Network error — keep polling, don't fail immediately
			time.Sleep(pollInterval)
			continue
		}

		if len(statuses) > 0 && statuses[0] != nil {
			status := statuses[0]

			// Check for on-chain error
			if status.Err != nil {
				return fmt.Errorf("await confirmation: %w", models.ErrTransactionFailed)
			}

			// Check confirmation level
			if status.ConfirmationStatus != nil {
				cs := *status.ConfirmationStatus
				if cs == "confirmed" || cs == "finalized" {
					return nil // Success!
				}
			}
		}

		// Not yet confirmed — wait and retry
		select {
		case <-ctx.Done():
			return fmt.Errorf("await confirmation: %w", ctx.Err())
		case <-time.After(pollInterval):
		}
	}
}
