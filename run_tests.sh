#!/bin/bash

# Test Runner Script for Distributed Inventory Management
# This script ensures a clean test environment by killing any processes on ports 8080/8081

set -e

echo "=== Distributed Inventory Management Test Runner ==="
echo

# Kill any processes using test ports
echo "Cleaning up test ports (8080, 8081)..."
lsof -ti:8080,8081 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 2

echo "✓ Ports cleaned"
echo

# Run all tests
echo "Running test suite..."
echo
go test ./... -v "$@"

TEST_EXIT_CODE=$?

echo
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo "✅ All tests passed!"
else
    echo "❌ Some tests failed (exit code: $TEST_EXIT_CODE)"
fi

exit $TEST_EXIT_CODE
