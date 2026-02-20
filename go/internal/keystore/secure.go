package keystore

// ZeroBytes overwrites every byte in the slice with zero.
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
