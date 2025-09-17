#!/bin/bash

set -e

echo "🧪 Running CoffeDB tests..."

# Run unit tests
echo "📋 Running unit tests..."
go test -v ./internal/...

# Run race detection tests
echo "🏁 Running race detection tests..."
go test -race ./internal/...

# Run benchmarks
echo "📊 Running benchmarks..."
go test -bench=. ./internal/... || true

echo "✅ All tests completed!"
