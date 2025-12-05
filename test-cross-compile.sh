#!/bin/bash

# Test script to validate cross-compilation builds
# This simulates the GitHub Actions build process

set -e

echo "=== Testing Cross-Compilation Builds ==="

# Test Linux AMD64
echo "Testing Linux AMD64..."
go build -tags noaudio -ldflags "-s -w" -o discordo-linux-amd64 .
ls -la discordo-linux-amd64

# Test Linux ARM64 (cross-compilation)
echo "Testing Linux ARM64 cross-compilation..."
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags noaudio -ldflags "-s -w" -o discordo-linux-arm64 .
ls -la discordo-linux-arm64

# Test Windows AMD64 (cross-compilation)
echo "Testing Windows AMD64 cross-compilation..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags noaudio -ldflags "-s -w" -o discordo-windows-amd64.exe .
ls -la discordo-windows-amd64.exe

# Test macOS AMD64 (cross-compilation)
echo "Testing macOS AMD64 cross-compilation..."
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -tags noaudio -ldflags "-s -w" -o discordo-macos-amd64 .
ls -la discordo-macos-amd64

# Test macOS ARM64 (cross-compilation)
echo "Testing macOS ARM64 cross-compilation..."
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -tags noaudio -ldflags "-s -w" -o discordo-macos-arm64 .
ls -la discordo-macos-arm64

echo "=== All builds completed successfully ==="

# Cleanup
rm -f discordo-*

echo "=== Cleanup completed ==="