# Hound

A Solana toolkit for token price tracking, wallet management, and portfolio monitoring via CLI and macOS menubar.

## What Does It Do?

- **Track Token Prices**: Real-time Solana token prices from multiple DEXs
- **Manage Wallets**: Secure wallet import with encrypted storage (compatible with Phantom, Solflare, Ledger)
- **Monitor Portfolio**: View all your holdings and their USD values
- **Swap Tokens**: Execute token swaps via Jupiter aggregator
- **MenuBar App**: Live portfolio display in your macOS menubar

## Installation

### Prerequisites
- [Odin compiler](https://odin-lang.org/)
- [Task runner](https://taskfile.dev/): `brew install go-task`
- macOS or Linux

### Build
```bash
git clone https://github.com/dvrd/hound.git
cd hound
task build
```

The binary will be at `./bin/hound`

### MenuBar App (macOS only)
```bash
task menubar:build
task menubar:run
```

## Quick Start

### 1. Import Your Wallet

```bash
./bin/hound wallet import
```

Follow the prompts to:
- Choose wallet standard (BIP44 for Phantom/Solflare compatibility)
- Enter your 12 or 24-word seed phrase
- Set a password (12+ chars, must include uppercase, lowercase, digit, special character)

Your wallet is encrypted and stored at `~/.config/hound/hound.db`

### 2. Check Your Balance

```bash
./bin/hound wallet status
```

Shows:
- SOL balance
- All SPL token balances
- Current USD values
- 24h price changes

### 3. Track Token Prices

```bash
# Add a token
./bin/hound add bonk "Bonk" DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263

# Check price
./bin/hound bonk
# Output: bonk: $0.000028 (+5.2%)

# List all tokens
./bin/hound list
```

## CLI Commands

### Price Commands

```bash
# Check token price
./bin/hound <symbol>
./bin/hound sol

# Force pool rediscovery
./bin/hound fetch <symbol> --refresh

# Add new token
./bin/hound add <symbol> "<name>" <contract_address>

# List configured tokens
./bin/hound list
```

### Wallet Commands

```bash
# Import wallet
./bin/hound wallet import

# List all wallets
./bin/hound wallet list

# Check balances (detailed view)
./bin/hound wallet status

# Show all tokens including zero balance
./bin/hound wallet status --all

# View specific wallet
./bin/hound wallet status --wallet <address|label>

# Sort holdings
./bin/hound wallet status --sort value    # By USD value (default)
./bin/hound wallet status --sort symbol   # Alphabetically
./bin/hound wallet status --sort balance  # By token amount

# Switch active wallet
./bin/hound wallet switch <label>

# Update wallet password
./bin/hound wallet update-password

# Delete wallet
./bin/hound wallet delete <label>
```

### Swap Commands

```bash
# Get swap quote and execute
./bin/hound wallet swap <input_token> <output_token> <amount>

# Example: Swap 1 SOL for USDC
./bin/hound wallet swap sol usdc 1

# Preview only (no execution)
./bin/hound wallet swap sol usdc 1 --dry-run
```

### History

```bash
# View transaction history
./bin/hound history

# Filter by wallet
./bin/hound history --wallet <address>

# Limit results
./bin/hound history --limit 20
```

### Other Commands

```bash
# Show version
./bin/hound version

# Show help
./bin/hound --help
./bin/hound wallet --help
```

## MenuBar App

The menubar app provides real-time portfolio monitoring in your macOS menubar.

### Setup

```bash
# Build and run
task menubar:run

# Or build standalone app bundle
task menubar:bundle
```

### Usage

1. Look for the **🐕 icon** in your menubar
2. Click to view:
   - Total portfolio value
   - Individual token holdings
   - Real-time prices
   - 24h price changes
3. Refreshes automatically in the background

### Troubleshooting MenuBar

```bash
# Kill existing instances
pkill -9 hound-menubar

# Rebuild and run
task menubar:build
task menubar:run

# Check logs
log show --predicate 'process == "Hound"' --last 30s
```

## Wallet Compatibility

Hound supports industry-standard BIP44 wallets:

| Wallet | Compatible | How to Import |
|--------|-----------|---------------|
| 🟣 Phantom | ✅ Yes | Select "BIP44 Standard" during import |
| 🌅 Solflare | ✅ Yes | Select "BIP44 Standard" during import |
| 🔐 Ledger | ✅ Yes | Select "BIP44 Standard" during import |
| 🎒 Backpack | ✅ Yes | Select "BIP44 Standard" during import |
| 🛡️ Trust Wallet | ✅ Yes | Select "BIP44-Change" during import |
| ⚡ solana-keygen | ✅ Yes | Select "Solana CLI" during import |

**Your derived address will match your Phantom/Solflare wallet** when using BIP44 Standard.

## Configuration

### Database Location
```
~/.config/hound/hound.db
```

Contains encrypted wallets and token configuration.

### Supported Networks
- Solana Mainnet-Beta

### Supported DEXs
- **Orca**: Whirlpool, Concentrated Liquidity
- **Raydium**: AMM V4, CLMM
- **Jupiter**: Aggregator (for swaps)

## Common Issues

### "Token not found"
Add the token first:
```bash
./bin/hound add <symbol> "<name>" <contract_address>
```

### "No pools configured"
Force pool discovery:
```bash
./bin/hound fetch <symbol> --refresh
```

### "Wallet import failed"
Check that:
- Seed phrase is 12 or 24 words
- Password meets requirements (12+ chars with uppercase, lowercase, digit, special)
- `~/.config/hound/` directory has correct permissions

### "Insufficient balance"
Make sure you have:
- Enough input tokens for the swap
- Enough SOL to pay transaction fees (~0.000005 SOL)

### MenuBar app not appearing
```bash
pkill -9 hound-menubar
task menubar:build
task menubar:run
```

### Prices not updating
- Check internet connection
- Token may need pool refresh: `./bin/hound fetch <symbol> --refresh`
- Verify token exists: `./bin/hound list`

## Security Notes

- **Encrypted Storage**: Wallets encrypted with AES-256-GCM + Argon2id
- **Password Requirements**: 12+ characters with uppercase, lowercase, digit, and special character
- **Memory Safety**: Sensitive data is zeroed after use
- **Never Share**: Don't share your seed phrase or password with anyone

**⚠️ Important**:
- This is alpha software - use at your own risk
- Always verify addresses and amounts before swaps
- Never use publicly known test mnemonics with real funds
- Keep backups of your seed phrases securely offline

## Support

For issues or questions:
- Check the troubleshooting section above
- Review command help: `./bin/hound --help`
- Verify wallet compatibility in the table above

---

**Version**: v0.21.0

**License**: MIT
