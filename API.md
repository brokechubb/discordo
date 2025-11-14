# API Documentation

This document provides an overview of Discordo's internal architecture and package structure for developers working on the codebase.

## Package Structure

```
discordo/
├── cmd/                    # Command-line interface and application logic
├── internal/               # Internal packages (not exported)
│   ├── cache/             # Caching layer
│   ├── config/            # Configuration management
│   ├── consts/            # Constants
│   ├── http/              # HTTP client and transport
│   ├── keyring/           # Keyring operations
│   ├── logger/            # Logging utilities
│   ├── login/             # Authentication flows
│   ├── markdown/          # Markdown rendering
│   ├── notifications/     # Desktop notifications
│   └── ui/                # UI utilities
├── main.go                # Application entry point
└── AGENTS.md              # Development guidelines
```

## Core Packages

### cmd/

The main application logic and UI components.

#### Key Types

- `application`: Main application struct managing the UI and state
- `guildsTree`: Guild and channel navigation component
- `messagesList`: Message display and interaction component
- `messageInput`: Message composition component
- `interactionHandler`: Discord interaction handling

#### Main Functions

```go
func Run() error                    // Application entry point
func (a *application) Run() error   // Main application loop
```

### internal/config/

Configuration management using TOML files.

#### Key Types

```go
type Config struct {
    Mouse             bool
    Editor            string
    Status            string
    Markdown          bool
    // ... other configuration fields
}

type Keys struct {
    FocusGuildsTree   string
    FocusMessagesList string
    // ... other keybindings
}
```

#### Main Functions

```go
func Load(path string) (*Config, error)  // Load configuration from file
func DefaultPath() string                // Get default config path
```

### internal/keyring/

Secure token storage using OS keyring.

#### Main Functions

```go
func GetToken() (string, error)         // Retrieve stored token
func SetToken(token string) error        // Store token securely
func DeleteToken() error                 // Remove stored token
```

### internal/notifications/

Desktop notification system.

#### Key Types

```go
type Notifier interface {
    Show(title, message string) error
}
```

#### Main Functions

```go
func New() Notifier                      // Create platform-specific notifier
```

### internal/login/

Authentication flows including QR code and form-based login.

#### Key Types

```go
type LoginForm struct {
    // Login form implementation
}

type QRForm struct {
    // QR code login implementation
}
```

## External Dependencies

### Discord Libraries

- `github.com/diamondburned/arikawa/v3`: Discord API client
- `github.com/diamondburned/ningen/v3`: Discord gateway state management

### UI Framework

- `github.com/ayn2op/tview`: Terminal UI framework (forked from rivo/tview)
- `github.com/gdamore/tcell/v2`: Low-level terminal interface

### Other Key Dependencies

- `github.com/BurntSushi/toml`: TOML configuration parsing
- `github.com/yuin/goldmark`: Markdown rendering
- `github.com/zalando/go-keyring`: Cross-platform keyring access
- `golang.design/x/clipboard`: Clipboard support

## Application Lifecycle

1. **Initialization** (`cmd/root.go`)
   - Parse command-line flags
   - Initialize logger
   - Load configuration
   - Set up Discord state

2. **Authentication** (`internal/login/`)
   - Token retrieval from keyring or command-line
   - Login form or QR code authentication
   - Token storage in keyring

3. **UI Setup** (`cmd/application.go`)
   - Create tview application
   - Initialize UI components
   - Set up keybindings
   - Start Discord gateway connection

4. **Main Loop** (`cmd/application.go`)
   - Handle user input
   - Process Discord events
   - Update UI components
   - Manage application state

## Event Handling

### Discord Events

Discord events are handled through the ningen library's event system:

```go
discordState.AddHandler(func(m *gateway.MessageCreateEvent) {
    // Handle new message
})
```

### UI Events

UI events are handled through tview's callback system:

```go
input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    // Handle key press
})
```

## Configuration System

The configuration system uses a hierarchical approach:

1. **Default values** embedded in code
2. **Configuration file** (TOML format)
3. **Environment variables** (for token)
4. **Command-line flags** (override all others)

## Error Handling

Discordo uses Go's standard error handling with structured logging:

```go
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

slog.Error("operation failed", "error", err, "context", "additional info")
```

## Logging

Structured logging using `log/slog`:

```go
slog.Info("user action", "action", "login", "user", userID)
slog.Debug("gateway event", "type", eventType, "data", eventData)
slog.Error("api error", "error", err, "endpoint", endpoint)
```

## Testing Strategy

While Discordo currently has no automated tests, the intended testing approach:

1. **Unit tests** for individual packages
2. **Integration tests** for Discord API interactions
3. **UI tests** for terminal interface
4. **End-to-end tests** for complete workflows

## Development Guidelines

See [AGENTS.md](./AGENTS.md) for comprehensive development guidelines including:

- Code style and formatting
- Import organization
- Naming conventions
- Error handling patterns
- Testing approaches

## Contributing to the API

When modifying the internal API:

1. Maintain backward compatibility where possible
2. Update relevant documentation
3. Add examples for new functionality
4. Consider impact on other components
5. Follow established patterns and conventions

## Performance Considerations

- **Gateway events**: Handle efficiently to avoid blocking
- **UI updates**: Batch updates where possible
- **Memory usage**: Monitor for leaks in long-running sessions
- **Network requests**: Implement proper rate limiting
- **Caching**: Use appropriate caching strategies

For more detailed development information, see the [Contributing Guide](./CONTRIBUTING.md).