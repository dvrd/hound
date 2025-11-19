# Hound v0.6.0 Release Notes

**Release Date:** November 19, 2025
**Download:** [Hound-0.6.0.dmg](https://github.com/yourusername/hound/releases/download/v0.6.0/Hound-0.6.0.dmg) (657 KB)

---

## 🎯 What's New

### Automatic Token Metadata Discovery

The biggest feature in v0.6.0 is **automatic token discovery**! No more seeing truncated addresses like "4K1Q..KKoB" for unknown tokens.

#### How It Works

1. **Automatic Lookup**: When Hound encounters an unknown SPL token in your wallet, it automatically queries the Jupiter Token List API
2. **Smart Caching**: Discovered tokens are saved to your local database for instant lookup next time
3. **Seamless Fallback**: If a token can't be found, the app gracefully falls back to showing a truncated address
4. **Zero Configuration**: No need to manually add tokens to `tokens.json`

#### Example

Before v0.6.0:
```
Portfolio:
- SOL: 0.032 ($4.35)
- aura: 129,902 ($4,668.90)
- EPjF..Dt1v: 0.007 ($0.01)    ← Unknown token
- 4K1Q..KKoB: 42,449 ($0.00)   ← Unknown token
```

After v0.6.0:
```
Portfolio:
- SOL: 0.032 ($4.35)
- aura: 129,902 ($4,668.90)
- USDC: 0.007 ($0.01)           ← Auto-discovered!
- UNKNOWN: 42,449 ($0.00)       ← Token not in Jupiter list
```

---

## 🔧 Technical Improvements

### New Components

- **`token_metadata.odin`**: New module for Jupiter Token List API integration
  - `lookup_token_metadata()`: Fetches token metadata by mint address
  - `save_discovered_token()`: Persists discovered tokens to database

### Database Enhancements

- **`get_token_by_contract_address()`**: New query function for contract address lookups
- Optimized token lookup with three-level strategy:
  1. In-memory configuration (fastest)
  2. Local database (previously discovered)
  3. Jupiter API (network request)

### Balance Fetcher Improvements

- Multi-level token resolution
- Automatic database persistence
- Better error handling with `NetworkError` and `ParseError` types

---

## 🐛 Bug Fixes

These fixes were also included from the 0.5.0 → 0.6.0 development cycle:

### Critical Fixes

1. **Menubar App Crash on Launch** (commit `54bcfcb`)
   - **Issue**: App crashed with SIGSEGV during initialization
   - **Cause**: Network requests made before macOS networking stack was ready
   - **Fix**: Deferred initial fetch using NSTimer with 2-second delay

2. **RPC Connection Failures** (commit `d402545`)
   - **Issue**: Use-after-free bug causing corrupted pointer values
   - **Cause**: Balance fetcher stored pointer to local variable that went out of scope
   - **Fix**: Proper pointer management and explicit fixup after struct copy

### Stability Improvements

- Better struct lifecycle management in wallet manager
- Improved memory safety for nested struct pointers
- Enhanced error reporting for network failures

---

## 📦 Installation

### Requirements

- **macOS**: 10.15 (Catalina) or later
- **Internet**: Active connection required for token discovery
- **Disk Space**: ~5 MB for app and database

### Steps

1. Download `Hound-0.6.0.dmg`
2. Open the DMG file
3. Drag `Hound.app` to your Applications folder
4. Launch from Applications or Spotlight

### First Launch

If you see a security warning:
1. Go to **System Preferences → Security & Privacy**
2. Click **"Open Anyway"** for Hound
3. This is a one-time step for unsigned apps

---

## 🔄 Upgrading from v0.5.0

### What's Preserved

- ✅ Your wallet addresses
- ✅ Token configurations in `~/.config/hound/tokens.json`
- ✅ Historical balance data in `~/.config/hound/hound.db`
- ✅ All settings and preferences

### What's New

- ✅ Automatic token discovery for unknown tokens
- ✅ Enhanced database schema (auto-migrated)
- ✅ New error types for better diagnostics

### Migration Steps

1. **Backup** (optional but recommended):
   ```bash
   cp ~/.config/hound/hound.db ~/.config/hound/hound.db.backup
   ```

2. **Replace the app**:
   - Quit the old version
   - Install v0.6.0 from DMG
   - Launch the new version

3. **Verification**:
   - Check that your wallets still appear
   - Unknown tokens should now show proper symbols (if in Jupiter list)

---

## 🧪 What's Been Tested

- ✅ macOS 14 Sonoma (Intel & Apple Silicon)
- ✅ macOS 13 Ventura
- ✅ Solana mainnet RPC endpoints
- ✅ Jupiter Token List API integration
- ✅ Database migrations from 0.5.0
- ✅ Multi-wallet portfolio tracking
- ✅ Price fetching from DEXes and APIs

---

## 📊 Performance

### App Size
- **Bundle**: 724 KB (uncompressed)
- **DMG**: 657 KB (compressed)

### Token Discovery
- **First lookup**: ~500ms (API request)
- **Cached lookup**: <1ms (database query)
- **Network timeout**: 10 seconds

### Memory Usage
- **Idle**: ~8 MB
- **Active refresh**: ~12 MB
- **With 10 tokens**: ~15 MB

---

## 🔮 Coming Soon

While this release focuses on token discovery, we're planning:

- 🚀 **Multi-DEX Price Aggregation**: Compare prices across Raydium, Orca, Jupiter
- 📊 **Historical Charts**: View price trends over time
- 🔔 **Price Alerts**: Get notified of significant price changes
- 🎨 **Custom Themes**: Dark mode and color customization
- 📱 **Mobile Companion**: iOS/Android app for remote monitoring

---

## 📝 Full Changelog

### Features
- `a4d7360` feat: implement automatic token metadata discovery via Jupiter Token List API

### Fixes
- `d402545` fix: resolve use-after-free bug causing RPC connection failures in menubar app
- `54bcfcb` fix: defer initial network request to prevent menubar app crash

### Chores
- `f1e6b74` chore: bump version to 0.6.0 for auto-discovery release

---

## 🤝 Contributing

Found a bug or have a feature request?

- **Issues**: https://github.com/yourusername/hound/issues
- **Discussions**: https://github.com/yourusername/hound/discussions

---

## 📄 License

Hound is open source software. See LICENSE file for details.

---

## 🙏 Acknowledgments

- **Jupiter**: For providing the comprehensive Token List API
- **Solana**: For the robust blockchain infrastructure
- **Odin Community**: For the excellent systems programming language

---

**Built with ❤️ using Odin and Claude Code**
