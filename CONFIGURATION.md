# Configuration Guide

Discordo uses a TOML configuration file to customize behavior, keybindings, and appearance. This document provides comprehensive documentation of all available options.

## Configuration File Location

The configuration file is automatically created at:

- **Unix**: `$XDG_CONFIG_HOME/discordo/config.toml` or `$HOME/.config/discordo/config.toml`
- **macOS**: `$HOME/Library/Application Support/discordo/config.toml`
- **Windows**: `%AppData%/discordo/config.toml`

If no configuration file exists, Discordo uses built-in defaults. You can find the default configuration in [`internal/config/config.toml`](../internal/config/config.toml).

## Command-Line Options

Discordo supports several command-line options that override configuration:

```bash
discordo [flags]

Flags:
  -token string
        Authentication token (can also be set via DISCORDO_TOKEN environment variable)
  -config-path string
        Path to configuration file (default: OS-specific location)
  -log-path string
        Path to log file (default: OS-specific location)
  -log-level string
        Log level: debug, info, warn, error (default: "info")
```

## Configuration Sections

### General Settings

```toml
# Enable mouse support
mouse = true

# External editor for message composition
# Set to "default" to use $EDITOR environment variable
editor = "default"

# Discord presence status
# Options: "default", "online", "dnd", "idle", "invisible", "offline"
status = "default"

# Enable markdown rendering in messages
markdown = true

# Hide messages from blocked users
hide_blocked_users = true

# Show attachment links in messages
show_attachment_links = true

# Number of autocomplete suggestions for mentions
# Set to 0 to disable autocomplete list (tab completion still works)
autocomplete_limit = 20

# Number of messages to fetch when selecting a channel
# Range: 0-100
messages_limit = 50
```

### Timestamps

```toml
[timestamps]
enabled = true
# Time format using Go's time layout format
# See: https://pkg.go.dev/time#Layout
format = "3:04PM"
```

### Notifications

```toml
[notifications]
enabled = true
# Display duration in seconds (0 = system default of 30 seconds)
duration = 0

[notifications.sound]
enabled = true
# Only play sound for mentions/pings
only_on_ping = true
```

### Tip.cc Integration

```toml
[tipcc]
# Automatically claim airdrops
auto_claim = false
# Enable debug logging
debug = true
# Delay between operations in milliseconds
delay_ms = 100
```

## Keybindings

### Global Shortcuts

```toml
[keys]
# Focus specific panels
focus_guilds_tree = "Ctrl+G"
focus_messages_list = "Ctrl+T"
focus_message_input = "Ctrl+Space"

# Navigation
focus_previous = "Ctrl+H"
focus_next = "Ctrl+L"

# UI toggles
toggle_guilds_tree = "Ctrl+B"
clear_notification = "Ctrl+N"

# Actions
quit = "Ctrl+C"
logout = "Ctrl+D"  # Removes token from keyring
```

### Guilds Tree

```toml
[keys.guilds_tree]
select_previous = "Rune[k]"
select_next = "Rune[j]"
select_first = "Rune[g]"
select_last = "Rune[G]"
select_current = "Enter"  # Open channel or expand guild
yank_id = "Rune[i]"       # Copy channel ID
collapse_parent_node = "Rune[-]"
move_to_parent_node = "Rune[p]"
```

### Messages List

```toml
[keys.messages_list]
# Navigation
select_previous = "Rune[k]"
select_next = "Rune[j]"
select_first = "Rune[g]"
select_last = "Rune[G]"

# Message actions
select_reply = "Rune[s]"     # View message reference
reply = "Rune[r]"            # Reply to message
reply_mention = "Rune[R]"    # Reply with mention
edit = "Rune[e]"             # Edit own message
delete = "Rune[D]"           # Delete own message (confirmation)
delete_confirm = "Rune[d]"   # Confirm deletion
cancel = "Esc"

# Content actions
open = "Rune[o]"             # Open attachments/links
yank_content = "Rune[y]"     # Copy message content
yank_url = "Rune[u]"         # Copy message URL
yank_id = "Rune[i]"          # Copy message ID
```

### Message Input

```toml
[keys.message_input]
send = "Enter"
cancel = "Esc"               # Clear text or cancel reply
paste = "Ctrl+V"             # Paste text/images from clipboard
tab_complete = "Tab"         # Complete usernames
open_editor = "Ctrl+E"       # Open external editor
open_file_picker = "Ctrl+\\" # Open file picker for attachments

# Alt+Enter automatically inserts a new line
```

### Mentions List

```toml
[keys.mentions_list]
up = "Ctrl+P"
down = "Ctrl+N"
```

## Theme Configuration

### Title Bar

```toml
[theme.title]
alignment = "left"  # "left", "center", or "right"

normal_style = { attributes = "dim" }
active_style = { foreground = "green", attributes = "bold" }
```

### Borders

```toml
[theme.border]
enabled = true
# Padding: [top, bottom, left, right]
padding = [0, 0, 1, 1]

normal_style = { attributes = "dim" }
active_style = { foreground = "green", attributes = "bold" }

# Border styles: "hidden", "plain", "round", "thick", "double"
normal_set = "round"
active_set = "round"
```

### Guilds Tree

```toml
[theme.guilds_tree]
auto_expand_folders = true
graphics = true  # Tree-like structure with lines
graphics_color = "default"

# Width proportion (3 = ~30% of screen width)
# Common values: 2 (~22%), 3 (~30%), 4 (~40%)
width_proportion = 3
min_width = 25  # Minimum width in characters
```

### Messages List

```toml
[theme.messages_list]
reply_indicator = ">"
forwarded_indicator = "<"

# Message element styling
mention_style = { foreground = "blue" }
emoji_style = { foreground = "green" }
url_style = { foreground = "blue" }
attachment_style = { foreground = "yellow" }
```

### Mentions List

```toml
[theme.mentions_list]
min_width = 20   # Minimum width (0 = auto)
max_height = 0   # Maximum height (0 = auto)
```

## Style Format

Style objects use the following format:

```toml
style = { 
    foreground = "color",    # Text color
    background = "color",    # Background color
    attributes = "attr"      # Single attribute or ["attr1", "attr2"]
}
```

### Available Colors

- Standard colors: `black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `white`
- Bright variants: `bright_black`, `bright_red`, etc.
- Terminal colors: `default` (uses terminal default)

### Available Attributes

- `bold`
- `blink`
- `reverse` (swap foreground/background)
- `underline`
- `dim`

## Environment Variables

- `DISCORDO_TOKEN`: Authentication token (alternative to `--token` flag)
- `EDITOR`: Default editor for message composition

## Examples

### Minimal Configuration

```toml
mouse = true
markdown = true
status = "online"
```

### Custom Keybindings

```toml
[keys]
focus_guilds_tree = "Alt+1"
focus_messages_list = "Alt+2"
focus_message_input = "Alt+3"

[keys.messages_list]
reply = "Rune[a]"
edit = "Rune[e]"
delete = "Rune[d]"
```

### Dark Theme

```toml
[theme.title]
active_style = { foreground = "cyan", attributes = "bold" }

[theme.border]
active_style = { foreground = "cyan", attributes = "bold" }

[theme.messages_list]
mention_style = { foreground = "magenta" }
url_style = { foreground = "cyan" }
```

## Troubleshooting

### Configuration Not Loading

1. Verify the file path is correct for your OS
2. Check TOML syntax using an online validator
3. Ensure all required sections are present
4. Check log files for error messages

### Keybindings Not Working

1. Ensure key names are correctly formatted
2. Check for conflicts with global shortcuts
3. Some keys may be intercepted by your terminal
4. Try alternative key combinations

### Theme Issues

1. Verify color names are supported by your terminal
2. Check if your terminal supports TrueColor
3. Some attributes may not be supported in all terminals

For more help, see the [Contributing Guide](./CONTRIBUTING.md) or join our [Discord server](https://discord.com/invite/VzF9UFn2aB).