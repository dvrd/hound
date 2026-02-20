# Hound

A Solana toolkit for token price tracking, wallet management, and portfolio monitoring via CLI.

## What Does It Do?

- **Track Token Prices**: Real-time Solana token prices from multiple DEXs
- **Manage Wallets**: Secure wallet import with encrypted storage (compatible with Phantom, Solflare, Ledger)
- **Monitor Portfolio**: View all your holdings and their USD values
- **Swap Tokens**: Execute token swaps via Jupiter aggregator

## Installation

### Prerequisites
- [Go 1.25+](https://go.dev/dl/)
- [Task runner](https://taskfile.dev/): `brew install go-task`

### Build
```bash
git clone https://github.com/dvrd/hound.git
cd hound
task build
```

The binary will be at `./bin/hound`

### Install system-wide
```bash
task install
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
./bin/hound tokens list

# Check all tracked tokens
./bin/hound tokens list --json
```

## CLI Commands

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
./bin/hound wallet status [address|label]
```

### Token Commands

```bash
# List tracked tokens
./bin/hound tokens list
```

### Swap Commands

```bash
# Get swap quote and execute (via TUI)
./bin/hound
# Navigate to swap view from the TUI
```

### History

```bash
# View swap history
./bin/hound history
```

### JSON Output

All list commands support `--json` for scripting:

```bash
./bin/hound wallet list --json
./bin/hound wallet status --json
./bin/hound tokens list --json
./bin/hound history --json
```

### Other Commands

```bash
# Show version
./bin/hound version

# Show help
./bin/hound --help
./bin/hound wallet --help
```

## Development

### Run tests
```bash
task test

# Verbose
task test:verbose

# Watch mode (requires entr)
task test:watch
```

### Lint
```bash
task lint
```

### Version bumping
```bash
task version:patch   # 0.0.X — bug fixes
task version:minor   # 0.X.0 — new features
task version:major   # X.0.0 — breaking changes
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HOUND_RPC_ENDPOINT` | Solana RPC endpoint URL | Public mainnet-beta |

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

## Wallet Compatibility

Hound supports industry-standard BIP44 wallets:

| Wallet | Compatible | How to Import |
|--------|-----------|---------------|
| Phantom | Yes | Select "BIP44 Standard" during import |
| Solflare | Yes | Select "BIP44 Standard" during import |
| Ledger | Yes | Select "BIP44 Standard" during import |
| Backpack | Yes | Select "BIP44 Standard" during import |
| Trust Wallet | Yes | Select "BIP44-Change" during import |
| solana-keygen | Yes | Select "Solana CLI" during import |

**Your derived address will match your Phantom/Solflare wallet** when using BIP44 Standard.

## Common Issues

### "Wallet import failed"
Check that:
- Seed phrase is 12 or 24 words
- Password meets requirements (12+ chars with uppercase, lowercase, digit, special)
- `~/.config/hound/` directory has correct permissions

### "Insufficient balance"
Make sure you have:
- Enough input tokens for the swap
- Enough SOL to pay transaction fees (~0.000005 SOL)

## Security Notes

- **Encrypted Storage**: Wallets encrypted with AES-256-GCM + Argon2id
- **Password Requirements**: 12+ characters with uppercase, lowercase, digit, and special character
- **Memory Safety**: Sensitive data is zeroed after use
- **Never Share**: Don't share your seed phrase or password with anyone

**Important**:
- This is alpha software — use at your own risk
- Always verify addresses and amounts before swaps
- Never use publicly known test mnemonics with real funds
- Keep backups of your seed phrases securely offline

---

**Version**: See `VERSION` file

**License**: MIT
