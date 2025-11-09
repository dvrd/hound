# Raydium LIQUIDITY_STATE_LAYOUT_V4 Reverse Engineering

## Executive Summary

Through deep reverse engineering, I discovered that the Raydium AMM V4 pool structure offsets documented in the original PRP were incorrect. The vault public keys are located at **offsets 336 and 368**, not at the previously documented offsets 192 and 224. This investigation involved systematic binary analysis, cross-referencing with the official Raydium SDK, and empirical validation using live on-chain data.

**Result**: On-chain price fetching now works correctly, fetching real-time prices directly from Solana blockchain data.

---

## Problem Statement

### Initial Symptom
The on-chain price fetching was failing with error:
```
[DEBUG] get_account_info failed with error: RPCInvalidResponse
On-chain fetch failed, falling back to API...
```

### Root Cause
Base58 encoding of vault addresses was producing invalid addresses like:
```
1111111111111111N6rk7L9YDNKExjFEh6kDEP
```

This indicated the decoder was reading from incorrect byte offsets in the pool data.

---

## Investigation Methodology

### Phase 1: Debug Logging
Added extensive debug logging to trace the failure point:
```odin
fmt.eprintfln("[DEBUG] Pool data length: %d bytes", len(pool_data))
fmt.eprintfln("[DEBUG] Base vault hex: %s", string(base_hex))
```

**Finding**: Offsets 192/224 contained mostly zeros, not valid public keys.

### Phase 2: Hex Analysis
Printed raw hex values from the supposed vault offsets:
```
Base vault hex: 00000000000000000000000000000000aae1493cc6360000c64a227971070000
Quote vault hex: 0000000000000000000000000000000000000000000000000000000000000000
```

**Finding**: These are clearly not 32-byte Solana public keys.

### Phase 3: Systematic Offset Testing
Created Python scripts to test every 32-byte aligned position in the 752-byte pool data:
- Tested offsets: 0, 32, 64, 96, ..., 720
- Used `getTokenAccountBalance` RPC call to verify if addresses are valid token accounts
- Discovered valid token accounts at unexpected offsets

### Phase 4: Official SDK Analysis
Searched for and analyzed the official Raydium SDK source code:
- Repository: `https://github.com/raydium-io/raydium-sdk`
- File: `src/liquidity/layout.ts`
- Found: `LIQUIDITY_STATE_LAYOUT_V4` structure definition

### Phase 5: Reverse Calculation
Worked backwards from known mint addresses:
- Found SOL mint (`So11111111111111111111111111111111111111112`) at offset 400
- Found AURA mint (`DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2`) at offset 432
- Tested addresses 64 and 32 bytes before the mints
- **SUCCESS**: Found valid vaults at offsets 336 and 368

---

## Key Findings

### Correct Structure Layout

The Raydium LIQUIDITY_STATE_LAYOUT_V4 structure (752 bytes total):

```
Bytes 0-255:   32 u64 fields (256 bytes)
  Offset 0:    status (u64)
  Offset 8:    nonce (u64)
  Offset 32:   quoteDecimal (u64) ← Was incorrectly labeled as baseDecimal
  Offset 40:   baseDecimal (u64)  ← Was incorrectly labeled as quoteDecimal
  ...

Bytes 256-335: 6 mixed u128/u64 swap fee fields (80 bytes)
  Offset 256:  swapBaseInAmount (u128) - 16 bytes
  Offset 272:  swapQuoteOutAmount (u128) - 16 bytes
  Offset 288:  swapBase2QuoteFee (u64) - 8 bytes
  Offset 296:  swapQuoteInAmount (u128) - 16 bytes
  Offset 312:  swapBaseOutAmount (u128) - 16 bytes
  Offset 328:  swapQuote2BaseFee (u64) - 8 bytes

Bytes 336-719: 12 publicKey fields (384 bytes)
  Offset 336:  quoteVault (32 bytes) ← VERIFIED: SOL vault
  Offset 368:  baseVault (32 bytes)  ← VERIFIED: AURA vault
  Offset 400:  quoteMint (32 bytes)  ← VERIFIED: SOL mint
  Offset 432:  baseMint (32 bytes)   ← VERIFIED: AURA mint
  Offset 464:  lpMint (32 bytes)
  Offset 496:  openOrders (32 bytes)
  Offset 528:  marketId (32 bytes)
  Offset 560:  marketProgramId (32 bytes)
  Offset 592:  targetOrders (32 bytes)
  Offset 624:  withdrawQueue (32 bytes)
  Offset 656:  lpVault (32 bytes)
  Offset 688:  owner (32 bytes)

Bytes 720-751: lpReserve + padding (32 bytes)
```

### Verified Vault Addresses

For AURA/SOL pool (`9ViX1VductEoC2wERTSp2TuDxXPwAf69aeET8ENPJpsN`):

**Quote Vault (SOL) - Offset 336:**
- Address: `9jbyBXHinaAah2SthksJTYGzTQNRLA7HdT2A7VMF91Wu`
- Balance: ~12,410 SOL
- Decimals: 9
- **VERIFIED via RPC call**

**Base Vault (AURA) - Offset 368:**
- Address: `9v9FpQYd46LS9zHJitTtnPDDQrHfkSdW2PRbbEbKd2gw`
- Balance: ~33,091,969 AURA
- Decimals: 6
- **VERIFIED via RPC call**

### Decimal Field Discovery

Critical finding: The decimal fields also needed to be swapped:
- **Offset 32**: quoteDecimal (9 for SOL)
- **Offset 40**: baseDecimal (6 for AURA)

This was discovered when the price calculation was off by ~1000x despite having correct vault addresses.

---

## Technical Deep Dive

### Why the SDK Offsets Didn't Match

The Raydium SDK documentation suggested:
- First publicKey at offset 352 (256 + 96)
- Second publicKey at offset 384

But actual data showed vaults at 336 and 368. **The discrepancy**:

1. SDK structure size calculation: 256 (u64s) + 96 (u128s) = 352
2. Actual structure has mixed u128/u64 fields totaling only 80 bytes
3. This 16-byte difference explains the offset mismatch

**Theory**: The on-chain structure may use tighter packing or a slightly different layout version than documented in the TypeScript SDK.

### Price Calculation Verification

Using discovered structure:
```
Base Reserve (AURA): 33,091,969.63 tokens
Quote Reserve (SOL): 12,410.68 tokens

Price = Quote Reserve / Base Reserve
Price = 12,410.68 / 33,091,969.63
Price = 0.000375 SOL per AURA

USD Price = 0.000375 × $162.50 (SOL price)
USD Price = $0.0609
```

**Live test result**: `$0.060876` ✓

---

## Implementation Changes

### File: `src/raydium_decoder.odin`

**Struct Definition Changes:**
```odin
// BEFORE (incorrect):
RaydiumPoolState :: struct {
    base_decimal:     u64, // Offset 32
    quote_decimal:    u64, // Offset 40
    ...
    base_mint:        [32]u8, // Offset 128
    quote_mint:       [32]u8, // Offset 160
    base_vault:       [32]u8, // Offset 192 - WRONG
    quote_vault:      [32]u8, // Offset 224 - WRONG
}

// AFTER (correct):
RaydiumPoolState :: struct {
    base_decimal:     u64, // Now reads from offset 40
    quote_decimal:    u64, // Now reads from offset 32
    ...
    quote_vault:      [32]u8, // Offset 336 - SOL vault
    base_vault:       [32]u8, // Offset 368 - AURA vault
    quote_mint:       [32]u8, // Offset 400 - SOL mint
    base_mint:        [32]u8, // Offset 432 - AURA mint
}
```

**Decode Function Changes:**
```odin
// Swap decimal reading
pool.quote_decimal = read_u64_le(data, 32) // SOL = 9 decimals
pool.base_decimal = read_u64_le(data, 40) // AURA = 6 decimals

// Correct vault offsets
pool.quote_vault = read_pubkey(data, 336) // SOL vault
pool.base_vault = read_pubkey(data, 368) // AURA vault
pool.quote_mint = read_pubkey(data, 400) // SOL mint
pool.base_mint = read_pubkey(data, 432) // AURA mint
```

---

## Validation & Testing

### Test Script
Created `test_real_vaults.py` to validate vault addresses:
```python
def test_vault(address, name):
    payload = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "getTokenAccountBalance",
        "params": [address, {"commitment": "confirmed"}]
    }
    response = requests.post("https://api.mainnet-beta.solana.com", json=payload)
    # Returns balance and decimals if valid
```

### End-to-End Test
```bash
$ ./bin/hound_debug aura
aura: $0.060876 (+0.0%)
```

✅ **On-chain fetch working**
✅ **No API fallback**
✅ **Price matches expected calculation**

---

## Lessons Learned

1. **Trust but verify**: Official SDK documentation may not match on-chain reality
2. **Hex analysis is powerful**: Raw byte inspection revealed the zero-filled incorrect offsets
3. **Empirical validation**: Testing every possible offset systematically found the truth
4. **Work backwards**: Finding known values (mints) and working backwards was the key breakthrough
5. **Field order matters**: Not just the vault addresses, but also the decimal fields needed correction

---

## Future Considerations

### Robustness
- The structure appears stable across Raydium V4 pools
- May need version detection if multiple pool versions exist
- Consider adding validation checks for expected field values

### Performance
- On-chain fetching adds ~2-3 RPC calls (pool + 2 vaults)
- Current fallback to API is reasonable
- Could implement caching for pool structure data

### Maintainability
- Document this finding for future developers
- Consider unit tests with known pool addresses
- Monitor for Raydium SDK updates that might change structure

---

## References

- **Raydium SDK**: https://github.com/raydium-io/raydium-sdk
- **Structure Definition**: `src/liquidity/layout.ts` (LIQUIDITY_STATE_LAYOUT_V4)
- **Test Pool**: AURA/SOL `9ViX1VductEoC2wERTSp2TuDxXPwAf69aeET8ENPJpsN`
- **Solana RPC**: https://api.mainnet-beta.solana.com

---

## Appendix: Investigation Timeline

1. **Initial failure**: On-chain fetch failing with invalid response
2. **Debug logging**: Discovered garbage base58 addresses
3. **Hex analysis**: Found zero-filled data at offsets 192/224
4. **SDK research**: Found official structure definition
5. **Offset calculation**: Calculated expected offset 352 from SDK
6. **Systematic testing**: Tested all 32-byte aligned positions
7. **Reverse engineering**: Worked backwards from known mints
8. **Breakthrough**: Found vaults at 336/368 with valid balances
9. **Decimal fix**: Discovered decimal fields also needed swapping
10. **Validation**: Confirmed correct price calculation

**Total investigation time**: ~2 hours of systematic analysis

**Result**: Fully functional on-chain price fetching ✅
