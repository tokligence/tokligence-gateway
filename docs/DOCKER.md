# Docker Deployment Guide

Tokligence Gateway provides two Docker editions optimized for different use cases.

## Quick Start

### Personal Edition (No Authentication)

Perfect for individual developers:

```bash
# Using docker-compose
docker-compose --profile personal up -d

# Or using Docker directly
docker build -f Dockerfile.personal -t tokligence/gateway:personal .
docker run -d \
  -p 8081:8081 \
  -e TOKLIGENCE_ANTHROPIC_API_KEY=your_key_here \
  -e TOKLIGENCE_ROUTES='claude*=>anthropic' \
  tokligence/gateway:personal
```

### Team Edition (Authentication Enabled)

For teams with user management:

```bash
# Using docker-compose
docker-compose --profile team up -d

# Or using Docker directly
docker build -f Dockerfile.team -t tokligence/gateway:team .
docker run -d \
  -p 8081:8081 \
  -e TOKLIGENCE_ANTHROPIC_API_KEY=your_key_here \
  -e TOKLIGENCE_ROUTES='claude*=>anthropic' \
  -e DEFAULT_ADMIN_EMAIL=admin@example.com \
  -e DEFAULT_ADMIN_PASSWORD=secure_password \
  tokligence/gateway:team
```

## Differences Between Editions

| Feature | Personal Edition | Team Edition |
|---------|-----------------|--------------|
| Authentication | ❌ Disabled | ✅ Enabled |
| User Management | ❌ No | ✅ Yes |
| API Keys | ❌ Not required | ✅ Required |
| Default Admin | N/A | ✅ Auto-created |
| Use Case | Individual developers | Teams and organizations |
| CLI Tools | gatewayd only | gateway + gatewayd |

## Environment Variables

### Common Variables (Both Editions)

```bash
# Provider API Keys
TOKLIGENCE_ANTHROPIC_API_KEY=sk-ant-api03-...
TOKLIGENCE_OPENAI_API_KEY=sk-...

# Routing Configuration
TOKLIGENCE_ROUTES=claude*=>anthropic,gpt*=>openai

# Logging
TOKLIGENCE_LOG_LEVEL=info  # debug, info, warn, error
TOKLIGENCE_LOG_FILE_DAEMON=/app/logs/gatewayd.log

# Database
TOKLIGENCE_DB_PATH=/app/data/tokligence.db
```

### Team Edition Only

```bash
# Default Admin User
DEFAULT_ADMIN_EMAIL=admin@example.com
DEFAULT_ADMIN_PASSWORD=changeme
```

## Data Persistence

### Using Docker Volumes

```bash
# Personal Edition
docker run -d \
  -v gateway-data:/app/data \
  -v gateway-logs:/app/logs \
  tokligence/gateway:personal

# Team Edition
docker run -d \
  -v gateway-data:/app/data \
  -v gateway-logs:/app/logs \
  tokligence/gateway:team
```

### Using Bind Mounts

```bash
# Create local directories
mkdir -p ./data ./logs

# Run with bind mounts
docker run -d \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/logs:/app/logs \
  tokligence/gateway:personal
```

## Configuration

### Custom Configuration File

You can mount a custom configuration file:

```bash
docker run -d \
  -v $(pwd)/config/my-gateway.ini:/app/config/gateway.ini \
  tokligence/gateway:personal
```

### Environment Variables Override

Environment variables take precedence over config files:

```bash
docker run -d \
  -e TOKLIGENCE_HTTP_ADDRESS=:9000 \
  -e TOKLIGENCE_ANTHROPIC_MAX_TOKENS=4096 \
  tokligence/gateway:personal
```

## Team Edition - User Management

### View Default Credentials

On first startup, the team edition displays default admin credentials:

```bash
docker logs tokligence-gateway-team
```

### Create API Key for Default Admin

```bash
# Create an API key
docker exec tokligence-gateway-team \
  /app/gateway user create-key admin@example.com

# Output: sk-tok-...
```

### Create Additional Users

```bash
# Add a new user
docker exec tokligence-gateway-team \
  /app/gateway user add developer@example.com

# Create API key for the new user
docker exec tokligence-gateway-team \
  /app/gateway user create-key developer@example.com
```

## Health Checks

Both editions include health checks:

```bash
# Check container health
docker ps --filter name=gateway

# Manual health check
curl http://localhost:8081/health
```

## Codex CLI Integration

### Personal Edition

```bash
# Set environment variables
export OPENAI_BASE_URL=http://localhost:8081/v1
export OPENAI_API_KEY=dummy  # Auth disabled

# Run Codex
codex --full-auto --config 'model="claude-3-5-sonnet-20241022"'
```

### Team Edition

```bash
# Get API key from container
API_KEY=$(docker exec tokligence-gateway-team \
  /app/gateway user create-key admin@example.com)

# Set environment variables
export OPENAI_BASE_URL=http://localhost:8081/v1
export OPENAI_API_KEY=$API_KEY

# Run Codex
codex --full-auto --config 'model="claude-3-5-sonnet-20241022"'
```

## Building Multi-Architecture Images

```bash
# Setup buildx
docker buildx create --use

# Build for multiple architectures
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f Dockerfile.personal \
  -t tokligence/gateway:0.3.0-personal \
  --push .

docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f Dockerfile.team \
  -t tokligence/gateway:0.3.0-team \
  --push .
```

## Docker Compose Profiles

The `docker-compose.yml` uses profiles to manage editions:

```bash
# Run personal edition
docker-compose --profile personal up -d

# Run team edition
docker-compose --profile team up -d

# Stop and remove
docker-compose --profile personal down
docker-compose --profile team down
```

## Logs

### View Live Logs

```bash
# Docker logs
docker logs -f tokligence-gateway-personal

# Or from mounted volume
tail -f ./logs/gatewayd.log
```

### Log Rotation

Logs are automatically rotated daily with the format:
- `gatewayd.log` - Current log
- `gatewayd-YYYY-MM-DD.log` - Archived logs

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker logs tokligence-gateway-personal

# Check if port is in use
lsof -i :8081  # or ss -ltnp | grep 8081
```

### Permission Issues

```bash
# Ensure volumes are writable
docker run --rm -v gateway-data:/data alpine chmod 777 /data
```

### API Key Not Working (Team Edition)

```bash
# Verify user exists
docker exec tokligence-gateway-team \
  /app/gateway user list

# Recreate API key
docker exec tokligence-gateway-team \
  /app/gateway user create-key admin@example.com
```

### Reset Database

```bash
# Stop container
docker-compose --profile personal down

# Remove volume
docker volume rm tokligence-gateway_gateway-personal-data

# Restart
docker-compose --profile personal up -d
```

## Security Best Practices

### 1. Change Default Credentials (Team Edition)

```bash
# Never use default password in production
DEFAULT_ADMIN_PASSWORD=your_secure_password_here
```

### 2. Use Secrets for API Keys

```bash
# Use Docker secrets (Swarm/Compose)
echo "sk-ant-api03-..." | docker secret create anthropic_key -

# Reference in compose
services:
  gateway-team:
    secrets:
      - anthropic_key
    environment:
      - TOKLIGENCE_ANTHROPIC_API_KEY_FILE=/run/secrets/anthropic_key
```

### 3. Network Isolation

```bash
# Run in custom network
docker network create gateway-network
docker run --network gateway-network ...
```

### 4. Resource Limits

```bash
docker run -d \
  --memory="512m" \
  --cpus="1.0" \
  tokligence/gateway:personal
```

## Production Deployment

### Using Docker Swarm

```yaml
version: '3.8'
services:
  gateway:
    image: tokligence/gateway:0.3.0-team
    deploy:
      replicas: 2
      update_config:
        parallelism: 1
        delay: 10s
      restart_policy:
        condition: on-failure
    ports:
      - "8081:8081"
    environment:
      - TOKLIGENCE_ANTHROPIC_API_KEY_FILE=/run/secrets/anthropic_key
    secrets:
      - anthropic_key
    volumes:
      - gateway-data:/app/data

secrets:
  anthropic_key:
    external: true

volumes:
  gateway-data:
```

### Using Kubernetes

See `docs/KUBERNETES.md` for Kubernetes deployment guide.

## Support

- Issues: [GitHub Issues](https://github.com/tokligence/tokligence-gateway/issues)
- Documentation: [Full Documentation](../README.md)
- Docker Hub: https://hub.docker.com/r/tokligence/gateway
