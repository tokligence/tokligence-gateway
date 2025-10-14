# Tokligence Gateway Database Schema Design

## Overview

The Tokligence Gateway uses a multi-database architecture to separate concerns:
- **Identity Database**: Manages users and API keys
- **Ledger Database**: Tracks token usage and billing

Both databases support SQLite (for Community/OSS edition) and PostgreSQL (for Enterprise edition) with identical schemas.

## Core Design Principles

1. **Soft Delete by Default**: All delete operations set a `deleted_at` timestamp rather than removing records
2. **UUID Support**: All records have a UUID for distributed system compatibility
3. **Audit Trail**: All tables include `created_at`, `updated_at`, and `deleted_at` timestamps
4. **Database Agnostic**: Schema works identically on SQLite and PostgreSQL

## Identity Database Schema

### users Table

Stores user accounts for gateway access.

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE DEFAULT (uuid_generate_v4()),
    email TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL,
    display_name TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_deleted_at ON users(deleted_at);
```

**Fields:**
- `id`: Auto-incrementing primary key
- `uuid`: Globally unique identifier
- `email`: User's email address (unique)
- `role`: User role (root_admin, gateway_admin, gateway_user)
- `display_name`: Optional display name
- `status`: Account status (active, inactive)
- `created_at`: Record creation timestamp
- `updated_at`: Last modification timestamp
- `deleted_at`: Soft delete timestamp (NULL if active)

**Roles:**
- `root_admin`: Full system access, bypass marketplace verification
- `gateway_admin`: Can manage users and API keys
- `gateway_user`: Regular user access

### api_keys Table

Stores API keys for authentication.

```sql
CREATE TABLE api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE DEFAULT (uuid_generate_v4()),
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL,
    scopes TEXT,
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX idx_api_keys_deleted_at ON api_keys(deleted_at);
```

**Fields:**
- `id`: Auto-incrementing primary key
- `uuid`: Globally unique identifier
- `user_id`: Foreign key to users table
- `key_hash`: SHA-256 hash of the full API key
- `key_prefix`: First 12 characters of key for identification
- `scopes`: JSON array of allowed scopes
- `expires_at`: Optional expiration timestamp
- `created_at`: Key creation timestamp
- `updated_at`: Last modification timestamp
- `deleted_at`: Soft delete timestamp (NULL if active)

**Key Format:**
- Format: `tok_<32 random characters>`
- Only the hash is stored, original key shown once at creation

## Ledger Database Schema

### usage_entries Table

Tracks all token usage for billing and analytics.

```sql
CREATE TABLE usage_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE DEFAULT (uuid_generate_v4()),
    user_id INTEGER NOT NULL,
    api_key_id INTEGER,
    service_id INTEGER NOT NULL DEFAULT 0,
    prompt_tokens INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    direction TEXT NOT NULL CHECK(direction IN ('consume','supply')),
    memo TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_usage_entries_user_created ON usage_entries(user_id, created_at DESC);
CREATE INDEX idx_usage_entries_api_key_created ON usage_entries(api_key_id, created_at DESC);
CREATE INDEX idx_usage_entries_deleted_at ON usage_entries(deleted_at);
```

**Fields:**
- `id`: Auto-incrementing primary key
- `uuid`: Globally unique identifier
- `user_id`: User who consumed/supplied tokens
- `api_key_id`: Specific API key used (for tracking)
- `service_id`: Service/model identifier
- `prompt_tokens`: Number of prompt tokens
- `completion_tokens`: Number of completion tokens
- `direction`: 'consume' (using tokens) or 'supply' (providing tokens)
- `memo`: Optional description or metadata
- `created_at`: Transaction timestamp
- `updated_at`: Last modification timestamp
- `deleted_at`: Soft delete timestamp (NULL if active)

## Migration Strategy

### Schema Evolution
The system supports automatic schema migration on startup:
1. Tables are created if they don't exist
2. Missing columns are added to existing tables
3. Indexes are created or updated

### Soft Delete Implementation

All delete operations use soft delete by default:

```go
// Soft delete - sets deleted_at timestamp
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
    _, err := s.db.ExecContext(ctx,
        `UPDATE users SET deleted_at = CURRENT_TIMESTAMP,
         updated_at = CURRENT_TIMESTAMP
         WHERE id = ? AND deleted_at IS NULL`, id)
    return err
}

// Hard delete - permanent removal (requires explicit flag)
func (s *Store) HardDeleteUser(ctx context.Context, id int64) error {
    _, err := s.db.ExecContext(ctx,
        `DELETE FROM users WHERE id = ?`, id)
    return err
}
```

### Query Patterns

All queries must exclude soft-deleted records:

```sql
-- List active users
SELECT * FROM users WHERE deleted_at IS NULL;

-- Find user by email (excluding deleted)
SELECT * FROM users
WHERE email = ? AND deleted_at IS NULL;

-- Calculate usage (excluding deleted entries)
SELECT SUM(prompt_tokens + completion_tokens)
FROM usage_entries
WHERE user_id = ? AND deleted_at IS NULL;
```

## UUID Generation

### SQLite
Uses a complex expression to generate UUID v4:
```sql
DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' ||
substr(hex(randomblob(2)),2) || '-' ||
substr('89ab',abs(random()) % 4 + 1, 1) ||
substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6))))
```

### PostgreSQL
Uses native UUID generation:
```sql
DEFAULT uuid_generate_v4()
```

## Best Practices

1. **Always check deleted_at**: Every query must include `WHERE deleted_at IS NULL`
2. **Use transactions**: Multi-table operations should use transactions
3. **Index soft delete column**: The `deleted_at` column is indexed for performance
4. **Preserve history**: Soft delete preserves audit trail and allows recovery
5. **API key rotation**: Soft delete old keys when rotating
6. **Cascade carefully**: Foreign key cascades respect soft delete patterns

## CLI Operations

The gateway CLI supports both soft and hard delete:

```bash
# Soft delete (default) - marks as deleted
gateway admin users delete --id 123

# Hard delete - permanent removal
gateway admin users delete --id 123 --hard

# Same for API keys
gateway admin api-keys delete --id 456
gateway admin api-keys delete --id 456 --hard
```

## Future Considerations

1. **Partitioning**: usage_entries table may need partitioning for scale
2. **Archival**: Implement automated archival of old soft-deleted records
3. **Compliance**: Add data retention policies and GDPR compliance
4. **Replication**: Consider read replicas for high-volume deployments
