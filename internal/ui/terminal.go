package ui

import (
	"log/slog"
	"os"
	"strings"
)

// TerminalInfo holds information about the current terminal
type TerminalInfo struct {
	Name              string
	IsKitty           bool
	SupportsTrueColor bool
	SupportsGraphics  bool
	Term              string
}

// DetectTerminal returns detailed information about the current terminal
func DetectTerminal() TerminalInfo {
	info := TerminalInfo{
		Term: os.Getenv("TERM"),
	}

	// Detect Kitty terminal through multiple environment variables
	kittyPid := os.Getenv("KITTY_PID")
	kittyWindowId := os.Getenv("KITTY_WINDOW_ID")
	termProgram := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	term := strings.ToLower(info.Term)

	info.IsKitty = kittyPid != "" ||
		kittyWindowId != "" ||
		strings.Contains(termProgram, "kitty") ||
		strings.Contains(term, "kitty")

	if info.IsKitty {
		info.Name = "kitty"
		info.SupportsTrueColor = true // Kitty always supports 24-bit color
		info.SupportsGraphics = true  // Kitty supports graphics protocol
	} else {
		info.Name = "unknown"
		// Check for truecolor support
		colorterm := strings.ToLower(os.Getenv("COLORTERM"))
		info.SupportsTrueColor = colorterm == "truecolor" || colorterm == "24bit"
	}

	return info
}

// IsKittyTerminal checks if the current terminal is kitty
func IsKittyTerminal() bool {
	return DetectTerminal().IsKitty
}

// GetOptimalTerm returns the optimal TERM value for the current terminal
// For Kitty: uses xterm-kitty if available, otherwise xterm-256color
func GetOptimalTerm() string {
	info := DetectTerminal()

	if info.IsKitty {
		term := os.Getenv("TERM")
		// Keep xterm-kitty if already set (tcell 2.x has good support)
		if strings.Contains(strings.ToLower(term), "kitty") {
			return term
		}
		// Fall back to xterm-256color only if needed for compatibility
		return "xterm-256color"
	}

	return os.Getenv("TERM")
}

// GetKittyCompatibleTerm returns a terminal type that's compatible with kitty
// Deprecated: Use GetOptimalTerm instead
func GetKittyCompatibleTerm() string {
	return GetOptimalTerm()
}

// ApplyKittyOptimizations applies optimal environment variables for Kitty terminal
// This should be called before tcell/tview initialization
func ApplyKittyOptimizations() {
	ApplyKittyOptimizationsWithConfig("auto", "", true)
}

// ApplyKittyOptimizationsWithConfig applies Kitty optimizations with config options
// trueColor: "auto", "enable", or "disable"
// forceTerm: override TERM if non-empty
// optimizeForKitty: whether to apply Kitty-specific optimizations
func ApplyKittyOptimizationsWithConfig(trueColor, forceTerm string, optimizeForKitty bool) {
	info := DetectTerminal()

	// Handle forced TERM override
	if forceTerm != "" {
		os.Setenv("TERM", forceTerm)
		slog.Debug("Forced TERM value", "term", forceTerm)
	}

	// Handle truecolor setting
	switch strings.ToLower(trueColor) {
	case "enable":
		os.Setenv("TCELL_TRUECOLOR", "enable")
		os.Setenv("COLORTERM", "truecolor")
		slog.Debug("Forced truecolor enabled")
	case "disable":
		os.Setenv("TCELL_TRUECOLOR", "disable")
		slog.Debug("Forced truecolor disabled")
	case "auto", "":
		// Let tcell auto-detect - don't set TCELL_TRUECOLOR
		// For Kitty, ensure COLORTERM is set so tcell detects it properly
		if info.IsKitty && os.Getenv("COLORTERM") == "" {
			os.Setenv("COLORTERM", "truecolor")
		}
	}

	// Apply Kitty-specific optimizations if enabled and running in Kitty
	if !optimizeForKitty {
		slog.Debug("Kitty optimizations disabled by config")
		return
	}

	if !info.IsKitty {
		slog.Debug("Terminal is not Kitty, skipping Kitty-specific optimizations")
		return
	}

	slog.Debug("Applying Kitty terminal optimizations",
		"term", info.Term,
		"kitty_pid", os.Getenv("KITTY_PID"),
		"kitty_window_id", os.Getenv("KITTY_WINDOW_ID"),
		"truecolor_setting", trueColor)

	// Set optimal TERM if current one might cause issues (and not overridden)
	if forceTerm == "" {
		currentTerm := os.Getenv("TERM")
		if currentTerm == "" || currentTerm == "linux" {
			os.Setenv("TERM", "xterm-256color")
		}
		// Keep "xterm-kitty" if already set - modern tcell handles it well
	}
}

// ApplyGeneralTerminalOptimizations applies optimizations for any terminal
func ApplyGeneralTerminalOptimizations() {
	// Ensure UTF-8 locale for proper Unicode support
	if os.Getenv("LC_ALL") == "" && os.Getenv("LANG") == "" {
		// Don't override if locale is set elsewhere
		lang := os.Getenv("LC_CTYPE")
		if lang == "" {
			// Terminal apps generally work best with UTF-8
			os.Setenv("LC_CTYPE", "en_US.UTF-8")
		}
	}
}
