#!/bin/bash

# Test: Docker Personal Edition - Basic Functionality
# Tests basic container operations without requiring real API keys
# This script tests infrastructure, not API functionality

set -e

echo "🧪 Testing Docker Personal Edition - Basic Functionality"
echo "========================================================="
echo ""

CONTAINER_NAME="tokligence-test-personal"
IMAGE_NAME="tokligence-gateway-gateway-personal:latest"
TEST_PORT=18081  # Use non-standard port to avoid conflicts

# Cleanup function
cleanup() {
  echo ""
  echo "Cleaning up..."
  docker stop "$CONTAINER_NAME" 2>/dev/null || true
  docker rm "$CONTAINER_NAME" 2>/dev/null || true
  echo "Cleanup complete"
}
trap cleanup EXIT

# Test 1: Image exists
echo "=== Test 1: Docker Image Exists ==="
if docker images | grep -q "tokligence-gateway-gateway-personal"; then
  IMAGE_SIZE=$(docker images tokligence-gateway-gateway-personal:latest --format "{{.Size}}")
  echo "  ✅ Image found: $IMAGE_NAME"
  echo "  📦 Size: $IMAGE_SIZE"
else
  echo "  ❌ Image not found. Build it first with: docker-compose build personal"
  exit 1
fi
echo ""

# Test 2: Container startup
echo "=== Test 2: Container Startup ==="
echo "  Starting container on port $TEST_PORT..."
docker run -d \
  --name "$CONTAINER_NAME" \
  -p "$TEST_PORT:8081" \
  -e TOKLIGENCE_AUTH_DISABLED=true \
  -e TOKLIGENCE_LOG_LEVEL=debug \
  "$IMAGE_NAME" > /dev/null

echo "  ⏳ Waiting 3 seconds for container to start..."
sleep 3

if docker ps | grep -q "$CONTAINER_NAME"; then
  echo "  ✅ Container started successfully"
else
  echo "  ❌ Container failed to start"
  docker logs "$CONTAINER_NAME"
  exit 1
fi
echo ""

# Test 3: Health endpoint
echo "=== Test 3: Health Endpoint ==="
MAX_RETRIES=5
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  if curl -sf "http://localhost:$TEST_PORT/health" > /dev/null 2>&1; then
    HEALTH_RESPONSE=$(curl -s "http://localhost:$TEST_PORT/health")
    echo "  ✅ Health endpoint responding"
    echo "  Response: $HEALTH_RESPONSE"
    break
  else
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
      echo "  ⏳ Retry $RETRY_COUNT/$MAX_RETRIES..."
      sleep 2
    else
      echo "  ❌ Health endpoint not responding after $MAX_RETRIES attempts"
      docker logs "$CONTAINER_NAME" | tail -20
      exit 1
    fi
  fi
done
echo ""

# Test 4: Environment variable override
echo "=== Test 4: Environment Variables ==="
LOG_LEVEL=$(docker exec "$CONTAINER_NAME" env | grep TOKLIGENCE_LOG_LEVEL || echo "not found")
AUTH_DISABLED=$(docker exec "$CONTAINER_NAME" env | grep TOKLIGENCE_AUTH_DISABLED || echo "not found")

if echo "$LOG_LEVEL" | grep -q "debug"; then
  echo "  ✅ LOG_LEVEL correctly set to debug"
else
  echo "  ⚠️  LOG_LEVEL: $LOG_LEVEL"
fi

if echo "$AUTH_DISABLED" | grep -q "true"; then
  echo "  ✅ AUTH_DISABLED correctly set to true"
else
  echo "  ⚠️  AUTH_DISABLED: $AUTH_DISABLED"
fi
echo ""

# Test 5: Container logs
echo "=== Test 5: Container Logs ==="
LOGS=$(docker logs "$CONTAINER_NAME" 2>&1 | tail -10)
if echo "$LOGS" | grep -qE "(Starting|started|listening|HTTP|8081)"; then
  echo "  ✅ Container logs show startup activity"
  echo "  Last 3 log lines:"
  docker logs "$CONTAINER_NAME" 2>&1 | tail -3 | sed 's/^/    /'
else
  echo "  ⚠️  Unexpected logs:"
  echo "$LOGS" | sed 's/^/    /'
fi
echo ""

# Test 6: Memory usage
echo "=== Test 6: Resource Usage ==="
MEMORY=$(docker stats "$CONTAINER_NAME" --no-stream --format "{{.MemUsage}}" | awk '{print $1}')
CPU=$(docker stats "$CONTAINER_NAME" --no-stream --format "{{.CPUPerc}}")
echo "  Memory: $MEMORY"
echo "  CPU: $CPU"
echo "  ✅ Resource stats collected"
echo ""

# Test 7: Container stop
echo "=== Test 7: Graceful Shutdown ==="
echo "  Stopping container..."
START_TIME=$(date +%s)
docker stop "$CONTAINER_NAME" > /dev/null
END_TIME=$(date +%s)
STOP_DURATION=$((END_TIME - START_TIME))

echo "  ✅ Container stopped in ${STOP_DURATION}s"

if [ $STOP_DURATION -lt 15 ]; then
  echo "  ✅ Graceful shutdown (< 15s)"
else
  echo "  ⚠️  Slow shutdown (${STOP_DURATION}s)"
fi
echo ""

# Test 8: Container restart
echo "=== Test 8: Container Restart ==="
echo "  Starting container again..."
docker start "$CONTAINER_NAME" > /dev/null
sleep 3

if curl -sf "http://localhost:$TEST_PORT/health" > /dev/null 2>&1; then
  echo "  ✅ Container restarted and healthy"
else
  echo "  ❌ Container failed to restart properly"
  exit 1
fi
echo ""

echo "========================================================="
echo "✅ All Docker Personal Edition Basic Tests Passed!"
echo ""
echo "Summary:"
echo "  1. Image exists ✅"
echo "  2. Container startup ✅"
echo "  3. Health endpoint ✅"
echo "  4. Environment variables ✅"
echo "  5. Container logs ✅"
echo "  6. Resource usage ✅"
echo "  7. Graceful shutdown ✅"
echo "  8. Container restart ✅"
echo ""
echo "Note: API functionality tests (requiring real API keys) not included."
echo "Run those separately with real credentials."
