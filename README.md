# Hound

A comprehensive Solana toolkit combining token price tracking, wallet management, and portfolio monitoring in both CLI and macOS menubar interfaces.

## Features

### 🪙 Token Price Tracking
- Real-time Solana token prices from multiple DEXs (Orca, Raydium, Jupiter)
- Automatic liquidity pool discovery and caching
- 24-hour price change tracking
- Multi-DEX price aggregation with priority-based routing

### 💼 Wallet Management
- Secure wallet import with encrypted seed phrase storage (AES-256-GCM + Argon2id)
- Multi-wallet support with labels and primary wallet selection
- Real-time SOL and SPL token balance tracking
- Token auto-discovery from wallet addresses
- Swap functionality via Jupiter aggregator

### 📊 macOS MenuBar App
- Live portfolio value display in menubar
- Click to view detailed holdings
- Automatic balance refresh
- System tray integration

### 🔐 Security
- Industry-standard cryptography (BIP39/BIP44 support in development)
- Password-protected wallet encryption
- Secure memory handling with zero-ing
- OWASP 2024 password requirements (12+ chars, complexity)

## Installation

### Prerequisites
- [Odin compiler](https://odin-lang.org/) - Install from official website
- [Task runner](https://taskfile.dev/): `brew install go-task`
- macOS or Linux

### Build from Source

```bash
git clone https://github.com/dvrd/hound.git
cd hound
task build
```

### Optional: MenuBar App (macOS only)

```bash
task menubar:build  # Build menubar application
task menubar:run    # Run menubar app
```

## CLI Usage

### Token Price Commands

```bash
# Add a token
./bin/hound add sol "Solana" So11111111111111111111111111111111111111112

# Check token price
./bin/hound sol
# Output: sol: $142.50 (+2.3%)

# Check with forced pool rediscovery
./bin/hound fetch sol --refresh

# List all configured tokens
./bin/hound list
```

### Wallet Commands

```bash
# Import wallet from seed phrase (Phantom/Solflare compatible)
./bin/hound wallet import

# List all wallets
./bin/hound wallet list

# Check wallet balance and holdings
./bin/hound wallet status

# Switch active wallet
./bin/hound wallet switch "My Wallet"

# Update wallet password
./bin/hound wallet update-password
```

### Swap Commands

```bash
# Get swap quote
./bin/hound wallet swap <input_token> <output_token> <amount>

# Example: Swap 1 SOL for USDC
./bin/hound wallet swap sol usdc 1

# Dry-run mode (quote only, no execution)
./bin/hound wallet swap sol usdc 1 --dry-run
```

### Other Commands

```bash
./bin/hound version              # Show version (currently v0.19.2)
./bin/hound history              # Show transaction history
```

## MenuBar App Usage

1. **Build and Run**:
   ```bash
   task menubar:run
   ```

2. **Look for the 🐕 icon** in your macOS menubar

3. **Click the icon** to view:
   - Total portfolio value
   - Individual token balances
   - Real-time prices
   - 24h price changes

4. **Auto-refresh**: Portfolio updates automatically in the background

## How It Works

### Token Price Discovery
1. **Auto-discovery** - Finds best liquidity pools across Orca and Raydium
2. **Multi-DEX aggregation** - Compares prices from multiple sources
3. **Smart caching** - Pools cached for 1 hour, prices cached for 5 minutes
4. **Hybrid pricing** - Combines on-chain pool data with off-chain API data
5. **Priority routing** - Selects DEX based on liquidity and reliability

### Wallet Security
1. **Seed phrase import** - Derives Ed25519 keypair from seed
2. **Encryption** - Private keys encrypted with AES-256-GCM
3. **Key derivation** - Argon2id for password-based encryption
4. **Secure storage** - Encrypted data stored in SQLite database (`~/.config/hound/hound.db`)
5. **Memory safety** - Sensitive data zero-ed after use

### Architecture
- **Language**: Odin (https://odin-lang.org/)
- **Database**: SQLite for local storage
- **RPC**: QuickNode Solana RPC
- **DEXs**: Orca, Raydium, Jupiter
- **Oracles**: Pyth, Switchboard (for SOL price)

## Wallet Compatibility

Hound now supports industry-standard BIP44 wallet import, making it compatible with major Solana wallets:

| Wallet | Import Compatible | Standard Used | Status |
|--------|------------------|---------------|--------|
| 🟣 **Phantom** | ✅ Yes | BIP44 Standard | Fully Tested |
| 🌅 **Solflare** | ✅ Yes | BIP44 Standard | Fully Tested |
| 🔐 **Ledger** | ✅ Yes | BIP44 Standard | Compatible |
| 🎒 **Backpack** | ✅ Yes | BIP44 Standard | Compatible |
| 🛡️ **Trust Wallet** | ✅ Yes | BIP44 Change | Compatible |
| ⚡ **solana-keygen** | ✅ Yes | Solana CLI | Compatible |
| 🐕 **Legacy Hound** | ✅ Yes | Legacy | Backward Compat |

**Import your existing wallet**:
```bash
# Import from Phantom/Solflare
./bin/hound wallet import
# Select: "1. BIP44 Standard (Phantom, Solflare, Ledger, Backpack)"
# Enter your seed phrase (12 or 24 words)
# The derived address will match your Phantom wallet!
```

**Documentation**:
- [Migration Guide](./docs/WALLET_MIGRATION.md) - Switch from Legacy to BIP44
- [Wallet Standards](./docs/WALLET_STANDARDS.md) - Detailed comparison
- [FAQ](./docs/FAQ.md) - Common questions and troubleshooting

## Configuration

### Database Location
```
~/.config/hound/hound.db
```

### Supported Networks
- Solana Mainnet-Beta

### Supported DEXs
- **Orca**: WHIRLPOOL, Concentrated Liquidity
- **Raydium**: AMM V4, CLMM
- **Jupiter**: Aggregator (for swaps)

## Development

### Project Structure
```
hound/
├── src/
│   ├── cli/              # CLI commands and output
│   ├── lib/
│   │   ├── blockchain/   # Solana RPC, oracles
│   │   ├── dex/          # DEX integrations
│   │   ├── keystore/     # Cryptography (BIP39/BIP44 in progress)
│   │   ├── services/     # Business logic
│   │   ├── swap/         # Jupiter swap integration
│   │   └── wallet/       # Wallet management
│   └── menubar/          # macOS menubar app
└── tests/                # Test suite
```

### Running Tests
```bash
task test              # Run all tests
task test:integration  # Run integration tests
```

### Building
```bash
task build             # Build CLI
task menubar:build     # Build menubar app
task menubar:bundle    # Create .app bundle for macOS
```

For detailed architecture, testing guidelines, and contribution info, see [DEVELOPMENT.md](DEVELOPMENT.md).

## Roadmap

### ✅ Completed (v0.19.2)
- Multi-DEX price fetching (Orca, Raydium, Jupiter)
- Wallet import and management
- Token balance tracking
- Swap functionality via Jupiter
- HTTP client retry logic with exponential backoff
- macOS MenuBar application
- Token auto-discovery
- 24-hour price change tracking

### ✅ Recently Completed (v2.0.0)
- **BIP39/BIP44 Standard Support** (Phases 1-5 complete)
  - Full BIP39 mnemonic-to-seed conversion ✅
  - BIP32 HD derivation for Ed25519 ✅
  - PBKDF2-HMAC-SHA512 (2048 iterations) ✅
  - Phantom/Solflare/Ledger wallet compatibility ✅
  - Multi-standard wallet import (BIP44 Standard, BIP44-Change, Solana CLI, Legacy) ✅
  - Interactive CLI for wallet standard selection ✅
  - Comprehensive test coverage (37 tests, 100% pass) ✅
  - Migration guide and user documentation ✅

### 📋 Planned

- **Additional Features**
  - Hardware wallet support (Ledger)
  - Transaction history export
  - Portfolio analytics
  - Price alerts
  - Windows/Linux menubar support

## Version

Current version: **v0.19.2**

See [VERSIONING.md](VERSIONING.md) for version history and release notes.

## Recent Changes

### v0.19.2 (Current)
- Fix: Nil slice assertions in wallet commands
- Fix: Hide tokens with no price in wallet status
- Improve: Error handling for insufficient balance

### v0.19.1
- Fix: AES-GCM authentication tag storage
- Feature: Wallet password update command
- Fix: Sync encrypted_keypairs to wallets table on import

### v0.19.0
- Feature: HTTP client retry logic with exponential backoff
- Feature: Enhanced logging with request/response details
- Feature: Multi-DEX support with priority-based routing
- Feature: Hybrid 24-hour price change tracking

## Troubleshooting

### MenuBar app not showing
```bash
# Kill any existing instances
pkill -9 hound-menubar

# Rebuild and run
task menubar:build
task menubar:run

# Check logs
log show --predicate 'process == "Hound"' --last 30s
```

### Wallet import issues
- Ensure seed phrase is 12 or 24 words
- Password must meet requirements (12+ chars, uppercase, lowercase, digit, special)
- Check database permissions: `~/.config/hound/`

### Token price not showing
- Token may need pool discovery: `./bin/hound fetch <token> --refresh`
- Check if token exists: `./bin/hound list`
- Verify RPC connection (requires internet)

## Contributing

Contributions welcome! Please:
1. Read [DEVELOPMENT.md](DEVELOPMENT.md) for architecture
2. Write tests for new features
3. Update documentation

## Security

- Never commit seed phrases or private keys
- Never use publicly known test mnemonics for real funds
- Report security issues privately (not via GitHub issues)

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [Odin](https://odin-lang.org/)
- Powered by [QuickNode](https://www.quicknode.com/) Solana RPC
- DEX integrations: Orca, Raydium, Jupiter
- Price oracles: Pyth, Switchboard

---

**Note**: This is alpha software. Use at your own risk. Always verify addresses and amounts before executing swaps or transactions.
