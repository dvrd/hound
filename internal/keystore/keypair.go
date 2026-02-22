package keystore

import (
	"crypto/ed25519"
	"crypto/sha256"

	"github.com/dvrd/hound/internal/models"
	"github.com/mr-tron/base58"
)

// Keypair holds an Ed25519 key pair.
type Keypair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// DeriveKeypairBIP44 derives an Ed25519 keypair using BIP44 derivation.
// walletType determines the derivation path, accountIndex is the account number.
func DeriveKeypairBIP44(words []string, walletType models.WalletType, accountIndex int) (Keypair, error) {
	if err := ValidateMnemonic(words); err != nil {
		return Keypair{}, err
	}

	seed, err := MnemonicToSeed(words)
	if err != nil {
		return Keypair{}, err
	}
	defer ZeroBytes(seed[:])

	path := models.GetDerivationPath(walletType, accountIndex)
	hdKey, err := DeriveFromPath(seed, path)
	if err != nil {
		return Keypair{}, err
	}
	defer ZeroBytes(hdKey.Key[:])

	privKey := ed25519.NewKeyFromSeed(hdKey.Key[:])
	pubKey := privKey.Public().(ed25519.PublicKey)

	return Keypair{
		PublicKey:  pubKey,
		PrivateKey: privKey,
	}, nil
}

// Deprecated: Use DeriveKeypairBIP44 instead. Legacy derivation (SHA-256 of mnemonic)
// is not compatible with other wallets (Phantom, Solflare, etc.) and funds cannot be
// recovered using standard wallet software.
//
// DeriveKeypairLegacy derives an Ed25519 keypair using the legacy SHA-256 method.
// Joins words with spaces, SHA-256 hashes the result to get a 32-byte seed, then
// derives an Ed25519 keypair.
func DeriveKeypairLegacy(words []string) (Keypair, error) {
	// Use mutable []byte instead of immutable string (Fix H5).
	mnemonicBytes := JoinWordsToBytes(words)
	defer ZeroBytes(mnemonicBytes)

	hash := sha256.Sum256(mnemonicBytes)
	defer ZeroBytes(hash[:])

	privKey := ed25519.NewKeyFromSeed(hash[:])
	pubKey := privKey.Public().(ed25519.PublicKey)

	return Keypair{
		PublicKey:  pubKey,
		PrivateKey: privKey,
	}, nil
}

// KeypairToAddress returns the Base58-encoded public key (Solana address).
func KeypairToAddress(kp Keypair) string {
	return base58.Encode(kp.PublicKey)
}

// ZeroKeypair zeroes the private key bytes.
func ZeroKeypair(kp *Keypair) {
	ZeroBytes(kp.PrivateKey)
}
