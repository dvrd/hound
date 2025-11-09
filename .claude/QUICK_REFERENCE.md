# Hound Development Quick Reference
## TigerBeetle-Inspired Principles

> **Core Mantra**: Safety → Performance → Developer Experience

---

## 🛡️ Safety Checklist

### Every Function Must Have:
- [ ] **≥2 assertions** (arguments, returns, invariants)
- [ ] **Explicit error handling** (no ignored errors)
- [ ] **≤70 lines** (split if longer)
- [ ] **Explicit types** (`u64` not `usize`)
- [ ] **No recursion** (use bounded loops)

### Example:
```odin
fetch_price :: proc(token: Token) -> (PriceData, ErrorType) {
    assert(len(token.symbol) > 0)  // Assertion 1
    assert(len(token.pools) > 0)   // Assertion 2

    if len(token.pools) == 0 {
        return {}, .TokenNotConfigured  // Explicit error
    }

    result := calculate_price(token)
    assert(result.price_usd >= 0)  // Assertion 3
    return result, .None
}
```

---

## ⚡ Performance Checklist

### Design Phase:
- [ ] Back-of-envelope calculation done?
- [ ] Slowest resource identified? (Network → Disk → Memory → CPU)
- [ ] Within 90% of theoretical maximum?

### Implementation:
- [ ] Batching where possible?
- [ ] Static allocation used?
- [ ] Hot loops extracted?
- [ ] Unnecessary abstraction removed?

### Quick Math:
```
Network:  50-200ms per RPC call  ← BOTTLENECK
Disk:     <1ms      config read
Memory:   <1µs      pool decode
CPU:      <1ns      per cycle
```

---

## 📝 Code Style Quick Guide

### Naming:
```odin
// Functions: snake_case
fetch_onchain_price :: proc() {}

// Variables: snake_case + units
latency_ms_max := 1000
price_usd := 0.061

// Types: PascalCase
TokenConfig :: struct {}

// Acronyms: Proper caps
RPCConnection :: struct {}
```

### Assertions:
```odin
// ✅ DO: Split compound assertions
assert(a > 0)
assert(b < 100)

// ❌ DON'T: Combine
assert(a > 0 && b < 100)
```

### Errors:
```odin
// ✅ DO: Explicit propagation
result, err := fetch_data()
if err != .None {
    return {}, err
}

// ❌ DON'T: Ignore
result, _ := fetch_data()  // Dangerous!
```

### Off-by-One Prevention:
```odin
// ✅ DO: Clear semantics
token_index := 0     // 0-based
token_count := 10    // 1-based
buffer_size := 752   // bytes

// ✅ DO: Explicit division
exact := @divExact(a, b)
floor := @divFloor(a, b)
ceil := div_ceil(a, b)

// ✅ DO: Avoid negation
if index < length {  // Clear
    // ...
}
```

---

## 🧪 Testing Checklist

### Every Test Must Have:
- [ ] **DOCUMENTATION comment** explaining what/why
- [ ] **Real-world example** with actual values
- [ ] **≥2 assertions** verifying correctness
- [ ] **Clear error messages** in assertions
- [ ] **Edge cases covered** (zero, max, negative)

### Template:
```odin
@(test)
test_feature_description :: proc(t: ^testing.T) {
    // DOCUMENTATION: [What this tests]
    //
    // [Why it matters]
    //
    // Example: [Real-world scenario]

    // Arrange
    input := create_test_data()

    // Act
    result := function_under_test(input)

    // Assert (≥2)
    testing.expect(t, result.success,
        "Operation should succeed")
    testing.expect(t, result.value > 0,
        fmt.tprintf("Expected positive value, got %v", result.value))
}
```

---

## 📐 Hound-Specific Constants

### Critical Offsets (DO NOT CHANGE without verification):
```odin
RAYDIUM_POOL_SIZE       :: 752  // Verified via RPC
QUOTE_DECIMAL_OFFSET    :: 32   // SOL = 9 decimals
BASE_DECIMAL_OFFSET     :: 40   // AURA = 6 decimals
QUOTE_VAULT_OFFSET      :: 336  // Reverse-engineered
BASE_VAULT_OFFSET       :: 368  // Reverse-engineered
QUOTE_MINT_OFFSET       :: 400  // Verified via known address
BASE_MINT_OFFSET        :: 432  // Verified via known address
```

### Performance Targets:
```
Config load:      <10ms   (actual: ~2ms)    ✅
API fetch:        <500ms  (actual: ~200ms)  ✅
On-chain fetch:   <1000ms (actual: ~400ms)  ✅
Total response:   <1s     (actual: ~400ms)  ✅
```

---

## 🚫 Never Do This

### ❌ Recursion:
```odin
// ❌ BAD
fetch_recursive :: proc(n: int) -> int {
    if n == 0 { return 0 }
    return fetch_recursive(n - 1)  // Can overflow!
}

// ✅ GOOD
fetch_iterative :: proc(n: int) -> int {
    assert(n < MAX_ITERATIONS)
    result := 0
    for i in 0..<n {
        result += i
    }
    return result
}
```

### ❌ Ignored Errors:
```odin
// ❌ BAD
data, _ := fetch_data()  // Error ignored!

// ✅ GOOD
data, err := fetch_data()
if err != .None {
    return {}, err
}
```

### ❌ Dynamic Allocation in Hot Path:
```odin
// ❌ BAD
fetch_loop :: proc() {
    for i in 0..<1000 {
        buffer := make([]u8, 752)  // Allocates every iteration!
        // ...
        delete(buffer)
    }
}

// ✅ GOOD
fetch_loop :: proc() {
    buffer: [752]u8  // Allocated once
    for i in 0..<1000 {
        // Reuse buffer
    }
}
```

### ❌ TODOs in Main Branch:
```odin
// ❌ BAD
fetch_price :: proc() {
    // TODO: Handle error case
    result, _ := get_data()
    return result
}

// ✅ GOOD
fetch_price :: proc() {
    result, err := get_data()
    if err != .None {
        return {}, err
    }
    return result, .None
}
```

---

## 📊 Pre-Commit Checklist

Before pushing code:

### Code Quality:
- [ ] `odin fmt` run on all files
- [ ] No compilation warnings
- [ ] All tests pass (`task test`)
- [ ] ≥2 assertions per function
- [ ] All errors handled

### Documentation:
- [ ] Test includes DOCUMENTATION comment
- [ ] Complex logic has rationale comments
- [ ] Commit message explains "why" not "what"
- [ ] Updated relevant docs if needed

### Performance:
- [ ] No obvious performance regressions
- [ ] Hot paths use static allocation
- [ ] Batching where applicable

### Safety:
- [ ] No recursion introduced
- [ ] No ignored errors
- [ ] Bounds checked
- [ ] Types explicit

---

## 🔍 Code Review Focus

When reviewing code, check for:

1. **Safety violations** (highest priority)
   - Missing assertions
   - Ignored errors
   - Unbounded loops

2. **Performance issues** (medium priority)
   - Dynamic allocation in loops
   - Missing batching opportunities
   - Unnecessary abstractions

3. **Style consistency** (lowest priority)
   - Naming conventions
   - Comment quality
   - Line length

---

## 📚 Quick Links

- **Full Philosophy**: `.claude/DEVELOPMENT_PHILOSOPHY.md`
- **Test Guide**: `tests/README.md`
- **Reverse Engineering**: `RAYDIUM_REVERSE_ENGINEERING.md`
- **TigerBeetle TIGER_STYLE**: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md

---

## 💡 Remember

> "Simple systems are faster than fancy ones." - Jim Gray

**Zero Technical Debt**: Fix problems during design/implementation, never defer to production.

**The Three Pillars**: Safety → Performance → Developer Experience (in that order)

---

## Version

- **v1.0** (2025-11-09): Initial quick reference
