package tui

// PadRight pads s to exactly width characters, truncating with "~" if needed.
// Avoids fmt.Sprintf("%-*s", ...) allocation in hot render loops.
func PadRight(s string, width int) string {
	if len(s) >= width {
		if len(s) > width {
			return s[:width-1] + "~"
		}
		return s
	}
	// Pad with spaces.
	buf := make([]byte, width)
	copy(buf, s)
	for i := len(s); i < width; i++ {
		buf[i] = ' '
	}
	return string(buf)
}

// PadLeft right-aligns s within a field of the given width.
// Avoids fmt.Sprintf("%*s", ...) allocation in hot render loops.
func PadLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	pad := width - len(s)
	buf := make([]byte, width)
	for i := 0; i < pad; i++ {
		buf[i] = ' '
	}
	copy(buf[pad:], s)
	return string(buf)
}

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
