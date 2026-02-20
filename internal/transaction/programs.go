package transaction

// Well-known Solana program IDs.
var (
	SystemProgramID = mustPubkeyFromBase58("11111111111111111111111111111111")
	TokenProgramID  = mustPubkeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	ATAProgramID    = mustPubkeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	SysvarRentID    = mustPubkeyFromBase58("SysvarRent111111111111111111111111111111111")
	SOLMint         = mustPubkeyFromBase58("So11111111111111111111111111111111111111112")
)

func mustPubkeyFromBase58(s string) Pubkey {
	pk, err := PubkeyFromBase58(s)
	if err != nil {
		panic("invalid program ID: " + s + ": " + err.Error())
	}
	return pk
}
