#!/bin/bash
# Run all tool call tests

echo "=========================================="
echo "Tool Call Test Suite"
echo "=========================================="
echo ""

# Make all test scripts executable
chmod +x tests/test_tool_call_*.sh

# Track results
passed=0
failed=0

# Run each test
for test in tests/test_tool_call_*.sh; do
  echo "Running: $(basename $test)"
  echo "----------------------------------------"
  if bash "$test"; then
    ((passed++))
  else
    ((failed++))
  fi
  echo ""
  sleep 2  # Give gateway time between tests
done

# Summary
echo "=========================================="
echo "Test Results"
echo "=========================================="
echo "Passed: $passed"
echo "Failed: $failed"
echo ""

if [ $failed -eq 0 ]; then
  echo "✓ All tests passed"
  exit 0
else
  echo "✗ Some tests failed"
  exit 1
fi
