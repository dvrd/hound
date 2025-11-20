# Hound

A lightweight CLI tool for tracking Solana token prices from the terminal.

## Installation

```bash
git clone https://github.com/dvrd/hound.git
cd hound
task build
```

## Usage

### Add your first token

```bash
./bin/hound add sol "Solana" So11111111111111111111111111111111111111112
```

### Check a token price

```bash
./bin/hound sol
```

Output:
```
sol: $142.50 (+2.3%)
```

### List all tokens

```bash
./bin/hound list
```

### Other commands

```bash
./bin/hound version              # Show version
./bin/hound fetch aura           # Fetch with pool discovery
./bin/hound fetch sol --refresh  # Force pool rediscovery
```

## How it works

1. **Add tokens** - Store token info in local database (`~/.config/hound/hound.db`)
2. **Auto-discovery** - Hound automatically finds the best liquidity pools
3. **Multi-DEX** - Fetches prices from Orca, Raydium, or Jupiter
4. **Smart caching** - Pools are cached to speed up subsequent requests

## Requirements

- [Odin compiler](https://odin-lang.org/)
- [Task runner](https://taskfile.dev/): `brew install go-task`
- macOS or Linux

## For Developers

See [DEVELOPMENT.md](DEVELOPMENT.md) for architecture, testing, and contribution guidelines.

## License

MIT License - see [LICENSE](LICENSE) file for details.
