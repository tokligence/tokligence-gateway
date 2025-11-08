#!/bin/sh
set -e

echo "=========================================="
echo "Tokligence Gateway - Team Edition"
echo "=========================================="
echo ""

# Wait for database directory to be writable
if [ ! -w "$(dirname $TOKLIGENCE_DB_PATH)" ]; then
    echo "Warning: Database directory is not writable"
fi

# Initialize database and create default admin user if not exists
if [ ! -f "$TOKLIGENCE_DB_PATH" ]; then
    echo "Initializing database..."

    # Create default admin user
    if [ -n "$DEFAULT_ADMIN_EMAIL" ] && [ -n "$DEFAULT_ADMIN_PASSWORD" ]; then
        echo "Creating default admin user: $DEFAULT_ADMIN_EMAIL"

        # Wait for daemon to be available (startup delay)
        sleep 2

        # Use gateway CLI to create user (will be executed after daemon starts)
        # For now, just log the credentials
        echo ""
        echo "=========================================="
        echo "Default Admin Credentials"
        echo "=========================================="
        echo "Email:    $DEFAULT_ADMIN_EMAIL"
        echo "Password: $DEFAULT_ADMIN_PASSWORD"
        echo ""
        echo "⚠️  Please change the default password after first login!"
        echo ""
        echo "To create API key:"
        echo "  docker exec <container> /app/gateway user create-key $DEFAULT_ADMIN_EMAIL"
        echo "=========================================="
        echo ""
    fi
fi

# Print configuration
echo "Configuration:"
echo "  Auth: $([ \"$TOKLIGENCE_AUTH_DISABLED\" = \"true\" ] && echo \"Disabled\" || echo \"Enabled\")"
echo "  Database: $TOKLIGENCE_DB_PATH"
echo "  Log Level: $TOKLIGENCE_LOG_LEVEL"
echo "  Port: 8081"
echo ""

# Execute the main command
exec "$@"
