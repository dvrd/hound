# Hound

A lightweight CLI tool for tracking Solana token prices from the terminal, written in Odin.

## Features

- Real-time token price fetching from DexScreener API
- 24-hour price change tracking
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

Fetch price for AURA token (default):
```bash
./bin/hound DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2
```

Output:
```
AURA: $0.060230 (+0.2%)
```

For any Solana token, provide its contract address:
```bash
./bin/hound <TOKEN_CONTRACT_ADDRESS>
```

### Exit Codes

Hound uses BSD-compliant exit codes for scripting:

- `0` - Success
- `1` - General error
- `2` - Usage error (missing/invalid arguments)
- `69` - Service unavailable (API errors, rate limits)
- `70` - Internal error (parsing failures)
- `78` - Configuration error (network issues)

Example usage in scripts:
```bash
./bin/hound DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2
if [ $? -eq 0 ]; then
    echo "Price fetched successfully"
fi
```

## Error Handling

Hound provides specific error types with helpful messages:

- **Missing argument**: Shows usage instructions
- **Invalid token**: Displays token address format example
- **Token not found**: Suggests checking the contract address
- **Rate limited**: Advises waiting before retrying
- **Network errors**: Distinguishes between timeouts and connection failures
- **Server errors**: Reports API unavailability

## Architecture

### Project Structure

```
hound/
├── src/
│   ├── main.odin           # Entry point and error display
│   ├── price_fetcher.odin  # DexScreener API integration
│   └── types.odin          # Error types and data structures
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

Test the application with a known token:
```bash
task debug
./bin/hound_debug DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [odin-http](https://github.com/laytan/odin-http) by Laytan Laats
- [DexScreener](https://dexscreener.com/) for the API
- [Odin programming language](https://odin-lang.org/) by Ginger Bill
