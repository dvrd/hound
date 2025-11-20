# Development Guide

This document contains detailed technical information for developers contributing to Hound.

## Table of Contents

- [Architecture](#architecture)
- [Development Philosophy](#development-philosophy)
- [Testing](#testing)
- [Configuration](#configuration)
- [Error Handling](#error-handling)
- [Contributing](#contributing)

## Architecture

### Project Structure

```
hound/
├── cli/               # CLI application
│   ├── main.odin     # Entry point
│   ├── commands/     # Command handlers (add, fetch, list, version)
│   └── output/       # Output formatting (errors, messages, price, pool, token)
├── core/             # Core business logic
│   ├── database/     # SQLite database operations
│   ├── memory/       # Arena allocators
│   ├── models/       # Data types and error types
│   ├── blockchain/   # Blockchain interaction (RPC, decoders, oracles)
│   ├── wallet/       # Wallet balance fetching
│   ├── dex/          # DEX operations (router, pool discovery, price fetching)
│   └── services/     # Service layer (business logic separation)
├── src/              # Application-specific code
│   ├── menubar/      # MenuBar application (AppKit bindings, UI)
│   ├── wallet_manager/   # Wallet management
│   ├── jupiter_swap/     # Jupiter swap integration
│   ├── transaction/      # Transaction serialization, Phantom integration
│   ├── token_config/     # Token configuration management
│   └── version/          # Version information
├── tests/            # Test suites
├── vendor/           # Third-party dependencies
│   └── odin-http/   # HTTP client library (patched for macOS)
├── PRPs/             # Product requirements documents
└── scripts/          # Build and utility scripts
```

### Technologies

- **Language**: [Odin](https://odin-lang.org/) - Fast, concise systems programming language
- **HTTP Client**: [odin-http](https://github.com/laytan/odin-http) - Beta HTTP client library
- **DEX Integrations**:
  - [Orca Whirlpool](https://www.orca.so/) - Concentrated Liquidity Market Maker (CLMM)
  - [Jupiter Aggregator](https://jup.ag/) - Price API v3
  - [Raydium](https://raydium.io/) - AMM v4 pools
- **API**: [DexScreener](https://dexscreener.com/) - DEX aggregator API (24h change data)
- **Blockchain**: [Solana RPC](https://solana.com/) - On-chain data fetching
- **Database**: SQLite - Local token and pool storage
- **Build Tool**: [Task](https://taskfile.dev/) - Modern task runner

### Multi-DEX Architecture

Hound uses a priority-based routing system with automatic fallback:

1. **Pool Discovery** - Automatically finds the best liquidity pools for each token
2. **Priority Routing** - Tries pools in order of liquidity/reliability
3. **Fallback Chain** - If all pools fail, falls back to Jupiter Aggregator API
4. **Caching** - Discovered pools are cached in the database for speed

This provides redundancy and maximizes uptime even if specific DEXs are unavailable.

### Database Schema

```sql
CREATE TABLE tokens (
    id INTEGER PRIMARY KEY,
    symbol TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    contract_address TEXT UNIQUE NOT NULL,
    chain TEXT NOT NULL,
    is_quote_token INTEGER DEFAULT 0,
    usd_price REAL DEFAULT 0
);

CREATE TABLE pools (
    id INTEGER PRIMARY KEY,
    token_id INTEGER NOT NULL,
    dex TEXT NOT NULL,
    pool_address TEXT NOT NULL,
    quote_token TEXT NOT NULL,
    pool_type TEXT,
    liquidity_usd REAL,
    discovered_at INTEGER,
    FOREIGN KEY (token_id) REFERENCES tokens(id)
);
```

## Development Philosophy

Hound follows engineering principles inspired by **[TigerBeetle](https://tigerbeetle.com/)**'s rigorous approach to safety-critical systems:

**Core Priorities** (in order):
1. **Safety** - Correct price data is mission-critical
2. **Performance** - Sub-second response times
3. **Developer Experience** - Clear, documented code

**Key Principles**:
- ✅ High assertion density (≥2 per function)
- ✅ Explicit error handling (zero ignored errors)
- ✅ Static memory allocation
- ✅ Tests as living documentation
- ✅ Zero technical debt policy

📚 **Read More**: [`.claude/DEVELOPMENT_PHILOSOPHY.md`](.claude/DEVELOPMENT_PHILOSOPHY.md) | [Quick Reference](.claude/QUICK_REFERENCE.md)

## Testing

### Running Tests

```bash
# Run all tests
task test

# Run specific test suites
task test:decoder      # Pool decoder tests (Orca, Raydium)
task test:price        # Price calculation tests
task test:config       # Configuration tests
task test:integration  # End-to-end integration tests

# Run with verbose output
task test:verbose

# Watch mode (auto-run on file changes)
task test:watch
```

### Test Coverage

- **Orca Decoder**: 12 tests - Q64.64 conversion, pool structure validation
- **Jupiter Client**: 9 tests - API integration, caching, error handling
- **DEX Router**: 10 tests - Priority routing, fallback mechanisms
- **Raydium Decoder**: Tests for AMM v4 pool decoding
- **Integration Tests**: End-to-end multi-DEX scenarios

All tests serve dual purposes:
1. **Verification** - Ensure code correctness
2. **Documentation** - Show how the system works

📊 **Test Results**: 73+ tests, 100% pass rate

📚 **Read More**: [`tests/README.md`](tests/README.md)

### Manual Testing

```bash
task debug
./bin/hound_debug list        # List all configured tokens
./bin/hound_debug add test "Test Token" 4k3Dyjzvzp8eMZWUXbBCjEvwSkkk59S5iCNLY3QrkX6R
./bin/hound_debug test        # Test token lookup
```

## Configuration

### Database Location

Hound stores all configuration in a SQLite database:
- **Path**: `~/.config/hound/hound.db`
- **Schema**: See [Database Schema](#database-schema) above

### Adding Tokens

```bash
# Basic command
./bin/hound add <symbol> <name> <contract_address>

# Example
./bin/hound add aura "AURA Memecoin" DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2
```

The `add` command:
1. Validates the contract address format
2. Checks for duplicates
3. Creates the database if it doesn't exist
4. Optionally runs pool discovery

### Pool Discovery

Pool discovery happens automatically:
1. When you add a new token (optional prompt)
2. When you run `hound fetch <symbol>` for the first time
3. When you use the `--refresh` flag

Discovered pools are cached in the database for subsequent requests.

## Error Handling

### Error Types

Hound uses a comprehensive error type system:

```odin
ErrorType :: enum {
    None,
    MissingArgument,      // No token address provided
    InvalidToken,         // Malformed token address
    TokenNotFound,        // 404 or empty pairs array
    TokenNotConfigured,   // Symbol not found in database
    ConfigNotFound,       // Database doesn't exist
    ConfigParseError,     // Failed to read database
    NetworkTimeout,       // Timeout waiting for response
    ConnectionFailed,     // Cannot establish connection
    RateLimited,          // 429 Too Many Requests
    ServerError,          // 500/503 API down
    InvalidResponse,      // Malformed JSON or unexpected structure
    DatabaseError,        // Database operation failed
    DatabaseCorrupted,    // Database integrity check failed
    PoolSearchFailed,     // Pool search failed
    NoPoolsFound,         // No liquidity pools found
    // ... and more
}
```

### Exit Codes

Hound uses BSD-compliant exit codes for scripting:

- `0` - Success
- `1` - General error (token not configured)
- `2` - Usage error (missing/invalid arguments)
- `65` - Data format error (migration)
- `69` - Service unavailable (API errors, rate limits)
- `70` - Internal error (parsing failures)
- `74` - I/O error (database)
- `78` - Configuration error (config not found, invalid database)

Example usage in scripts:
```bash
./bin/hound aura
if [ $? -eq 0 ]; then
    echo "Price fetched successfully"
fi
```

### User-Friendly Messages

All errors include:
1. **Clear description** - What went wrong
2. **Context** - Why it matters
3. **Action** - What to do next

Example:
```
Error: Token 'aura' not found in database
Run 'hound list' to see available tokens.
Add new tokens with: hound add <symbol> <name> <address>
```

## Contributing

We welcome contributions! Before submitting:

1. **Read the philosophy**: Review [`.claude/DEVELOPMENT_PHILOSOPHY.md`](.claude/DEVELOPMENT_PHILOSOPHY.md)
2. **Follow the checklist**: Use [`.claude/QUICK_REFERENCE.md`](.claude/QUICK_REFERENCE.md)
3. **Write tests**: All new features must include documented tests
4. **Run checks**:
   ```bash
   odin fmt src/ tests/         # Format code
   task test                    # Run all tests
   ```

### Pre-Commit Checklist

- [ ] `odin fmt` run on all files
- [ ] All tests pass (`task test`)
- [ ] ≥2 assertions per function
- [ ] All errors handled explicitly
- [ ] Test includes DOCUMENTATION comment
- [ ] Commit message explains "why" not "what"

### Git Hooks (Optional)

For contributors: Install git hooks to automatically sync version files:

```bash
task hooks:install
```

This installs a pre-commit hook that automatically updates `src/version/version.odin` when `VERSION` changes.

## Additional Resources

### Core Documentation
- **[Development Philosophy](.claude/DEVELOPMENT_PHILOSOPHY.md)** - Complete engineering principles and standards
- **[Quick Reference](.claude/QUICK_REFERENCE.md)** - Fast lookup for common patterns and checklists
- **[Test Suite Guide](tests/README.md)** - Comprehensive testing documentation with examples
- **[Versioning Guide](VERSIONING.md)** - Semantic versioning and release management

### Technical Documentation
- **[Raydium Reverse Engineering](RAYDIUM_REVERSE_ENGINEERING.md)** - Deep dive into on-chain pool structure analysis

### External Resources
- **[TigerBeetle TIGER_STYLE](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md)** - Source of our engineering philosophy
- **[Raydium SDK](https://github.com/raydium-io/raydium-sdk)** - Official Raydium protocol documentation
- **[Orca Whirlpool Docs](https://orca-so.github.io/whirlpools/)** - Whirlpool CLMM documentation

## Acknowledgments

- **[TigerBeetle](https://tigerbeetle.com/)** - Inspiration for our engineering philosophy
- **[odin-http](https://github.com/laytan/odin-http)** by Laytan Laats - HTTP client library
- **[Odin programming language](https://odin-lang.org/)** by Ginger Bill - The language that powers Hound
- **[Orca](https://www.orca.so/)** - Whirlpool CLMM protocol and documentation
- **[Jupiter Aggregator](https://jup.ag/)** - Price API v3 for Solana tokens
- **[Raydium](https://raydium.io/)** - AMM v4 on-chain liquidity protocol
- **[DexScreener](https://dexscreener.com/)** - DEX aggregator API for 24h change data
- **[Solana](https://solana.com/)** - High-performance blockchain infrastructure

## macOS DNS Resolution

This project includes a fix for DNS resolution issues on macOS. Odin's `core:net` package reads `/etc/resolv.conf`, which is not consulted on macOS. Hound implements native `getaddrinfo()` support that properly uses mDNSResponder, ensuring reliable DNS resolution even with VPN or custom DNS configurations.
