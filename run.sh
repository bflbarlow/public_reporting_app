#!/bin/bash

# Reporting App - Development Script
set -e

echo "🚀 Starting Reporting App"

# Check for .env file
if [ ! -f .env ]; then
    echo "⚠️  No .env file found. Copying .env.example..."
    cp .env.example .env
    echo "⚠️  Please edit .env file with your settings"
    exit 1
fi

# Load environment variables
export $(grep -v '^#' .env | xargs)

# Check for HMAC_SECRET (unless public paths enabled)
if [ "$ENABLE_PUBLIC_PATHS" != "true" ]; then
    if [ -z "$HMAC_SECRET" ] || [ "$HMAC_SECRET" = "your-secret-key-here-change-me" ]; then
        echo "❌ HMAC_SECRET must be set in .env (not the default value) when ENABLE_PUBLIC_PATHS is not true"
        exit 1
    fi
fi

echo "📊 Environment loaded"
echo "📁 Reports directory: $REPORTS_DIR"
if [ "$ENABLE_PUBLIC_PATHS" = "true" ]; then
    echo "🔓 PUBLIC PATHS ENABLED - security bypassed"
else
    echo "🔒 HMAC secret: [set]"
fi

# Parse command line arguments
if [ "$1" = "genurl" ]; then
    shift
    echo "🔗 Generating signed URL..."
    go run main.go -genurl "$@"
elif [ "$1" = "build" ]; then
    echo "🔨 Building binary..."
    go build -o reporting_app main.go
    echo "✅ Binary built: ./reporting_app"
elif [ "$1" = "test" ]; then
    echo "🧪 Running tests..."
    go test ./...
else
    echo "🌐 Starting server on port $PORT..."
    echo "📝 Available commands:"
    echo "   ./run.sh            - Start server"
    echo "   ./run.sh genurl     - Generate signed URL"
    echo "   ./run.sh build      - Build binary"
    echo "   ./run.sh test       - Run tests"
    echo ""
    echo "🔗 Example URL generation:"
    echo "   ./run.sh genurl -report example_dashboard -params \"organization_id=1,start_date=2024-01-01\""
    echo ""
    go run main.go
fi