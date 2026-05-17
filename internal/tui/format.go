package tui

import "unicode/utf8"

// PadRight pads s to exactly width runes, truncating with "~" if needed.
// Avoids fmt.Sprintf("%-*s", ...) allocation in hot render loops.
// Uses rune count for correct handling of multi-byte UTF-8 characters.
func PadRight(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		if n > width {
			// Truncate to width-1 runes and append ~.
			i := 0
			for r := 0; r < width-1; r++ {
				_, size := utf8.DecodeRuneInString(s[i:])
				i += size
			}
			return s[:i] + "~"
		}
		return s
	}
	// Pad with spaces. For ASCII-dominated content (common case),
	// padding bytes == padding runes.
	pad := width - n
	buf := make([]byte, len(s)+pad)
	copy(buf, s)
	for i := len(s); i < len(buf); i++ {
		buf[i] = ' '
	}
	return string(buf)
}

// PadLeft right-aligns s within a field of the given width (in runes).
// Avoids fmt.Sprintf("%*s", ...) allocation in hot render loops.
// Uses rune count for correct handling of multi-byte UTF-8 characters.
func PadLeft(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	pad := width - n
	buf := make([]byte, pad+len(s))
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
