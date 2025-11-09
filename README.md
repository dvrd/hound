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

### Running Tests

Test the application with configured tokens:
```bash
task debug
./bin/hound_debug list        # List all configured tokens
./bin/hound_debug aura        # Test AURA token lookup
./bin/hound_debug sol         # Test SOL token lookup
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [odin-http](https://github.com/laytan/odin-http) by Laytan Laats
- [DexScreener](https://dexscreener.com/) for the API
- [Odin programming language](https://odin-lang.org/) by Ginger Bill
