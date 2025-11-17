# Kitty Terminal Visual Artifacts Fix

## Problem
Users of kitty terminal may experience visual artifacts when using TUI applications like discordo. These artifacts can include:
- Flickering or ghost characters
- Improper color rendering
- Display glitches during updates
- Visual corruption in borders or UI elements

## Solution
This fix addresses kitty terminal visual artifacts by implementing terminal-specific optimizations during application startup.

## Changes Made

### 1. Terminal Detection
Added `internal/ui/terminal.go` with functions to detect if the application is running in kitty terminal:
- `IsKittyTerminal()` - Checks if running in kitty
- `GetKittyCompatibleTerm()` - Returns appropriate TERM value

### 2. Environment Optimization
The application now sets specific environment variables when running in kitty:
- `TCELL_TRUECOLOR=disable` - Disables problematic truecolor handling
- `COLORTERM=truecolor` - Ensures proper color support detection
- Proper TERM value handling for compatibility

### 3. Application Initialization
Terminal optimizations are applied during startup in `cmd/root.go` before the TUI is initialized.

## How It Works
When discordo starts, it checks if it's running in kitty terminal by looking for:
- `TERM_PROGRAM` environment variable containing "kitty"
- `KITTY_PID` environment variable

If kitty is detected, it applies the necessary optimizations before initializing the TUI.

## Additional Configuration Tips

If you still experience issues, you can also try adding these settings to your kitty configuration file (`~/.config/kitty/kitty.conf`):

```
# Reduce visual artifacts in TUI applications
repaint_delay 10
input_delay 10
```

Or for more aggressive artifact reduction:
```
# For TUI applications with visual issues
repaint_delay 25
input_delay 25
```

## Benefits
- Reduces visual artifacts and flickering in kitty terminal
- Maintains full functionality of the application
- Only applies optimizations when running in kitty
- No impact on other terminal emulators