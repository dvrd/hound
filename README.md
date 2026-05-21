# Hound

A terminal UI for Solana — manage wallets, track your portfolio, swap tokens, send transfers, and browse history without leaving your shell.

![Hound demo](demo.gif)

## Features

- **Portfolio at a glance** — SOL + all SPL tokens with live USD values and 24h change
- **Instant on open** — portfolios preloaded in the background so the first view is never blank
- **Token details** — market cap, top holders, price history via Jupiter & DexScreener
- **Swap** — execute token swaps through Jupiter aggregator with symbol search overlay
- **Send** — transfer SOL or any SPL token with ComputeBudget priority fees
- **Transaction history** — paginated swap and transfer history via Helius RPC
- **Phantom-compatible** — BIP44 derivation matches Phantom, Solflare, and Ledger
- **Resilient RPC** — automatic retry with backoff on HTTP 429 and endpoint rotation
- **Fast rendering** — zero-alloc cached rendering, 9× faster frame times

## Install

### Homebrew (recommended)

```bash
brew install dvrd/hound/hound
```

### From source

```bash
git clone https://github.com/dvrd/hound.git
cd hound
task build        # binary at ./bin/hound
task install      # or install system-wide
```

Requires [Go 1.25+](https://go.dev/dl/) and [Task](https://taskfile.dev/): `brew install go-task`

## Usage

```bash
hound
```

Everything happens inside the TUI. Navigate with arrow keys, `enter` to drill in, `esc` to go back. Each view shows its keybindings in the pinned footer.

### JSON output (for scripting)

```bash
hound wallet list --json
hound wallet status --json
hound tokens list --json
hound history --json
```

## Security

- Encrypted with AES-256-GCM + Argon2id
- Sensitive data zeroed from memory after use
- No telemetry — your keys never leave your machine
- Alpha software — don't use with funds you can't afford to lose

## Development

```bash
task test     # run all tests
task lint     # lint
task build    # build binary
```

---

**Website**: [hound.devoured.io](https://hound.devoured.io)  
**License**: MIT
