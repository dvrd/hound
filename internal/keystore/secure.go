package keystore

// ZeroBytes overwrites every byte in the slice with zero.
// The //go:noinline directive prevents the compiler from optimizing away
// the zeroing loop via dead-store elimination (Fix M14).
//
//go:noinline
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ZeroSlice is a generic version for fixed-size arrays passed as slices.
// Same as ZeroBytes, provided as an alias for clarity.
func ZeroSlice(b []byte) {
	ZeroBytes(b)
}

// JoinWordsToBytes joins a slice of words with spaces into a mutable []byte.
// Unlike strings.Join which returns an immutable string, the returned []byte
// can be zeroed after use to clear the mnemonic from memory (Fix H5).
func JoinWordsToBytes(words []string) []byte {
	if len(words) == 0 {
		return []byte{}
	}

	// Calculate total length: sum of word lengths + (n-1) spaces
	totalLen := 0
	for _, w := range words {
		totalLen += len(w)
	}
	totalLen += len(words) - 1 // spaces between words

	buf := make([]byte, 0, totalLen)
	for i, w := range words {
		if i > 0 {
			buf = append(buf, ' ')
		}
		buf = append(buf, w...)
	}
	return buf
}
