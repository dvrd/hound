# Hound demo — Solana wallet TUI
Output demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set Padding 20
Set PlaybackSpeed 1.0

# Show version
Type "./bin/hound version"
Enter
Sleep 1s

# Show help
Type "./bin/hound --help"
Enter
Sleep 1.5s

# Launch TUI
Type "./bin/hound"
Enter
Sleep 3s

# Navigate into first wallet
Enter
Sleep 3s

# Sort by symbol
Type "2"
Sleep 1s

# Sort by value
Type "1"
Sleep 1s

# Toggle show all tokens
Type "a"
Sleep 1s

# Toggle back
Type "a"
Sleep 1s

# Go back to wallet list
Escape
Sleep 1.5s

# Navigate to token list
Type "t"
Sleep 2s

# Go back
Escape
Sleep 1s

# Quit
Type "q"
Sleep 500ms
