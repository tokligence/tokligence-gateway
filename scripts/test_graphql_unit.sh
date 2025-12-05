#!/bin/bash
# GraphQL Unit Test Runner
# Usage: ./scripts/test_graphql_unit.sh [-v]

set -e

# Default PostgreSQL DSN for testing
export TOKLIGENCE_DB_DSN="${TOKLIGENCE_DB_DSN:-postgres://postgres:postgres@localhost:5432/tokligence_test?sslmode=disable}"

VERBOSE=""
if [ "$1" = "-v" ]; then
    VERBOSE="-v"
fi

echo "Running GraphQL unit tests..."
echo "Database: $TOKLIGENCE_DB_DSN"
echo "=============================================="

go test ./internal/graphql/... $VERBOSE -count=1

echo "=============================================="
echo "All GraphQL unit tests passed!"
