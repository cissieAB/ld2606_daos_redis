#!/bin/bash
# Setup script for Traffic Backend Server

set -e

echo "=== Traffic Backend Server Setup ==="
echo

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "✗ Error: Go is not installed"
    echo "Please install Go 1.20 or later from https://go.dev/doc/install"
    exit 1
fi

echo "✓ Go version: $(go version)"
echo

# Check Go version (need 1.20+)
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.20"
if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then 
    echo "⚠ Warning: Go version $GO_VERSION detected. Version 1.20+ recommended."
fi

# Download Go modules
echo "Downloading Go modules..."
go mod download
echo "✓ Dependencies downloaded"
echo

# Verify dependencies
echo "Verifying dependencies..."
go mod verify
echo "✓ Dependencies verified"
echo

# Tidy up go.mod and go.sum
echo "Tidying go.mod..."
go mod tidy
echo "✓ go.mod tidied"
echo

# Build the project to verify everything works
echo "Building project..."
if go build -o backend .; then
    echo "✓ Build successful: ./backend"
    rm -f backend  # Clean up test build
else
    echo "✗ Build failed"
    exit 1
fi
echo

echo "=== Setup Complete ==="
echo
echo "Next steps:"
echo "  1. Start Redis (if not running):"
echo "     docker run -d -p 6379:6379 redis/redis-stack-server:latest"
echo
echo "  2. Run the server:"
echo "     go run .                    # Development mode"
echo "     DEBUG=true go run .         # With debug logging"
echo
echo "  3. Build and run:"
echo "     go build -o backend"
echo "     ./backend"
echo
echo "For more information, see README.md"
