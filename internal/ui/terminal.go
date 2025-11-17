package ui

import (
	"os"
	"strings"
)

// IsKittyTerminal checks if the current terminal is kitty
func IsKittyTerminal() bool {
	term := os.Getenv("TERM_PROGRAM")
	return strings.Contains(strings.ToLower(term), "kitty") || os.Getenv("KITTY_PID") != ""
}

// GetKittyCompatibleTerm returns a terminal type that's compatible with kitty
// to avoid visual artifacts in TUI applications
func GetKittyCompatibleTerm() string {
	// If we're in kitty, return a compatible TERM value
	if IsKittyTerminal() {
		term := os.Getenv("TERM")
		// If TERM is already set to a kitty-specific value, return as-is
		if strings.Contains(term, "kitty") {
			return term
		}
		// Otherwise, return a standard value that works well with kitty
		return "xterm-256color"
	}
	return os.Getenv("TERM")
}
