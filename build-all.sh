#!/bin/bash

echo "🚀 Building for Windows..."
GOOS=windows GOARCH=amd64 go build -o build/windows/pulsar-api.exe ./cmd/api

echo "🐧 Building for Linux..."
GOOS=linux GOARCH=amd64 go build -o build/linux/pulsar-api ./cmd/api

echo "🍎 Building for macOS (Intel)..."
GOOS=darwin GOARCH=amd64 go build -o build/mac/pulsar-api ./cmd/api

echo "🍏 Building for macOS (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -o build/mac/pulsar-api-arm64 ./cmd/api

echo "Done!"
