# Hound v1.0.0 demo
Output demo.mp4

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set Padding 20
Set PlaybackSpeed 1.0

# Launch TUI silently
Hide
Sleep 2s
Type "/tmp/hound_launch.sh"
Enter
Sleep 8s
Show

# Wallet Status - portfolio overview
Sleep 2s

# Navigate to History
Type "h"
Sleep 2.5s

# Scroll through history
Type "jjj"
Sleep 500ms
Type "kk"
Sleep 1.5s

# Navigate to Token List
Type "t"
Sleep 1.5s

# Search for SPSC token
Type "SPSC"
Sleep 2s

# Open token detail (Enter with search result selected)
Enter
Sleep 3s

# Quit
Ctrl+Q
Sleep 1s
