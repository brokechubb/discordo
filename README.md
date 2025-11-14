# Discordo

### Discordo is a lightweight, secure, and feature-rich Discord terminal client with **TIP.CC** autoclaim support.

![Preview](.github/preview.png)

## Features

- Lightweight
- Configurable
- Mouse & clipboard support
- Attachments
- Notifications
- 2-Factor & QR code authentication
- Discord-flavored markdown
- **Tip.cc integration with automated airdrop detection**

## Installation

### Building from source

```bash
git clone https://github.com/ayn2op/discordo
cd discordo
go build .
```

### Wayland clipboard support

`x11-dev` is required for X11 clipboard compatibility:

- Ubuntu: `apt install xwayland`
- Arch Linux: `pacman -S xorg-xwayland`

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

- **Ctrl+G**: Focus guilds tree
- **Ctrl+T**: Focus messages list
- **Ctrl+Space**: Focus message input
- **Ctrl+C**: Quit application
- **Ctrl+D**: Logout and remove token
- **Enter**: Select channel/send message
- **Esc**: Cancel current action

For complete keybindings and configuration options, see the [Configuration Guide](./CONFIGURATION.md).

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

### Contributing

We welcome all types of contributions:

- Bug reports and feature requests
- Code contributions and improvements
- Documentation enhancements
- Testing and feedback

See the [Contributing Guide](./CONTRIBUTING.md) for detailed development setup and guidelines.

### Building from Source

```bash
git clone https://github.com/ayn2op/discordo
cd discordo
go build .
```

For development commands and coding standards, see [AGENTS.md](./AGENTS.md).

## Changelog

For version history and upcoming features, see the [CHANGELOG.md](./CHANGELOG.md).

## Support

- 📖 [Documentation Index](./DOCS.md) - Complete documentation overview
- ⚙️ [Configuration Guide](./CONFIGURATION.md) - Detailed configuration options
- 🛠️ [Contributing Guide](./CONTRIBUTING.md) - Development and contribution guidelines
- 💬 [Discord Server](https://discord.com/invite/VzF9UFn2aB) - Community support
- 🐛 [Issue Tracker](https://github.com/ayn2op/discordo/issues) - Bug reports and feature requests

> [!IMPORTANT]
> Automated user accounts or "self-bots" are against Discord's Terms of Service. I am not responsible for any loss caused by using "self-bots" or Discordo.
