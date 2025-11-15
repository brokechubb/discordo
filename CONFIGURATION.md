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
# Path to custom sound file (supports .wav, .mp3, .ogg, etc.)
# Leave empty to use system default beep
file = ""
```

**Sound Configuration:**
- `enabled`: Enable/disable notification sounds
- `only_on_ping`: Only play sounds for mentions/pings
- `file`: Path to custom sound file
  - Supported formats: `.mp3`, `.ogg` (Linux/Windows), `.wav`, `.aiff` (macOS)
  - Use absolute paths or relative paths from where you run discordo
  - Falls back to system default beep if file doesn't exist
```

### Tip.cc Integration

```toml
[tipcc]
# Automatically claim airdrops (only works in focused channel)
auto_claim = false
# Enable debug logging
debug = true
# Delay between operations in milliseconds
delay_ms = 100
```

**Important**: Tip.cc autoclaim only works when you have the specific channel focused where the drop occurs. This prevents accidental claims across multiple channels and gives you control over which drops to claim.

#### Tip.cc Usage

1. **Confirmation Dialogs**: Always automatically handled (no configuration needed)
2. **Drop Claiming**: Set `auto_claim = true` in the `[tipcc]` section for automatic drop claiming
3. **Focus Channel**: Navigate to and focus the channel where you expect drops (for drops only)
4. **Monitor Activity**: Keep the channel focused - drop claiming only works in the active channel
5. **Manual Claims**: For drops in unfocused channels, you'll need to manually click the claim buttons

#### Tip.cc Configuration Options

- **auto_claim**: Enable/disable automatic drop claiming (default: false) - confirmation dialogs always work
- **debug**: Show detailed logging for tip.cc detection (default: true)
- **delay_ms**: Delay before claiming to avoid race conditions (default: 100ms)

#### Automatic Confirmation Dialogs

Discordo automatically handles confirmation dialogs **independently of the autoclaim setting**:

- **Always Active**: Confirmation dialogs are always auto-clicked for better user experience
- **Smart Detection**: Identifies confirmation dialogs by button labels and message content
- **Confirm Button Auto-click**: Automatically clicks "Confirm", "Accept", "Agree", "Yes", etc.
- **Cancel Button Safety**: Never auto-clicks "Cancel", "Decline", "No", etc.
- **Pattern Recognition**: Detects various confirmation patterns like "Are you sure?", "Do you want to?", etc.
- **Content Analysis**: Checks both message content and embeds for confirmation indicators
- **Separate Control**: `auto_claim` setting only controls drop claiming, not confirmation dialogs

#### Tip.cc Status Indicator

Discordo displays a persistent Tip.cc status indicator in the top bar:
- **Green "Auto-claim ON"**: Autoclaim is enabled (focused channel only)
- **Red "Auto-claim OFF"**: Autoclaim is disabled
- **Toggle**: Press `Ctrl+A` to toggle autocaim on/off
- **Notification**: A temporary notification confirms the status change

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
toggle_tipcc_autoclaim = "Ctrl+A"  # Toggle Tip.cc auto-claim

# Actions
quit = "Ctrl+C"
logout = "Ctrl+D"  # Removes token from keyring
```

### Guilds Tree

```toml
[keys.guilds_tree]
# Navigation - uses arrow keys by default
select_previous = "Up"     # Previous guild/channel
select_next = "Down"       # Next guild/channel
select_first = "Home"      # First guild/channel
select_last = "End"        # Last guild/channel

# Actions
select_current = "Enter"   # Open channel or expand guild
yank_id = "Rune[i]"        # Copy channel ID
collapse_parent_node = "Rune[-]"
move_to_parent_node = "Rune[p]"
```

### Messages List

```toml
[keys.messages_list]
# Navigation - uses arrow keys by default
select_previous = "Up"       # Previous message
select_next = "Down"         # Next message
select_first = "Home"        # First message
select_last = "End"          # Last message

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

# Width proportion (2 = ~22% of screen width)
# Common values: 2 (~22%), 3 (~30%), 4 (~40%)
width_proportion = 2
min_width = 25  # Minimum width in characters
```

### Messages List

```toml
[theme.messages_list]
reply_indicator = ">"
forwarded_indicator = "<"
dm_user_color = "green"  # Color for usernames in DM conversations

# Message element styling
mention_style = { foreground = "blue" }
emoji_style = { foreground = "green" }
url_style = { foreground = "blue" }
attachment_style = { foreground = "yellow" }
```

**DM User Color:**
- `dm_user_color`: Color used for usernames in direct messages and group DMs
- Default: `"green"` for better visibility across terminal themes
- Can be set to any valid tview/tcell color name (see [Available Colors](#available-colors))
- This color is used when no role colors are available (DMs don't have guild roles)

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

## Upgrading and Updating

### Keeping Discordo Updated

Discordo is actively developed with frequent updates. Here's how to stay current:

#### Method 1: Building from Source (Recommended)

```bash
# Navigate to your discordo directory
cd discordo

# Pull the latest changes
git pull origin tip-cc-autoclaim

# Rebuild the application
go build .

# Run the updated version
./discordo
```

#### Method 2: Fresh Clone

```bash
# Backup your configuration (optional)
cp ~/.config/discordo/config.toml ~/.config/discordo/config.toml.backup

# Remove old directory and clone fresh
rm -rf discordo
git clone -b tip-cc-autoclaim https://github.com/ayn2op/discordo
cd discordo
go build .

# Restore your configuration if needed
cp ../config.toml.backup ~/.config/discordo/config.toml
```

### Configuration Migration

When updating Discordo, your configuration file should continue to work. However, new features may add additional configuration options.

#### Automatic Migration

- New configuration options use built-in defaults
- Existing settings remain unchanged
- No manual intervention required for most updates

#### Manual Updates

Some updates may require manual configuration changes:

```toml
# Example: Adding new DM user color setting (added in recent update)
[theme.messages_list]
# Add this line if you want to customize DM user colors
dm_user_color = "green"  # Default is "green"
```

### Version-Specific Migration

#### Recent Changes Requiring Attention

**DM User Coloring Feature**:
- **New setting**: `dm_user_color` in `[theme.messages_list]` section
- **Default value**: `"green"` (automatically applied)
- **Purpose**: Colors usernames in DM and group DM conversations for better visibility
- **Action required**: None - feature works automatically with default color
- **Optional customization**: Add the setting only if you want to change the color

#### Customizing DM User Colors

If you want to change the default DM user color from green to something else:

```toml
[theme.messages_list]
# Change DM username color to any valid terminal color
dm_user_color = "cyan"     # Light blue
dm_user_color = "magenta"  # Purple
dm_user_color = "yellow"   # Yellow
dm_user_color = "white"    # White
dm_user_color = "red"      # Red
```

**Available colors**: `black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `white`, plus bright variants like `bright_blue`, `bright_green`, etc.

#### Migration Examples

**Before update** (no DM user coloring):
```toml
[theme.messages_list]
reply_indicator = ">"
mention_style = { foreground = "blue" }
```

**After update** (automatic - no changes needed):
- DM usernames will appear in green color automatically
- Existing configuration continues to work unchanged

**With customization** (optional):
```toml
[theme.messages_list]
reply_indicator = ">"
dm_user_color = "cyan"  # Custom color for DM usernames
mention_style = { foreground = "blue" }
```

**Navigation Changes**:
- Arrow keys are now default (replaced vim-style j/k)
- Old vim-style keys are deprecated but may still work
- Action: Update muscle memory to use arrow keys

### Backup and Restore

#### Before Updating

```bash
# Backup configuration
cp ~/.config/discordo/config.toml ~/.config/discordo/config.toml.$(date +%Y%m%d)

# Backup entire config directory (recommended)
cp -r ~/.config/discordo ~/.config/discordo.backup.$(date +%Y%m%d)
```

#### After Updating Issues

```bash
# Restore configuration if needed
cp ~/.config/discordo/config.toml.backup.YYYYMMDD ~/.config/discordo/config.toml

# Or restore entire config directory
rm -rf ~/.config/discordo
mv ~/.config/discordo.backup.YYYYMMDD ~/.config/discordo
```

### Checking Your Version

```bash
# Check git commit for latest changes
cd discordo
git log --oneline -5

# Check if you're on the correct branch
git branch
git status
```

### Breaking Changes

Discordo is in active development, so breaking changes may occur:

- **Configuration Format**: Major changes will be documented in CHANGELOG.md
- **Keybindings**: Changes are announced in release notes
- **Dependencies**: Go version requirements may change

**Mitigation Strategy**:
1. Always backup configuration before updating
2. Read CHANGELOG.md for breaking changes
3. Test updates in a safe environment first
4. Join the Discord server for community support

### Update Frequency

- **Active Development**: Expect updates multiple times per week
- **Stable Releases**: Less frequent, more thoroughly tested
- **Critical Fixes**: Released as needed

**Recommendation**: Update weekly or when you encounter issues that may be fixed in newer versions.

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

### Tip.cc Autoclaim Not Working

1. **Status Indicator**: Check the Tip.cc status in the top bar - it should show "Auto-claim ON"
2. **Channel Focus**: Ensure you have the channel with the drop focused - autoclaim only works in the currently active channel
3. **Toggle Status**: Press `Ctrl+A` to toggle autoclaim on/off if needed
4. **Configuration**: Verify `auto_claim = true` is set in the `[tipcc]` section
5. **Delay Settings**: If drops are being missed, try reducing `delay_ms` for faster response
6. **Debug Mode**: Enable `debug = true` to see detailed logging of tip.cc detection attempts
7. **Bot Messages**: Autoclaim only works for messages from the official Tip.cc bot (ID: 617037497574359050)

For more help, see the [Contributing Guide](./CONTRIBUTING.md) or join our [Discord server](https://discord.com/invite/VzF9UFn2aB).