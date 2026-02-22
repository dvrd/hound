---
date: 2026-02-22
topic: "Batch 2 — Make It Safe (Security Hardening)"
status: validated
---

# Batch 2 — "Make It Safe" Security Hardening

## Problem Statement

The Batch 1 audit revealed 6 security issues that must be fixed before Hound handles real funds. The most critical: the stored `password_hash` in the database IS the AES-256-GCM encryption key — anyone who reads the SQLite file can decrypt every private key instantly.

## Constraints

- No CGO — pure Go only (no SQLCipher)
- Must migrate existing encrypted keypairs without data loss
- Backward-compatible unlock: existing wallets work after upgrade
- All 24 test packages must continue passing
- Zero-downtime migration (happens on next unlock, not at startup)

## Fixes (6 total)

### Fix 1 — C1: Separate Password Hash from Encryption Key (CRITICAL)

**Problem:** `HashPassword()` is an alias for `DeriveKey()` — same salt, same params, same 32-byte output. The `password_hash` BLOB stored in `encrypted_keypairs` IS the AES key.

**Solution:** Dual-salt derivation:
1. `encryption_salt` (existing `salt` column) → `DeriveKey(pw, encryption_salt)` → AES-256-GCM key (never stored)
2. `verifier_salt` (new column) → `DeriveKey(pw, verifier_salt)` → stored as `password_hash` (useless for decryption)

**Schema change:** `ALTER TABLE encrypted_keypairs ADD COLUMN verifier_salt BLOB`
**Schema change:** `ALTER TABLE hyperliquid_wallets ADD COLUMN verifier_salt BLOB`

**Migration flow (on next unlock):**
1. Check if `verifier_salt IS NULL` for this keypair
2. If NULL → old format: derive key using old single-salt method, decrypt, verify
3. Generate new `verifier_salt`, derive new `password_hash` from it
4. Re-derive encryption key with new Argon2 params (H7)
5. Re-encrypt private key with new key
6. Update row: set `verifier_salt`, new `password_hash`, new `salt`, new `encrypted_private_key`, new `nonce`, new `tag`
7. If NOT NULL → new format: derive verifier from `verifier_salt`, compare to `password_hash`, then derive AES key from `salt`

**Files:** `keystore/argon2.go`, `database/database.go`, `database/wallets.go`, `services/keystore.go`

### Fix 2 — H7: Bump Argon2 Parameters

**Problem:** Current params (19 MiB / 2 iterations / 1 thread) are OWASP floor — too weak for a crypto wallet.

**Solution:** Bump to 64 MiB / 3 iterations / 4 threads. New keys use new params immediately. Existing keys upgraded during C1 migration.

**Versioning:** Add `argon2_version TINYINT DEFAULT 1` column to both tables. Version 1 = old params, version 2 = new params. On unlock, check version and use correct params for verification, then upgrade.

**Files:** `keystore/argon2.go`

### Fix 3 — H4: SQLite Single Connection

**Problem:** `sql.DB` pools connections. PRAGMAs (`foreign_keys=ON`, `journal_mode=WAL`) are per-connection and won't apply to new pool connections.

**Solution:** Call `db.SetMaxOpenConns(1)` immediately after `sql.Open()`. SQLite is single-writer anyway — this guarantees all queries use the same connection with our PRAGMAs.

**Files:** `database/database.go`

### Fix 4 — H5: Mnemonic as []byte

**Problem:** `strings.Join(words, " ")` creates an immutable Go `string` that cannot be zeroed. The mnemonic persists in memory until GC.

**Solution:**
1. Add `JoinWordsToBytes(words []string) []byte` in `keystore/secure.go` — builds mnemonic as mutable `[]byte`
2. Replace `strings.Join` calls in `bip39.go` and `keypair.go` with `JoinWordsToBytes`
3. `defer ZeroBytes(mnemonicBytes)` immediately after creation
4. Zero `entropy` bytes in `GenerateMnemonic` after use
5. Zero `seedBytes` in `MnemonicToSeed` after copying to fixed array

**Limitation:** The `tyler-smith/go-bip39` library accepts `string` internally. We must convert `[]byte` → `string` at the library boundary. This is unavoidable without forking the library. We minimize exposure by zeroing our own copies.

**Files:** `keystore/secure.go`, `keystore/bip39.go`, `keystore/keypair.go`

### Fix 5 — M14: Prevent ZeroBytes Optimization

**Problem:** Go compiler's dead-store elimination can remove the zeroing loop if the slice isn't read afterward.

**Solution:**
1. Add `//go:noinline` directive to `ZeroBytes` function
2. Also zero the intermediate `raw` slice in `DeriveKey` after copying to fixed array

**Files:** `keystore/secure.go`, `keystore/argon2.go`

### Fix 6 — M6: Clear Passwords in TUI Models

**Problem:** `passwordInput.Value()` returns a `string` (immutable, can't zero). The textinput internal buffer retains the password indefinitely.

**Solution:**
1. In `send.go`: Call `m.passwordInput.Reset()` after extracting password value in `updatePassword`
2. In `walletimport.go`: Call `m.passwordInput.Reset()`, `m.confirmPwInput.Reset()`, `m.seedInput.Reset()` after use
3. In `walletimport.go`: Set `m.password = ""` and `m.words = nil` on navigation away (Escape/back)
4. In `send.go`: Set password-related fields to zero values on navigation away

**Limitation:** Go strings are immutable — we can't zero the extracted `string` value. We can only minimize the window by clearing input buffers immediately and clearing struct fields on exit.

**Files:** `tui/views/send/send.go`, `tui/views/walletimport/walletimport.go`

## Data Flow (C1 Migration)

```
UNLOCK (existing wallet, verifier_salt IS NULL):
  1. Read row: salt, encrypted_private_key, nonce, tag, password_hash (= old AES key)
  2. old_key = DeriveKeyV1(password, salt)  // old params: 19MiB/2/1
  3. Verify: old_key == password_hash        // old broken check
  4. Decrypt: plaintext = AES-GCM-Open(old_key, nonce, encrypted_private_key, tag)
  5. Generate: new_encryption_salt, new_verifier_salt
  6. new_key = DeriveKeyV2(password, new_encryption_salt)  // new params: 64MiB/3/4
  7. new_hash = DeriveKeyV2(password, new_verifier_salt)
  8. Re-encrypt: new_ciphertext, new_nonce, new_tag = AES-GCM-Seal(new_key, plaintext)
  9. UPDATE row: salt=new_encryption_salt, verifier_salt=new_verifier_salt,
     password_hash=new_hash, encrypted_private_key=new_ciphertext,
     nonce=new_nonce, tag=new_tag, argon2_version=2
  10. ZeroBytes(plaintext), ZeroBytes(old_key), ZeroBytes(new_key)

UNLOCK (new wallet, verifier_salt IS NOT NULL):
  1. Read row: salt, verifier_salt, encrypted_private_key, nonce, tag, password_hash, argon2_version
  2. check_hash = DeriveKeyV2(password, verifier_salt)
  3. Verify: check_hash == password_hash  // safe — hash ≠ key
  4. key = DeriveKeyV2(password, salt)
  5. Decrypt: plaintext = AES-GCM-Open(key, nonce, encrypted_private_key, tag)
  6. ZeroBytes(key)
  7. Return plaintext
```

## Error Handling

- **Migration failure:** If re-encryption fails mid-way, the old row is untouched (we only UPDATE after successful re-encrypt). The next unlock attempt retries migration.
- **Wrong password:** Detected at step 3 (old format) or step 3 (new format) before any decryption attempt.
- **Corrupt data:** AES-GCM authentication tag check catches corruption at decrypt time.

## Testing Strategy

- **Unit tests for dual-salt derivation:** Verify DeriveKey with different salts produces different keys
- **Unit tests for migration:** Create old-format row, unlock, verify row is upgraded to new format
- **Unit tests for Argon2 V1 vs V2:** Verify old params and new params produce different outputs for same input
- **Unit tests for JoinWordsToBytes:** Verify output matches strings.Join, verify zeroing works
- **Unit tests for ZeroBytes noinline:** Verify bytes are actually zeroed (limited — can't test compiler behavior, but can test runtime behavior)
- **Integration test:** Import wallet with new code, unlock, verify private key is correct
- **All 24 existing test packages must pass**

## Open Questions

None — all decisions are made. Proceeding to implementation plan.
