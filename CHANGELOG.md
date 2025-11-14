# Changelog

All notable changes to Discordo will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Arrow key navigation support for guilds tree and messages list
- Persistent Tip.cc autoclaim status indicator in top bar
- Key combination to toggle Tip.cc autoclaim (Ctrl+A)
- Visual feedback for Tip.cc autoclaim status (green/red indicators)
- Automatic confirmation dialog handling (auto-clicks Confirm buttons, always enabled)
- Smart confirmation dialog detection by content and button labels
- Safety features to avoid auto-clicking Cancel/Decline buttons
- Separate control: Confirmation dialogs work independently of auto_claim setting
- Comprehensive documentation system
- CONTRIBUTING.md with development guidelines
- CONFIGURATION.md with detailed configuration options
- Enhanced command-line option documentation
- Cross-references between documentation files

### Changed
- Default navigation keybindings changed from vim-style (j/k/g/G) to arrow keys (↑↓/Home/End)
- Updated documentation to reflect new default navigation controls
- Clarified Tip.cc autoclaim scope (focused channel only) in documentation

### Changed
- Improved documentation structure and organization
- Updated README with better installation instructions

### Deprecated

### Removed

### Fixed

### Security

## [0.1.0] - 2024-11-14

### Added
- Initial release of Discordo terminal client
- Basic Discord functionality (guilds, channels, messages)
- Authentication via email/password and QR codes
- Token-based authentication with keyring storage
- Message composition and sending
- Markdown rendering support
- Attachment support
- Desktop notifications
- Mouse and clipboard support
- Configurable keybindings
- Theme customization
- Tip.cc integration with automated airdrop detection
- 2-Factor authentication support
- Cross-platform support (Windows, macOS, Linux)
- Wayland clipboard support

### Features
- **Lightweight**: Minimal resource usage
- **Secure**: Token storage in OS keyring
- **Configurable**: Extensive customization options
- **Rich interactions**: Full Discord message support
- **Terminal-native**: Optimized for terminal usage

## [Upcoming Features]

### Planned
- [ ] Voice chat support
- [ ] Screen sharing
- [ ] Plugin system
- [ ] Custom themes marketplace
- [ ] Advanced message filtering
- [ ] Search functionality
- [ ] Message history export
- [ ] Multi-account support
- [ ] Custom commands and macros
- [ ] Integration with external services

### Under Development
- [ ] Performance optimizations
- [ ] Enhanced notification system
- [ ] Improved attachment handling
- [ ] Better error handling and recovery
- [ ] Comprehensive test suite

## Version History

### Development Phase
Discordo is currently in heavy development with frequent breaking changes. Users should expect updates that may require configuration changes or introduce new features.

### Stability
- **Current Status**: Work in progress
- **API Stability**: Not guaranteed
- **Configuration Stability**: Subject to change
- **Feature Completeness**: Partial

## Migration Guide

### From 0.0.x to 0.1.0
No migration required - initial release.

### Future Migrations
When breaking changes are introduced, migration instructions will be provided in this section.

## Support

For questions about changes or upgrade issues:
- Check the [Configuration Guide](./CONFIGURATION.md)
- Review [Contributing Guidelines](./CONTRIBUTING.md)
- Join our [Discord server](https://discord.com/invite/VzF9UFn2aB)
- Open an issue on [GitHub](https://github.com/ayn2op/discordo/issues)

---

**Note**: This project is under active development. Release dates are estimates and subject to change based on development progress and community feedback.