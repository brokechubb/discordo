# Discordo

### Discordo is a lightweight, secure, and feature-rich Discord terminal client with **TIP.CC** autoclaim support.

## Features

- Lightweight and fast terminal-based Discord client
- Highly configurable with TOML configuration files
- Mouse & clipboard support with image paste capability
- File attachments and media preview
- Desktop notifications with sound support
- 2-Factor & QR code authentication
- Discord-flavored markdown rendering
- **Colored usernames in DM conversations** for better visibility
- **Tip.cc integration with automated airdrop detection (focused channel only)**
- Arrow key navigation (vim-style keys deprecated)
- Persistent status indicators
- Cross-platform support (Windows, macOS, Linux)
- Secure token storage in OS keyring

## Installation

### Option 1: Download Pre-compiled Binary (Recommended)

Download the appropriate binary for your platform from the [latest release](https://github.com/ayn2op/discordo/releases/latest):

| Platform | Binary | Command |
|----------|--------|---------|
| Linux AMD64 | `discordo-linux-amd64` | `wget https://github.com/ayn2op/discordo/releases/latest/download/discordo-linux-amd64 && chmod +x discordo-linux-amd64` |
| Linux ARM64 | `discordo-linux-arm64` | `wget https://github.com/ayn2op/discordo/releases/latest/download/discordo-linux-arm64 && chmod +x discordo-linux-arm64` |
| Windows | `discordo-windows-amd64.exe` | Download from releases page |
| macOS Intel | `discordo-macos-amd64` | `wget https://github.com/ayn2op/discordo/releases/latest/download/discordo-macos-amd64 && chmod +x discordo-macos-amd64` |
| macOS Apple Silicon | `discordo-macos-arm64` | `wget https://github.com/ayn2op/discordo/releases/latest/download/discordo-macos-arm64 && chmod +x discordo-macos-arm64` |

### Option 2: Building from Source

```bash
git clone -b tip-cc-autoclaim https://github.com/ayn2op/discordo
cd discordo
go build .
```

For detailed installation instructions and release information, see the [Release Guide](./RELEASE.md).

### Dependencies

#### Wayland clipboard support

`x11-dev` is required for X11 clipboard compatibility:

- Ubuntu: `apt install xwayland`
- Arch Linux: `pacman -S xorg-xwayland`

#### Go requirements

- Go 1.21 or later
- Git for cloning the repository

#### Optional dependencies

- Notification daemon (for desktop notifications)
- Keyring service (for secure token storage)

## Updating Discordo

Discordo is actively developed with frequent updates. Keep your installation current:

### Quick Update (Recommended)

```bash
cd discordo
git pull origin tip-cc-autoclaim
go build .
```

### Fresh Installation

```bash
# Backup your configuration first
cp ~/.config/discordo/config.toml ~/.config/discordo/config.toml.backup

# Get the latest version
rm -rf discordo
git clone -b tip-cc-autoclaim https://github.com/ayn2op/discordo
cd discordo
go build .

# Restore configuration if needed
cp ~/.config/discordo/config.toml.backup ~/.config/discordo/config.toml
```

### Configuration Updates

New features may add configuration options. Your existing configuration will continue to work with new defaults. See the [Configuration Guide](./CONFIGURATION.md#upgrading-and-updating) for detailed migration instructions.

## Usage

### Basic Usage

1. Run the `discordo` executable with no arguments:

    ```bash
    discordo
    ```

2. Enter your email and password and click on the "Login" button to continue.

### Command-Line Options

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

### Environment Variables

- `DISCORDO_TOKEN`: Your Discord authentication token
- `XDG_CONFIG_HOME`: Override default config directory (Unix)
- `HOME`: User home directory (used for config paths)

### Token Authentication

If you prefer to use an authentication token:

```bash
# Using command-line flag
discordo --token "YOUR_DISCORD_TOKEN"

# Using environment variable
export DISCORDO_TOKEN="YOUR_DISCORD_TOKEN"
discordo
```

The token is stored securely in the default OS-specific keyring for subsequent sessions.

### Key Controls

#### Navigation

- **Arrow Keys** (↑↓): Navigate guilds, channels, and messages
- **Home/End**: Jump to first/last item in lists
- **Ctrl+G**: Focus guilds tree
- **Ctrl+T**: Focus messages list
- **Ctrl+Space**: Focus message input

#### Actions

- **Enter**: Select channel/send message
- **Esc**: Cancel current action
- **Ctrl+A**: Toggle Tip.cc auto-claim on/off
- **Ctrl+B**: Toggle guilds tree visibility
- **Ctrl+C**: Quit application
- **Ctrl+D**: Logout and remove token
- **Ctrl+K**: Clear notifications
- **Ctrl+H**: Focus previous widget
- **Ctrl+L**: Focus next widget

For complete keybindings and configuration options, see the [Configuration Guide](./CONFIGURATION.md).

### Tip.cc Autoclaim

Discordo includes automated Tip.cc airdrop detection and claiming with a persistent status indicator:

- **Auto-Confirmation**: Always automatically clicks "Confirm" buttons in confirmation dialogs
- **Drop Claiming**: Toggle automatic drop claiming with Ctrl+A (focused channel only)
- **Persistent Status**: Real-time autoclaim status displayed in the top bar
- **Focused Channel Only**: Drop claiming only works in the currently focused channel for safety
- **One-Touch Toggle**: Press `Ctrl+A` to instantly toggle drop claiming on/off
- **Visual Feedback**: Green indicator when enabled, red when disabled
- **Configurable Delay**: Adjustable delay to prevent race conditions
- **Debug Mode**: Detailed logging for troubleshooting
- **Manual Override**: Always available to manually claim drops
- **Smart Detection**: Identifies confirmation dialogs by content and button labels

```toml
[tipcc]
auto_claim = true    # Enable autoclaim
debug = true        # Show detection logs
delay_ms = 100      # Delay before claiming
```

**Status Indicator**:

- 🟢 **Auto-claim ON** (focused channel) - Ready to claim drops and auto-confirmations
- 🔴 **Auto-claim OFF** - Manual drop claiming + auto-confirmations

## Configuration

Discordo is highly configurable through a TOML configuration file. You can customize behavior, keybindings, themes, and more.

### Configuration Location

- Unix: `$XDG_CONFIG_HOME/discordo/config.toml` or `$HOME/.config/discordo/config.toml`
- macOS: `$HOME/Library/Application Support/discordo/config.toml`
- Windows: `%AppData%/discordo/config.toml`

### Getting Started

Discordo uses built-in defaults if no configuration file exists. For comprehensive documentation of all options including:

- Keybinding customization
- Theme configuration
- Notification settings
- Tip.cc integration
- Advanced options

See the complete [Configuration Guide](./CONFIGURATION.md).

### Quick Example

```toml
# Basic customization
mouse = true
markdown = true
status = "online"

# Custom keybindings
[keys]
focus_guilds_tree = "Alt+1"
focus_messages_list = "Alt+2"

# Theme tweaks
[theme.messages_list]
mention_style = { foreground = "magenta" }
dm_user_color = "green"  # Color for DM usernames

# Tip.cc autoclaim (focused channel only)
[tipcc]
auto_claim = true
debug = true
delay_ms = 100
```

## FAQ

### Manually adding token to keyring

Do this if you get the error:

> failed to get token from keyring: secret not found in keyring

#### Windows

Run the following command in a terminal window. Replace `YOUR_DISCORD_TOKEN` with your authentication token.

```sh
cmdkey /add:discordo /user:token /pass:YOUR_DISCORD_TOKEN
```

#### MacOS

Run the following command in a terminal window. Replace `YOUR_DISCORD_TOKEN` with your authentication token.

```sh
security add-generic-password -s discordo -a token -w "YOUR_DISCORD_TOKEN"
```

#### Linux

1. Start the keyring daemon.

```sh
eval $(gnome-keyring-daemon --start)
export $(gnome-keyring-daemon --start)
```

2. Create the `login` keyring if it does not exist already. See [GNOME/Keyring](https://wiki.archlinux.org/title/GNOME/Keyring) for more information.

3. Run the following command to create the `token` entry.

```sh
secret-tool store --label="Discord Token" service discordo username token
```

4. When it prompts for the password, paste your token, and hit enter to confirm.

## Development

Discordo is open source and welcomes contributions! The project is written in Go and uses the tview library for the terminal UI.

### Current Branch: `tip-cc-autoclaim`

This branch includes the latest features:

- Tip.cc autoclaim functionality
- Arrow key navigation (replacing vim-style keys)
- Enhanced notification system
- Improved configuration options
- Bug fixes and performance improvements

### Contributing

We welcome all types of contributions:

- Bug reports and feature requests
- Code contributions and improvements
- Documentation enhancements
- Testing and feedback

See the [Contributing Guide](./CONTRIBUTING.md) for detailed development setup and guidelines.

### Development Commands

```bash
# Build the application
go build .

# Run the application
go run .

# Format code
go fmt ./...

# Run static analysis
go vet ./...

# Run tests (when available)
go test ./...
```

### Building from Source

```bash
git clone -b tip-cc-autoclaim https://github.com/ayn2op/discordo
cd discordo
go build .
```

For development commands and coding standards, see [AGENTS.md](./AGENTS.md).

## Changelog

For version history and upcoming features, see the [CHANGELOG.md](./CHANGELOG.md).

## Releases

For pre-compiled binaries and installation instructions, see the [Release Guide](./RELEASE.md).

### Recent Changes (tip-cc-autoclaim branch)

- **Fixed**: DM conversation user coloring - users now display with proper colored usernames
- **Added**: Configurable DM user color theme setting
- **Fixed**: Key binding conflict - changed clear notifications from Ctrl+N to Ctrl+K
- **Added**: Persistent Tip.cc autoclaim status indicator
- **Changed**: Default navigation from vim-style (j/k) to arrow keys
- **Enhanced**: Configuration system with better documentation
- **Improved**: Error handling and stability

## Support

- 📖 [Documentation Index](./DOCS.md) - Complete documentation overview
- ⚙️ [Configuration Guide](./CONFIGURATION.md) - Detailed configuration options
- 🛠️ [Contributing Guide](./CONTRIBUTING.md) - Development and contribution guidelines
- 💬 [Discord Server](https://discord.com/invite/VzF9UFn2aB) - Community support
- 🐛 [Issue Tracker](https://github.com/ayn2op/discordo/issues) - Bug reports and feature requests

> [!IMPORTANT]
> Automated user accounts or "self-bots" are against Discord's Terms of Service. I am not responsible for any loss caused by using "self-bots" or Discordo.
