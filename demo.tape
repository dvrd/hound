# Hound demo — Solana wallet TUI v1.0.0
Output demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set Padding 20
Set PlaybackSpeed 1.0

# Launch TUI — wallets preload in background
Type "./bin/hound"
Enter
Sleep 4s

# Open first wallet → portfolio / walletstatus
Enter
Sleep 3s

# ── History ──────────────────────────────────────────────────────────────────

# Navigate to history
Type "h"
Sleep 3s

# Scroll down through transactions
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 600ms

# Scroll back up
Up
Sleep 400ms
Up
Sleep 400ms
Up
Sleep 800ms

# Back to walletstatus
Escape
Sleep 1.5s

# ── Token list ───────────────────────────────────────────────────────────────

# Navigate to token list from walletstatus
Type "t"
Sleep 2s

# Scroll through tokens
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 600ms
Up
Sleep 400ms
Up
Sleep 800ms

# ── Token detail ─────────────────────────────────────────────────────────────

# Open token detail for selected token
Enter
Sleep 5s

# Scroll down through detail (price chart / holders)
Down
Sleep 600ms
Down
Sleep 600ms
Down
Sleep 600ms
Down
Sleep 800ms

# Back to token list
Escape
Sleep 1s

# Back to walletstatus
Escape
Sleep 1s

# Quit
Ctrl+Q
Sleep 500ms
