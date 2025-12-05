#!/bin/bash

# Script to test the release workflow locally
# This simulates the GitHub Actions release build process

set -e

echo "=== Testing Release Workflow Locally ==="

# Clean any previous builds
rm -f discordo*

echo "Step 1: Verify Go installation"
go version
go env GOOS GOARCH

echo "Step 2: Update Go modules"
echo "Updating go modules..."
go mod tidy || echo "go mod tidy failed, continuing anyway"
go mod download || echo "go mod download failed, continuing anyway"

echo "Step 3: Set build variables"
export ARTIFACT_NAME="discordo"
export ASSET_NAME="discordo-linux-amd64"

echo "Step 4: Build Linux AMD64"
echo "Building Linux AMD64 with noaudio tag"
go build -tags noaudio -ldflags "-s -w" -o $ARTIFACT_NAME .
ls -la $ARTIFACT_NAME
mv $ARTIFACT_NAME $ASSET_NAME

echo "Step 5: Build Linux ARM64 (cross-compilation)"
export ASSET_NAME="discordo-linux-arm64"
echo "Cross-compiling Linux ARM64 with CGO_ENABLED=0"
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags=noaudio -ldflags "-s -w" -o $ARTIFACT_NAME .
ls -la $ARTIFACT_NAME
mv $ARTIFACT_NAME $ASSET_NAME

echo "Step 6: Build Windows AMD64 (cross-compilation)"
export ASSET_NAME="discordo-windows-amd64.exe"
echo "Building Windows AMD64"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags=noaudio -ldflags "-s -w" -o $ARTIFACT_NAME .
ls -la $ARTIFACT_NAME
mv $ARTIFACT_NAME $ASSET_NAME

echo "Step 7: Build macOS AMD64 (cross-compilation)"
export ASSET_NAME="discordo-macos-amd64"
echo "Building macOS AMD64"
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -tags=noaudio -ldflags "-s -w" -o $ARTIFACT_NAME .
ls -la $ARTIFACT_NAME
mv $ARTIFACT_NAME $ASSET_NAME

echo "Step 8: Build macOS ARM64 (cross-compilation)"
export ASSET_NAME="discordo-macos-arm64"
echo "Building macOS ARM64"
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -tags=noaudio -ldflags "-s -w" -o $ARTIFACT_NAME .
ls -la $ARTIFACT_NAME
mv $ARTIFACT_NAME $ASSET_NAME

echo "Step 9: Verify all artifacts"
echo "Release assets created:"
ls -la discordo-*

echo "Step 10: Test binary functionality"
echo "Testing Linux AMD64 binary..."
./discordo-linux-amd64 --help | head -5

echo "=== Release workflow test completed successfully ==="
echo "All binaries built and basic functionality verified"

# Cleanup test artifacts
rm -f discordo-*