#!/bin/bash

set -e

echo "🚀 Building CoffeDB..."

# Clean previous builds
rm -f coffedb

# Build the application
echo "📦 Installing dependencies..."
go mod tidy

echo "🔨 Building binary..."
go build -o coffedb ./cmd/server

echo "✅ Build completed successfully!"
echo "💡 Run with: ./coffedb"
echo "🐳 Or with Docker: docker-compose up -d"
