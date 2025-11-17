# Discordo - Agent Guidelines

## Build Commands
- **Build**: `go build -tags noaudio .` (Linux default)
- **Run**: `go run -tags noaudio .` (Linux default)
- **Format**: `go fmt ./...`
- **Vet**: `go vet ./...`
- **Test**: `go test ./...` (no tests currently exist)
- **Single test**: `go test -run TestName ./pkg/path`

### Build Tags
- **noaudio**: Default for Linux builds, disables audio dependencies
- **Full audio**: Omit `-tags noaudio` for platforms with audio support

## Code Style Guidelines

### Imports
- Group imports: stdlib, third-party, internal packages
- Use explicit import paths, no aliases unless needed
- Order: `import ("fmt"; "github.com/..."; "github.com/ayn2op/discordo/...")`

### Formatting & Types
- Use `gofmt` standard formatting
- Prefer explicit types over `var` with initialization
- Use named struct fields with TOML tags for config
- Error handling: return errors, use `fmt.Errorf` with `%w` for wrapping

### Naming Conventions
- PascalCase for exported types/functions
- camelCase for unexported
- Use descriptive names, avoid abbreviations
- Constants: `PascalCase` or `UPPER_SNAKE_CASE`

### Patterns
- Use `log/slog` for logging with structured fields
- Configuration via TOML with embedded defaults
- Use tview for UI components following existing patterns
- Separate packages by domain (config, keyring, notifications, etc.)

### Testing
- No tests currently - add `*_test.go` files when implementing
- Use table-driven tests for multiple cases
- Test error paths explicitly