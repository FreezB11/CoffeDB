#!/bin/bash

set -e

echo "🚀 CoffeDB Quick Start"
echo "===================="

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    exit 1
fi

echo "📦 Installing dependencies..."
go mod tidy

echo "🔨 Building CoffeDB..."
go build -o coffedb ./cmd/server

echo "🗂️ Creating data directory..."
mkdir -p data

echo "🚀 Starting CoffeDB server..."
echo "   Server will start on http://localhost:8080"
echo "   Press Ctrl+C to stop"
echo ""

# Run the database
./coffedb &
SERVER_PID=$!

# Wait a moment for server to start
sleep 2

echo "✅ Server started! Testing connection..."

# Test health endpoint
if curl -s http://localhost:8080/api/v1/health > /dev/null; then
    echo "💚 Health check passed!"
else
    echo "❌ Health check failed!"
    kill $SERVER_PID 2>/dev/null || true
    exit 1
fi

echo ""
echo "🎉 CoffeDB is running successfully!"
echo ""
echo "Try these commands:"
echo "   # Create a document"
echo "   curl -X POST http://localhost:8080/api/v1/collections/users/documents \"
echo "     -H 'Content-Type: application/json' \"
echo "     -d '{"name": "John Doe", "email": "john@example.com", "age": 30}'"
echo ""
echo "   # Query documents"  
echo "   curl 'http://localhost:8080/api/v1/collections/users/query?age=30'"
echo ""
echo "   # Check stats"
echo "   curl http://localhost:8080/api/v1/stats"
echo ""
echo "Press Ctrl+C to stop the server..."

# Wait for user interrupt
trap "echo ''; echo 'Stopping server...'; kill $SERVER_PID 2>/dev/null || true; echo 'Server stopped.'; exit 0" INT

# Keep script running
wait $SERVER_PID 2>/dev/null || true
