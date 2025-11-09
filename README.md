# Hound

A lightweight CLI tool for tracking Solana token prices from the terminal, written in Odin.

## Features

- Real-time token price fetching from DexScreener API
- 24-hour price change tracking
- **Multi-token support via JSON configuration**
- **Symbol-based token lookup (e.g., `hound aura`)**
- **Case-insensitive symbol matching**
- **List configured tokens with `hound list`**
- Production-ready error handling with user-friendly messages
- BSD-compliant exit codes for scripting
- Native DNS resolution on macOS (fixes VPN/custom DNS issues)
- Clean, formatted output for terminal display

## Prerequisites

- [Odin compiler](https://odin-lang.org/) (latest release)
- macOS or Linux
- Task runner: `brew install go-task` or `go install github.com/go-task/task/v3/cmd/task@latest`

## Installation

Clone the repository:

```bash
git clone https://github.com/dvrd/hound.git
cd hound
```

### Configuration Setup

Create a configuration file at `~/.config/hound/tokens.json`:

```bash
mkdir -p ~/.config/hound
cat > ~/.config/hound/tokens.json << 'EOF'
{
  "version": "1.0.0",
  "tokens": [
    {
      "symbol": "aura",
      "name": "AURA Memecoin",
      "contract_address": "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
      "chain": "solana"
    },
    {
      "symbol": "sol",
      "name": "Solana",
      "contract_address": "So11111111111111111111111111111111111111112",
      "chain": "solana"
    }
  ]
}
EOF
```

Add your favorite Solana tokens by adding more entries to the `tokens` array. Each token needs:
- `symbol`: Short identifier (e.g., "aura", "sol")
- `name`: Full token name
- `contract_address`: Solana contract address
- `chain`: Must be "solana"

### Git Hooks (Optional)

For contributors: Install git hooks to automatically sync version files:

```bash
task hooks:install
```

This installs a pre-commit hook that automatically updates `src/version.odin` when `VERSION` changes.

## Usage

### Build

Debug build (recommended for development):
```bash
task debug
```

Release build (optimized):
```bash
task build
```

Clean build artifacts:
```bash
task clean
```

### Run

Fetch price for a configured token by symbol:
```bash
./bin/hound aura
```

Output:
```
aura: $0.060230 (+0.2%)
```

List all configured tokens:
```bash
./bin/hound list
```

Output:
```
Available tokens:

  aura - AURA Memecoin
  sol - Solana
```

Show version information:
```bash
./bin/hound --version
# or
./bin/hound -v
```

Output:
```
hound v0.4.2
```

Symbol lookup is case-insensitive:
```bash
./bin/hound AURA    # Works the same as 'aura'
./bin/hound SoL     # Works the same as 'sol'
```

### Exit Codes

Hound uses BSD-compliant exit codes for scripting:

- `0` - Success
- `1` - General error (token not configured)
- `2` - Usage error (missing/invalid arguments)
- `69` - Service unavailable (API errors, rate limits)
- `70` - Internal error (parsing failures)
- `78` - Configuration error (config not found, invalid JSON)

Example usage in scripts:
```bash
./bin/hound aura
if [ $? -eq 0 ]; then
    echo "Price fetched successfully"
fi
```

### Error Messages

Hound provides specific error types with helpful messages:

- **Missing argument**: Shows usage instructions with examples
- **Token not configured**: Lists available tokens and config location
- **Config not found**: Shows example config file to create
- **Config parse error**: Explains required JSON format
- **Token not found**: Suggests checking the contract address on DexScreener
- **Rate limited**: Advises waiting before retrying
- **Network errors**: Distinguishes between timeouts and connection failures
- **Server errors**: Reports API unavailability

## Architecture

### Project Structure

```
hound/
├── src/
│   ├── main.odin           # Entry point and error display
│   ├── token_config.odin   # Token configuration loading and symbol resolution
│   ├── price_fetcher.odin  # DexScreener API integration
│   ├── types.odin          # Error types and data structures
│   └── output.odin         # Price output formatting
├── vendor/
│   └── odin-http/          # HTTP client library (patched for macOS)
├── PRPs/                   # Project requirements documents
├── Taskfile.yml           # Build tasks
└── README.md
```

### Technologies

- **Language**: [Odin](https://odin-lang.org/) - Fast, concise systems programming language
- **HTTP Client**: [odin-http](https://github.com/laytan/odin-http) - Beta HTTP client library
- **API**: [DexScreener](https://dexscreener.com/) - DEX aggregator API
- **Build Tool**: [Task](https://taskfile.dev/) - Modern task runner

## macOS DNS Resolution

This project includes a fix for DNS resolution issues on macOS. Odin's `core:net` package reads `/etc/resolv.conf`, which is not consulted on macOS. Hound implements native `getaddrinfo()` support that properly uses mDNSResponder, ensuring reliable DNS resolution even with VPN or custom DNS configurations.

## Development

### Development Philosophy

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

### Running Tests

Hound includes a comprehensive test suite with 42 tests covering all critical functionality:

```bash
# Run all tests
task test

# Run specific test suites
task test:decoder      # Raydium pool decoder tests
task test:price        # Price calculation tests
task test:config       # Configuration tests
task test:integration  # End-to-end integration tests

# Run with verbose output
task test:verbose

# Watch mode (auto-run on file changes)
task test:watch
```

All tests serve dual purposes:
1. **Verification** - Ensure code correctness
2. **Documentation** - Show how the system works

📊 **Test Results**: 42 tests, 100% pass rate, ~66ms execution time

📚 **Read More**: [`tests/README.md`](tests/README.md)

### Manual Testing

Test the application with configured tokens:
```bash
task debug
./bin/hound_debug list        # List all configured tokens
./bin/hound_debug aura        # Test AURA token lookup
./bin/hound_debug sol         # Test SOL token lookup
```

## Documentation

Hound maintains comprehensive documentation for developers and contributors:

### Core Documentation
- **[Development Philosophy](.claude/DEVELOPMENT_PHILOSOPHY.md)** - Complete engineering principles and standards (inspired by TigerBeetle)
- **[Quick Reference](.claude/QUICK_REFERENCE.md)** - Fast lookup for common patterns and checklists
- **[Test Suite Guide](tests/README.md)** - Comprehensive testing documentation with examples
- **[Versioning Guide](VERSIONING.md)** - Semantic versioning and release management

### Technical Documentation
- **[Raydium Reverse Engineering](RAYDIUM_REVERSE_ENGINEERING.md)** - Deep dive into on-chain pool structure analysis
- **Project Structure** - See [Architecture](#architecture) section above

### External Resources
- **[TigerBeetle TIGER_STYLE](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md)** - Source of our engineering philosophy
- **[Raydium SDK](https://github.com/raydium-io/raydium-sdk)** - Official Raydium protocol documentation

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

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- **[TigerBeetle](https://tigerbeetle.com/)** - Inspiration for our engineering philosophy
- **[odin-http](https://github.com/laytan/odin-http)** by Laytan Laats - HTTP client library
- **[DexScreener](https://dexscreener.com/)** - DEX aggregator API
- **[Odin programming language](https://odin-lang.org/)** by Ginger Bill - The language that powers Hound
- **[Raydium](https://raydium.io/)** - On-chain liquidity protocol
