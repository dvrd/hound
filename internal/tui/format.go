package tui

// TruncateAddress returns the first 4 and last 4 characters of addr joined
// by "...", or addr unchanged if it is 11 characters or fewer.
func TruncateAddress(addr string) string {
	if len(addr) <= 11 {
		return addr
	}
	return addr[:4] + "..." + addr[len(addr)-4:]
}

// Truncate shortens s to at most max characters. If s exceeds max, the last
// character is replaced with "~" to signal truncation.
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "~"
}
