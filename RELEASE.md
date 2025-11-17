# Release Guide

This document explains how to create and manage Discordo releases using the automated GitHub Actions workflow.

## Automated Releases

Discordo uses GitHub Actions to automatically create releases with pre-compiled binaries for multiple platforms.

### Triggering a Release

There are two ways to trigger a release:

#### Method 1: Tag-based Release (Recommended)

1. Create and push a version tag:
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

2. The release workflow will automatically:
   - Build binaries for all supported platforms
   - Create a GitHub release with changelog
   - Attach all binaries as release assets

#### Method 2: Manual Release

1. Go to the **Actions** tab in your GitHub repository
2. Select **Release** workflow from the sidebar
3. Click **Run workflow**
4. Fill in the parameters:
   - **Version**: Release version (e.g., `v0.1.0`)
   - **Prerelease**: Check if this is a prerelease

### Supported Platforms

The release workflow builds binaries for the following platforms:

| Platform | Architecture | Binary Name |
|----------|-------------|--------------|
| Linux | AMD64 | `discordo-Linux-amd64` |
| Linux | ARM64 | `discordo-Linux-arm64` |
| Windows | AMD64 | `discordo-windows-amd64.exe` |
| macOS | Intel (AMD64) | `discordo-macOS-amd64` |
| macOS | Apple Silicon (ARM64) | `discordo-macOS-arm64` |

## Release Assets

Each release includes:

### Pre-compiled Binaries
- **Optimized**: Built with `-ldflags "-s -w"` for smaller size
- **Stripped**: Debug symbols removed (Unix platforms)
- **Ready to run**: No compilation required

### Release Notes
- **Automatic changelog**: Generated from `CHANGELOG.md`
- **Installation instructions**: Platform-specific setup steps
- **Quick start guide**: Basic usage instructions

## Installation from Release

### Linux

```bash
# Download the appropriate binary
wget https://github.com/brokechubb/discordo/releases/latest/download/discordo-Linux-amd64

# Make it executable
chmod +x discordo-linux-amd64

# Run it
./discordo-linux-amd64
```

### Windows

```powershell
# Download using PowerShell
Invoke-WebRequest -Uri "https://github.com/brokechubb/discordo/releases/latest/download/discordo-windows-amd64.exe" -OutFile "discordo.exe"

# Run it
.\discordo.exe
```

### macOS

```bash
# Intel Macs
wget https://github.com/brokechubb/discordo/releases/latest/download/discordo-macOS-amd64
chmod +x discordo-macos-amd64
./discordo-macos-amd64

# Apple Silicon Macs
wget https://github.com/brokechubb/discordo/releases/latest/download/discordo-macOS-arm64
chmod +x discordo-macos-arm64
./discordo-macos-arm64
```

## Version Management

### Semantic Versioning

Discordo follows [Semantic Versioning](https://semver.org/):

- **MAJOR.MINOR.PATCH** (e.g., `0.1.0`)
- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### Prereleases

Use prereleases for testing and development:

- `v0.1.0-alpha` - Alpha release
- `v0.1.0-beta` - Beta release  
- `v0.1.0-rc.1` - Release candidate

### Branch Strategy

- **`tip-cc-autoclaim`**: Main development branch
- **Tags**: Created from main branch for releases
- **CI/CD**: Automated testing and building

## Release Process

### Before Release

1. **Update CHANGELOG.md**:
   ```markdown
   ## [v0.1.0] - 2024-11-15
   
   ### Added
   - New feature description
   
   ### Fixed  
   - Bug fix description
   ```

2. **Update version in code** (if applicable):
   - Check for hardcoded version strings
   - Update documentation dates

3. **Test thoroughly**:
    ```bash
    go test ./...
    go build -tags noaudio .
    ./discordo --help
    ```

4. **Commit changes**:
   ```bash
   git add .
   git commit -m "Release v0.1.0"
   git push origin main
   ```

### Creating Release

1. **Tag the release**:
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

2. **Monitor the workflow**:
   - Go to Actions tab in GitHub
   - Watch the Release workflow progress
   - Check for any build failures

3. **Verify the release**:
   - Download and test binaries
   - Check release notes accuracy
   - Verify all assets are present

### After Release

1. **Update documentation**:
   - Update README.md with new features
   - Update any version-specific instructions

2. **Communicate**:
   - Announce in Discord server
   - Create GitHub discussion
   - Update any external references

## Troubleshooting

### Build Failures

**Common issues**:
- **Go version incompatibility**: Workflow uses stable Go version
- **Missing dependencies**: Linux builds require `libx11-dev`
- **Cross-compilation errors**: Check GOOS/GOARCH settings

**Solutions**:
1. Check workflow logs for specific errors
2. Ensure all dependencies are declared in go.mod
3. Test build locally: `GOOS=linux GOARCH=arm64 go build -tags noaudio .`

### Release Issues

**Asset not uploaded**:
- Check build job completed successfully
- Verify artifact names match expected pattern
- Check file permissions

**Changelog missing**:
- Ensure CHANGELOG.md exists and has proper format
- Check for markdown syntax errors
- Verify Unreleased section exists

### Manual Recovery

If automated release fails, you can create a manual release:

1. **Build locally**:
   ```bash
   # Build for current platform
go build -tags noaudio -ldflags "-s -w" -o discordo .
   
# Cross-compile examples
    GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o discordo.exe .
    GOOS=linux GOARCH=amd64 go build -tags noaudio -ldflags "-s -w" -o discordo-linux .
   ```

2. **Create manual release**:
   - Go to Releases page in GitHub
   - Click "Create a new release"
   - Upload binaries manually
   - Copy changelog from CHANGELOG.md

## Release Workflow Details

### Build Matrix

The workflow uses a build matrix to create binaries for:

```yaml
strategy:
  matrix:
    include:
      - os: ubuntu-latest
        arch: amd64
        artifact_name: discordo
        asset_name: discordo-linux-amd64
      # ... other platforms
```

### Build Flags

Binaries are built with optimization flags:

```bash
go build -tags noaudio -ldflags "-s -w" -o discordo .
```

- `-tags noaudio`: Disable audio dependencies (Linux default)
- `-s`: Omit symbol table
- `-w`: Omit DWARF symbol table
- Result: Smaller binary sizes with better compatibility

### Artifact Management

- **Upload**: Each platform uploads its binary as an artifact
- **Download**: Release job downloads all artifacts
- **Package**: Binaries are renamed and organized
- **Release**: All binaries attached to GitHub release

## Security Considerations

### Binary Verification

Users can verify binary authenticity:

1. **Check GitHub release**: Only download from official releases
2. **Verify checksums**: Future releases may include SHA256 checksums
3. **Code review**: All code is open source and reviewable

### Minimal Dependencies

Release binaries include only necessary dependencies:
- Go runtime statically linked
- No external runtime requirements
- Minimal system library dependencies

## Contributing to Releases

### Testing Pre-releases

Help test pre-releases:

1. Download prerelease binaries
2. Test on your platform
3. Report issues in GitHub
4. Provide feedback in Discord

### Suggesting Improvements

Ideas for improving releases:

- Additional platform support
- Better installation methods
- Package manager support (brew, apt, etc.)
- Automatic update mechanisms

For release-related issues, open a GitHub issue with the "release" label.