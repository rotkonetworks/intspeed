#!/bin/bash
set -euo pipefail

echo "🚀 Installing International Speedtest CLI v2.0"
echo ""

# Detect package manager preference
if command -v nix &> /dev/null && [ -f flake.nix ]; then
    echo "🎯 Nix detected - using Nix for installation"
    
    echo "🔨 Building with Nix..."
    nix build .#intspeed-cli .#intspeed-server
    
    echo "📦 Installing binaries..."
    sudo cp result/bin/intspeed /usr/local/bin/ 2>/dev/null || {
        echo "Installing to user profile instead..."
        nix profile install .#intspeed-cli .#intspeed-server
    }
    
    echo "✅ Nix installation complete!"
    
elif command -v go &> /dev/null; then
    echo "🐹 Using Go toolchain for installation"
    
    # Check Go version
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    REQUIRED_VERSION="1.21"
    
    if ! printf '%s\n%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V -C; then
        echo "❌ Go version $GO_VERSION is too old. Please upgrade to Go $REQUIRED_VERSION or later."
        exit 1
    fi
    
    echo "✅ Go version $GO_VERSION detected"
    
    # Build
    echo "🔨 Building..."
    make build
    
    # Install
    echo "📦 Installing..."
    sudo cp build/intspeed /usr/local/bin/
    sudo cp build/intspeed-server /usr/local/bin/
    sudo chmod +x /usr/local/bin/intspeed*
    
    echo "✅ Go installation complete!"
    
else
    echo "❌ Neither Nix nor Go found."
    echo ""
    echo "Install options:"
    echo "1. Install Nix: curl -L https://nixos.org/nix/install | sh"
    echo "2. Install Go: https://golang.org/doc/install"
    exit 1
fi

# Setup
mkdir -p results

echo ""
echo "🎉 Installation successful!"
echo ""
echo "📋 Available commands:"
echo "  intspeed test --html     # Run tests with HTML report"
echo "  intspeed locations       # List test locations"
echo "  intspeed-server          # Start interactive web UI"
echo ""

if command -v nix &> /dev/null; then
    echo "🚀 Development with Nix:"
    echo "  nix develop             # Enter development shell"
    echo "  dev-scripts setup       # Initialize development"
    echo "  dev-scripts server      # Start dev server with hot reload"
    echo ""
fi

echo "💻 Open http://localhost:8080 after starting the server"
