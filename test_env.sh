#!/bin/bash
# Test the ENABLE_PUBLIC_PATHS feature

echo "Testing ENABLE_PUBLIC_PATHS=false (default)"
export ENABLE_PUBLIC_PATHS=false
export HMAC_SECRET=test-secret
export PORT=8090
export REPORTS_DIR=./reports
export STATIC_DIR=./static
export DATABASES_CONFIG=./databases.yaml

echo "Should require HMAC..."
go run main.go -genurl -report example_dashboard -params "organization_id=1" 2>&1 | grep -v "Loaded report"

echo ""
echo "Testing ENABLE_PUBLIC_PATHS=true"
export ENABLE_PUBLIC_PATHS=true
export HMAC_SECRET=test-secret

echo "Should work even with dummy HMAC_SECRET..."
go run main.go -genurl -report example_dashboard -params "organization_id=1" 2>&1 | grep -v "Loaded report"