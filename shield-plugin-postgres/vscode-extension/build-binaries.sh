#!/bin/bash

# Create bin directory
mkdir -p bin

# Build for macOS (amd64)
echo "Building for macOS (amd64)..."
cd .. && GOOS=darwin GOARCH=amd64 go build -o vscode-extension/bin/shield-plugin-postgres . && cd vscode-extension

# Build for Windows (amd64)
echo "Building for Windows (amd64)..."
cd .. && GOOS=windows GOARCH=amd64 go build -o vscode-extension/bin/shield-plugin-postgres.exe . && cd vscode-extension

# Make files executable
chmod +x bin/*

echo "Build completed. Binaries are in the bin directory."
