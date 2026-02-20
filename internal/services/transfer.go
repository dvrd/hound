package services

import (
	"crypto/ed25519"
	"fmt"

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

	// 4. Get latest blockhash
	blockhashStr, _, err := blockchain.GetLatestBlockhash(rpcClient)
	if err != nil {
		return "", fmt.Errorf("send SOL: %w", err)
	}

	// 5. Parse pubkeys
	fromPubkey, _ := transaction.PubkeyFromBase58(fromAddr)
	toPubkey, _ := transaction.PubkeyFromBase58(toAddr)
	blockhash, err := transaction.PubkeyFromBase58(blockhashStr)
	if err != nil {
		return "", fmt.Errorf("send SOL: parse blockhash: %w", err)
	}

	// 6. Build instruction
	transferIx := transaction.SystemTransfer(fromPubkey, toPubkey, lamports)

	// 7. Build message
	msg := transaction.NewMessage(fromPubkey, []transaction.Instruction{transferIx}, blockhash)

	// 8. Sign
	tx, err := transaction.NewTransaction(msg, []ed25519.PrivateKey{privKey})
	if err != nil {
		return "", fmt.Errorf("send SOL: sign: %w", err)
	}

	// 9. Submit
	sig, err := blockchain.SendTransaction(rpcClient, tx.ToBase64())
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

	// 4. Parse pubkeys
	fromPubkey, _ := transaction.PubkeyFromBase58(fromAddr)
	toPubkey, _ := transaction.PubkeyFromBase58(toAddr)
	mintPubkey, err := transaction.PubkeyFromBase58(mint)
	if err != nil {
		return "", fmt.Errorf("send SPL: invalid mint: %w", err)
	}

	// 5. Derive ATAs
	senderATA, err := transaction.DeriveATA(fromPubkey, mintPubkey)
	if err != nil {
		return "", fmt.Errorf("send SPL: derive sender ATA: %w", err)
	}
	recipientATA, err := transaction.DeriveATA(toPubkey, mintPubkey)
	if err != nil {
		return "", fmt.Errorf("send SPL: derive recipient ATA: %w", err)
	}

	// 6. Check if recipient ATA exists
	accountData, err := blockchain.GetAccountInfo(rpcClient, recipientATA.String())
	if err != nil {
		return "", fmt.Errorf("send SPL: check recipient ATA: %w", err)
	}

	// 7. Build instructions
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

	// 8. Get blockhash, build message, sign, submit
	blockhashStr, _, err := blockchain.GetLatestBlockhash(rpcClient)
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

	sig, err := blockchain.SendTransaction(rpcClient, tx.ToBase64())
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
