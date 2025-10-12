package sqlite

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

const (
	apiKeyTokenPrefix  = "tok_"
	apiKeyPrefixLength = 12
)

// Store implements userstore.Store backed by SQLite.
type Store struct {
	db *sql.DB
}

// New opens (or creates) a SQLite user store at the supplied path.
func New(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create identity directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) initSchema() error {
	const baseSchema = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	email TEXT NOT NULL UNIQUE,
	role TEXT NOT NULL,
	display_name TEXT,
	status TEXT NOT NULL DEFAULT 'active',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_keys (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	key_hash TEXT NOT NULL UNIQUE,
	key_prefix TEXT NOT NULL,
	scopes TEXT,
	expires_at TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
`
	if _, err := s.db.Exec(baseSchema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	// Backfill legacy columns if the database existed prior to the schema changes.
	if err := s.ensureColumn("api_keys", "user_id", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("api_keys", "key_prefix", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureColumn(table, column, definition string) error {
	query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)
	if _, err := s.db.Exec(query); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return nil
		}
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}

// Close releases underlying resources.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) EnsureRootAdmin(ctx context.Context, email string) (*userstore.User, error) {
	email = normalizeEmail(email)
	if email == "" {
		email = "admin@local"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()

	row := tx.QueryRowContext(ctx, `SELECT id, email, role, display_name, status, created_at, updated_at FROM users WHERE role = ? LIMIT 1`, userstore.RoleRootAdmin)
	var u userstore.User
	var createdAt, updatedAt time.Time
	scanErr := row.Scan(&u.ID, &u.Email, &u.Role, &u.DisplayName, &u.Status, &createdAt, &updatedAt)
	if scanErr == nil {
		if !strings.EqualFold(u.Email, email) {
			if _, err = tx.ExecContext(ctx, `UPDATE users SET email = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, email, u.ID); err != nil {
				return nil, err
			}
			u.Email = email
		}
		u.CreatedAt = createdAt
		u.UpdatedAt = updatedAt
		if u.Status == "" {
			u.Status = userstore.StatusActive
		}
		return &u, nil
	}
	if scanErr != sql.ErrNoRows {
		return nil, scanErr
	}

	res, err := tx.ExecContext(ctx, `INSERT INTO users(email, role, status) VALUES(?, ?, ?)`, email, userstore.RoleRootAdmin, userstore.StatusActive)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &userstore.User{
		ID:        id,
		Email:     email,
		Role:      userstore.RoleRootAdmin,
		Status:    userstore.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *Store) FindByEmail(ctx context.Context, email string) (*userstore.User, error) {
	email = normalizeEmail(email)
	row := s.db.QueryRowContext(ctx, `SELECT id, email, role, display_name, status, created_at, updated_at FROM users WHERE email = ? LIMIT 1`, email)
	return scanUser(row)
}

func (s *Store) GetUser(ctx context.Context, id int64) (*userstore.User, error) {
	return s.userByID(ctx, id)
}

func (s *Store) ListUsers(ctx context.Context) ([]userstore.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, email, role, display_name, status, created_at, updated_at FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []userstore.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *user)
	}
	return users, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, email string, role userstore.Role, displayName string) (*userstore.User, error) {
	email = normalizeEmail(email)
	if email == "" {
		return nil, errors.New("email required")
	}
	if role != userstore.RoleGatewayAdmin && role != userstore.RoleGatewayUser {
		return nil, fmt.Errorf("unsupported role %s", role)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO users(email, role, display_name, status) VALUES(?, ?, ?, ?)`, email, role, displayName, userstore.StatusActive)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, fmt.Errorf("user with email %s already exists", email)
		}
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &userstore.User{
		ID:          id,
		Email:       email,
		Role:        role,
		DisplayName: displayName,
		Status:      userstore.StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (s *Store) UpdateUser(ctx context.Context, id int64, displayName string, role userstore.Role) (*userstore.User, error) {
	if role != userstore.RoleGatewayAdmin && role != userstore.RoleGatewayUser && role != userstore.RoleRootAdmin {
		return nil, fmt.Errorf("invalid role %s", role)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET display_name = ?, role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, displayName, role, id); err != nil {
		return nil, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, email, role, display_name, status, created_at, updated_at FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *Store) SetUserStatus(ctx context.Context, id int64, status userstore.Status) error {
	if status != userstore.StatusActive && status != userstore.StatusInactive {
		return fmt.Errorf("invalid status %s", status)
	}
	result, err := s.db.ExecContext(ctx, `UPDATE users SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CreateAPIKey(ctx context.Context, userID int64, scopes []string, expiresAt *time.Time) (*userstore.APIKey, string, error) {
	if userID == 0 {
		return nil, "", errors.New("user id required")
	}
	token, prefix, hash, err := generateAPIKey()
	if err != nil {
		return nil, "", err
	}
	var scopesJSON []byte
	if len(scopes) > 0 {
		scopesJSON, err = json.Marshal(scopes)
		if err != nil {
			return nil, "", err
		}
	}
	var expires interface{}
	if expiresAt != nil {
		expires = expiresAt.UTC()
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO api_keys(user_id, key_hash, key_prefix, scopes, expires_at) VALUES(?, ?, ?, ?, ?)`, userID, hash, prefix, string(scopesJSON), expires)
	if err != nil {
		return nil, "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC()
	apiKey := &userstore.APIKey{
		ID:        id,
		UserID:    userID,
		Prefix:    prefix,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return apiKey, token, nil
}

func (s *Store) ListAPIKeys(ctx context.Context, userID int64) ([]userstore.APIKey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, key_prefix, scopes, expires_at, created_at, updated_at FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []userstore.APIKey
	for rows.Next() {
		var key userstore.APIKey
		var scopesRaw sql.NullString
		var expires sql.NullTime
		if err := rows.Scan(&key.ID, &key.UserID, &key.Prefix, &scopesRaw, &expires, &key.CreatedAt, &key.UpdatedAt); err != nil {
			return nil, err
		}
		if scopesRaw.Valid && scopesRaw.String != "" {
			var scopes []string
			if err := json.Unmarshal([]byte(scopesRaw.String), &scopes); err == nil {
				key.Scopes = scopes
			}
		}
		if expires.Valid {
			t := expires.Time
			key.ExpiresAt = &t
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *Store) DeleteAPIKey(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM api_keys WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) LookupAPIKey(ctx context.Context, token string) (*userstore.APIKey, *userstore.User, error) {
	prefix, hash := deriveAPIKeyLookup(token)
	if hash == "" {
		return nil, nil, errors.New("invalid token")
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, user_id, key_prefix, scopes, expires_at, created_at, updated_at FROM api_keys WHERE key_prefix = ? AND key_hash = ? LIMIT 1`, prefix, hash)
	var key userstore.APIKey
	var scopesRaw sql.NullString
	var expires sql.NullTime
	if err := row.Scan(&key.ID, &key.UserID, &key.Prefix, &scopesRaw, &expires, &key.CreatedAt, &key.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if scopesRaw.Valid && scopesRaw.String != "" {
		var scopes []string
		_ = json.Unmarshal([]byte(scopesRaw.String), &scopes)
		key.Scopes = scopes
	}
	if expires.Valid {
		t := expires.Time
		key.ExpiresAt = &t
	}
	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return nil, nil, nil
	}
	user, err := s.userByID(ctx, key.UserID)
	if err != nil {
		return nil, nil, err
	}
	return &key, user, nil
}

func (s *Store) userByID(ctx context.Context, id int64) (*userstore.User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, email, role, display_name, status, created_at, updated_at FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func scanUser(scanner interface{ Scan(dest ...any) error }) (*userstore.User, error) {
	var u userstore.User
	var createdAt, updatedAt time.Time
	if err := scanner.Scan(&u.ID, &u.Email, &u.Role, &u.DisplayName, &u.Status, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	u.CreatedAt = createdAt
	u.UpdatedAt = updatedAt
	if u.Status == "" {
		u.Status = userstore.StatusActive
	}
	return &u, nil
}

func generateAPIKey() (token, prefix, hash string, err error) {
	var buf [32]byte
	if _, err = rand.Read(buf[:]); err != nil {
		return "", "", "", err
	}
	token = apiKeyTokenPrefix + base64.RawURLEncoding.EncodeToString(buf[:])
	if len(token) > apiKeyPrefixLength {
		prefix = token[:apiKeyPrefixLength]
	} else {
		prefix = token
	}
	sum := sha256.Sum256([]byte(token))
	hash = hex.EncodeToString(sum[:])
	return token, prefix, hash, nil
}

func deriveAPIKeyLookup(token string) (prefix, hash string) {
	if !strings.HasPrefix(token, apiKeyTokenPrefix) {
		return "", ""
	}
	sum := sha256.Sum256([]byte(token))
	hash = hex.EncodeToString(sum[:])
	if len(token) > apiKeyPrefixLength {
		prefix = token[:apiKeyPrefixLength]
	} else {
		prefix = token
	}
	return prefix, hash
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}
