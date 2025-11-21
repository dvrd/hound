#+feature global-context
package utils

import "core:fmt"
import "core:log"
import "core:os"
import "core:strings"
import "core:sys/posix"

// read_password_secure reads a password from stdin with echo disabled
//
// Uses POSIX termios to temporarily disable terminal echo.
// This works on macOS, Linux, and other Unix-like systems.
//
// Returns: Password string (trimmed), bool indicating success
read_password_secure :: proc(prompt: string = "Password: ") -> (password: string, ok: bool) {
	// Print prompt
	fmt.print(prompt)

	// Get terminal file descriptor
	fd := posix.FD(os.stdin)

	// Get current terminal settings
	original_termios: posix.termios
	if posix.tcgetattr(fd, &original_termios) != .OK {
		log.error("Failed to get terminal attributes")
		return "", false
	}

	// Create modified termios with echo disabled
	new_termios := original_termios
	new_termios.c_lflag -= {.ECHO}   // Disable echo
	new_termios.c_lflag += {.ECHONL} // Still print newline

	// Apply modified settings
	if posix.tcsetattr(fd, .TCSANOW, &new_termios) != .OK {
		log.error("Failed to set terminal attributes")
		return "", false
	}

	// Ensure we restore original settings on exit
	defer {
		if posix.tcsetattr(fd, .TCSANOW, &original_termios) != .OK {
			log.error("Failed to restore terminal attributes")
		}
	}

	// Read password
	password_buffer: [256]byte
	n, read_err := os.read(os.stdin, password_buffer[:])
	if read_err != nil {
		log.errorf("Failed to read password: %v", read_err)
		// Zero buffer before returning
		for i := 0; i < len(password_buffer); i += 1 {
			password_buffer[i] = 0
		}
		return "", false
	}

	// Trim whitespace (newline, spaces, etc.)
	password = strings.trim_space(string(password_buffer[:n]))

	// Zero the buffer after extracting password
	for i := 0; i < len(password_buffer); i += 1 {
		password_buffer[i] = 0
	}

	return password, true
}

// read_password_with_confirmation prompts for password twice and validates match
//
// This is a convenience wrapper for secure password entry with confirmation.
// Automatically zeros sensitive data on mismatch.
//
// Parameters:
//   - prompt1: First password prompt (default: "Password: ")
//   - prompt2: Confirmation prompt (default: "Confirm password: ")
//
// Returns: Password string, bool indicating success
read_password_with_confirmation :: proc(
	prompt1: string = "Password: ",
	prompt2: string = "Confirm password: ",
) -> (password: string, ok: bool) {
	// First entry
	password1, ok1 := read_password_secure(prompt1)
	if !ok1 {
		return "", false
	}
	defer zero_string(&password1)

	// Second entry
	password2, ok2 := read_password_secure(prompt2)
	if !ok2 {
		return "", false
	}
	defer zero_string(&password2)

	// Compare
	if password1 != password2 {
		log.error("Passwords do not match")
		return "", false
	}

	// Return copy of password1
	return strings.clone(password1), true
}

// zero_string securely zeros a string's memory
//
// IMPORTANT: This only zeros the string buffer, not the underlying allocation.
// For true secure zeroing, use explicit byte arrays.
zero_string :: proc(s: ^string) {
	if s == nil || len(s^) == 0 {
		return
	}

	// Zero the string data
	data := transmute([]byte)s^
	for i := 0; i < len(data); i += 1 {
		data[i] = 0
	}
}
