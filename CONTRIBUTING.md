# Contributing to Discordo

Thank you for your interest in contributing to Discordo! This document provides guidelines and information for contributors.

## Development Setup

### Prerequisites

- Go 1.25.3 or later
- Git

### Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/your-username/discordo.git
   cd discordo
   ```
3. Add the upstream repository:
   ```bash
   git remote add upstream https://github.com/ayn2op/discordo.git
   ```
4. Create a new branch for your feature:
   ```bash
   git checkout -b feature/your-feature-name
   ```

### Building and Running

```bash
# Build the project
go build .

# Run the project
go run .

# Format code
go fmt ./...

# Run static analysis
go vet ./...

# Run tests (when available)
go test ./...
```

## Code Style Guidelines

See [AGENTS.md](./AGENTS.md) for detailed coding standards and patterns used in this project.

### Key Points

- Follow Go conventions and `gofmt` formatting
- Use structured logging with `log/slog`
- Implement proper error handling with wrapped errors
- Separate concerns into appropriate packages
- Use tview for UI components following existing patterns

## Submitting Changes

### Commit Messages

Use clear, descriptive commit messages:
- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests liberally after the first line

### Pull Request Process

1. Ensure your code follows the project's style guidelines
2. Update documentation as needed
3. Test your changes thoroughly
4. Ensure all tests pass
5. Update the CHANGELOG.md if applicable
6. Submit a pull request with a clear description of changes

### Types of Contributions

- **Bug fixes**: Please include steps to reproduce the bug
- **New features**: Describe the use case and implementation approach
- **Documentation improvements**: Always welcome
- **Performance optimizations**: Include benchmarks if possible

## Testing

Currently, Discordo does not have automated tests. When adding tests:

- Create `*_test.go` files in the appropriate packages
- Use table-driven tests for multiple test cases
- Test both success and error paths
- Follow Go testing conventions

## Reporting Issues

When reporting bugs, please include:

- Operating system and version
- Go version
- Discordo version (or commit hash)
- Steps to reproduce the issue
- Expected behavior vs. actual behavior
- Any relevant logs or error messages

## Feature Requests

Feature requests are welcome! Please:

- Check if the feature already exists or is planned
- Describe the use case clearly
- Consider if the feature aligns with the project's goals
- Be open to discussion and refinement

## Development Resources

- [Go Documentation](https://golang.org/doc/)
- [tview Documentation](https://github.com/rivo/tview)
- [Arikawa Discord Library](https://github.com/diamondburned/arikawa)
- [Project Configuration](./CONFIGURATION.md)

## Community

- Join our [Discord server](https://discord.com/invite/VzF9UFn2aB) for discussions
- Check [GitHub Issues](https://github.com/ayn2op/discordo/issues) for known problems
- Review [Pull Requests](https://github.com/ayn2op/discordo/pulls) to see what's being worked on

## License

By contributing to Discordo, you agree that your contributions will be licensed under the same license as the project.