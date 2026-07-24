package sqlitestore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	idpstore "github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Store is the supported durable, single-active-node SQLite implementation of
// idpstore.Store.
type Store struct {
	db         *sql.DB
	runner     sqlRunner
	mu         *sync.Mutex
	path       string
	backupCopy func(context.Context, *sql.Conn, string) error
}

type sqlRunner interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

var _ idpstore.Store = (*Store)(nil)
var _ idpstore.MaintenanceStore = (*Store)(nil)

// Config defines the SQLite file and durability policy. The supported
// production envelope uses exactly one open connection on a local filesystem.
type Config struct {
	Path               string
	BusyTimeout        time.Duration
	JournalMode        string
	Synchronous        string
	MaxOpenConnections int
}

// DefaultConfig returns WAL, synchronous=FULL, a five-second busy timeout, and
// the required single connection.
func DefaultConfig(path string) Config {
	return Config{
		Path:               path,
		BusyTimeout:        5 * time.Second,
		JournalMode:        "WAL",
		Synchronous:        "FULL",
		MaxOpenConnections: 1,
	}
}

// Open creates or opens the database, applies checksummed migrations, and
// enforces owner-only permissions on the database and SQLite sidecars.
func Open(ctx context.Context, cfg Config) (*Store, error) {
	if strings.TrimSpace(cfg.Path) == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	defaults := DefaultConfig(cfg.Path)
	if cfg.BusyTimeout <= 0 {
		cfg.BusyTimeout = defaults.BusyTimeout
	}
	if cfg.JournalMode == "" {
		cfg.JournalMode = defaults.JournalMode
	}
	if cfg.Synchronous == "" {
		cfg.Synchronous = defaults.Synchronous
	}
	if cfg.MaxOpenConnections <= 0 {
		cfg.MaxOpenConnections = defaults.MaxOpenConnections
	}
	if cfg.MaxOpenConnections != 1 {
		return nil, fmt.Errorf("SQLite store supports exactly one open connection; got %d", cfg.MaxOpenConnections)
	}
	cfg.JournalMode = strings.ToUpper(cfg.JournalMode)
	cfg.Synchronous = strings.ToUpper(cfg.Synchronous)
	if !oneOf(cfg.JournalMode, "WAL", "DELETE", "TRUNCATE") {
		return nil, fmt.Errorf("unsupported SQLite journal mode %q", cfg.JournalMode)
	}
	if !oneOf(cfg.Synchronous, "FULL", "NORMAL", "EXTRA") {
		return nil, fmt.Errorf("unsupported SQLite synchronous policy %q", cfg.Synchronous)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o700); err != nil {
		return nil, fmt.Errorf("create SQLite directory: %w", err)
	}
	f, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create SQLite database: %w", err)
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("set SQLite database permissions: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("close SQLite database bootstrap handle: %w", err)
	}
	db, err := sql.Open("sqlite3", cfg.Path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.MaxOpenConnections)
	db.SetMaxIdleConns(cfg.MaxOpenConnections)
	st := &Store{db: db, mu: &sync.Mutex{}, path: cfg.Path}
	pragmas := []string{
		fmt.Sprintf("PRAGMA busy_timeout=%d", cfg.BusyTimeout.Milliseconds()),
		"PRAGMA foreign_keys=ON",
		"PRAGMA journal_mode=" + cfg.JournalMode,
		"PRAGMA synchronous=" + cfg.Synchronous,
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("configure SQLite with %q: %w", pragma, err)
		}
	}
	if err := st.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := enforceFilesOwnerOnly(cfg.Path); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func enforceFilesOwnerOnly(path string) error {
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(candidate, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("set owner-only permissions on %s: %w", candidate, err)
		}
	}
	return nil
}

func (s *Store) Close() error     { return s.db.Close() }
func (s *Store) Persistent() bool { return true }

func (s *Store) conn() sqlRunner {
	if s.runner != nil {
		return s.runner
	}
	return s.db
}

func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var version sql.NullInt64
	if err := s.conn().QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, err
	}
	return int(version.Int64), nil
}

func (s *Store) SupportedSchemaVersion() int {
	names, err := MigrationNames()
	if err != nil {
		return 0
	}
	return len(names)
}

// SQLDB exposes the underlying database to adapter packages that need to store
// protocol-specific state while reusing the same SQLite file and transaction
// durability. Callers must not close the returned handle.
func (s *Store) SQLDB() *sql.DB { return s.db }

// MigrationNames returns embedded migration names after verifying contiguous,
// monotonically increasing numeric versions.
func MigrationNames() ([]string, error) {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	for index, name := range names {
		version, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", name, err)
		}
		if version != index+1 {
			return nil, fmt.Errorf("migration %q has version %d; expected contiguous version %d", name, version, index+1)
		}
	}
	return names, nil
}

// Migrate verifies prior checksums and applies each pending migration in its
// own transaction.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY,
        name TEXT NOT NULL UNIQUE,
        checksum TEXT NOT NULL,
        applied_at TIMESTAMP NOT NULL
    )`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}
	names, err := MigrationNames()
	if err != nil {
		return err
	}
	var current sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read current schema version: %w", err)
	}
	if current.Valid && current.Int64 > int64(len(names)) {
		return fmt.Errorf("database schema version %d is newer than supported version %d", current.Int64, len(names))
	}
	for _, name := range names {
		version, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil {
			return fmt.Errorf("parse migration version %q: %w", name, err)
		}
		b, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		checksum := hex.EncodeToString(sum[:])
		var existing string
		err = s.db.QueryRowContext(ctx, `SELECT checksum FROM schema_migrations WHERE version=?`, version).Scan(&existing)
		if err == nil {
			if existing != checksum {
				return fmt.Errorf("migration %s checksum mismatch: database=%s embedded=%s", name, existing, checksum)
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("read migration %s state: %w", name, err)
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(b)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,name,checksum,applied_at) VALUES(?,?,?,?)`, version, name, checksum, time.Now().UTC()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

func hashKey(b []byte) string        { return hex.EncodeToString(b) }
func enc(v any) ([]byte, error)      { return json.Marshal(v) }
func dec[T any](b []byte) (T, error) { var v T; err := json.Unmarshal(b, &v); return v, err }

func (s *Store) PutClient(ctx context.Context, c idpstore.Client) error {
	b, _ := enc(c)
	_, err := s.conn().ExecContext(ctx, `INSERT OR REPLACE INTO clients(id,data) VALUES(?,?)`, c.ID, b)
	return err
}
func (s *Store) GetClient(ctx context.Context, id string) (idpstore.Client, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM clients WHERE id=?`, id).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.Client{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.Client{}, err
	}
	return dec[idpstore.Client](b)
}
func (s *Store) ListClients(ctx context.Context) ([]idpstore.Client, error) {
	rows, err := s.conn().QueryContext(ctx, `SELECT data FROM clients ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []idpstore.Client
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		c, err := dec[idpstore.Client](b)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) PutUser(ctx context.Context, login string, u idpstore.User) error {
	b, _ := enc(u)
	_, err := s.conn().ExecContext(ctx, `INSERT INTO users(id,login,subject,data) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET login=excluded.login,subject=excluded.subject,data=excluded.data`, u.ID, login, u.Sub, b)
	return mapDup(err)
}

func (s *Store) DisplayNameAvailable(ctx context.Context, normalized string) (bool, error) {
	var claimed int
	err := s.conn().QueryRowContext(ctx, `SELECT 1 FROM display_name_claims WHERE normalized_name=?`, normalized).Scan(&claimed)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}

func (s *Store) ReserveDisplayName(ctx context.Context, normalized, userID string) error {
	_, err := s.conn().ExecContext(ctx, `INSERT INTO display_name_claims(normalized_name,user_id) VALUES(?,?)`, normalized, userID)
	if err != nil {
		if mapDup(err) == idpstore.ErrDuplicate {
			return idpstore.ErrDisplayNameTaken
		}
		return err
	}
	return nil
}
func (s *Store) GetUser(ctx context.Context, id string) (idpstore.User, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM users WHERE id=?`, id).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.User{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.User{}, err
	}
	return dec[idpstore.User](b)
}
func (s *Store) GetUserByLogin(ctx context.Context, login string) (idpstore.User, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM users WHERE login=?`, login).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.User{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.User{}, err
	}
	return dec[idpstore.User](b)
}

func (s *Store) GetUserBySubject(ctx context.Context, subject string) (idpstore.User, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM users WHERE subject=?`, subject).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.User{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.User{}, err
	}
	return dec[idpstore.User](b)
}

func (s *Store) PutPasswordCredential(ctx context.Context, credential idpstore.PasswordCredential) error {
	if existing, err := s.GetPasswordCredentialByLogin(ctx, credential.Login); err == nil && existing.UserID != credential.UserID {
		return idpstore.ErrDuplicate
	} else if err != nil && err != idpstore.ErrNotFound {
		return err
	}
	b, _ := enc(credential)
	_, err := s.conn().ExecContext(ctx, `INSERT OR REPLACE INTO password_credentials(user_id,login,data) VALUES(?,?,?)`, credential.UserID, credential.Login, b)
	return mapDup(err)
}
func (s *Store) GetPasswordCredentialByLogin(ctx context.Context, login string) (idpstore.PasswordCredential, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM password_credentials WHERE login=?`, login).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.PasswordCredential{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.PasswordCredential{}, err
	}
	return dec[idpstore.PasswordCredential](b)
}
func (s *Store) GetPasswordCredentialByUserID(ctx context.Context, userID string) (idpstore.PasswordCredential, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM password_credentials WHERE user_id=?`, userID).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.PasswordCredential{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.PasswordCredential{}, err
	}
	return dec[idpstore.PasswordCredential](b)
}
func (s *Store) DeletePasswordCredential(ctx context.Context, userID string) error {
	res, err := s.conn().ExecContext(ctx, `DELETE FROM password_credentials WHERE user_id=?`, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return idpstore.ErrNotFound
	}
	return nil
}
func (s *Store) GetAccountSecurityState(ctx context.Context, userID string) (idpstore.AccountSecurityState, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM account_security_states WHERE user_id=?`, userID).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.AccountSecurityState{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.AccountSecurityState{}, err
	}
	return dec[idpstore.AccountSecurityState](b)
}
func (s *Store) PutAccountSecurityState(ctx context.Context, state idpstore.AccountSecurityState) error {
	b, _ := enc(state)
	_, err := s.conn().ExecContext(ctx, `INSERT OR REPLACE INTO account_security_states(user_id,data) VALUES(?,?)`, state.UserID, b)
	return err
}
func (s *Store) ResetAccountSecurityState(ctx context.Context, userID string, now time.Time) error {
	state := idpstore.AccountSecurityState{UserID: userID, LastSuccessfulLoginAt: &now}
	return s.PutAccountSecurityState(ctx, state)
}

func (s *Store) CreateGrant(ctx context.Context, g idpstore.Grant) error {
	b, _ := enc(g)
	_, err := s.conn().ExecContext(ctx, `INSERT INTO grants(id,data) VALUES(?,?)`, g.ID, b)
	return mapDup(err)
}
func (s *Store) GetGrant(ctx context.Context, id string) (idpstore.Grant, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM grants WHERE id=?`, id).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.Grant{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.Grant{}, err
	}
	return dec[idpstore.Grant](b)
}
func (s *Store) RevokeGrant(ctx context.Context, id string, at time.Time) error {
	g, err := s.GetGrant(ctx, id)
	if err != nil {
		return err
	}
	g.RevokedAt = &at
	b, _ := enc(g)
	_, err = s.conn().ExecContext(ctx, `UPDATE grants SET data=? WHERE id=?`, b, id)
	return err
}

func (s *Store) CreateAuthorizationCode(ctx context.Context, c idpstore.AuthorizationCode) error {
	b, _ := enc(c)
	_, err := s.conn().ExecContext(ctx, `INSERT INTO authorization_codes(hash,data) VALUES(?,?)`, hashKey(c.CodeHash), b)
	return mapDup(err)
}
func (s *Store) ConsumeAuthorizationCode(ctx context.Context, codeHash []byte, now time.Time) (idpstore.AuthorizationCode, error) {
	if s.runner == nil {
		var code idpstore.AuthorizationCode
		err := s.Update(ctx, func(tx idpstore.TxStore) error {
			var err error
			code, err = tx.ConsumeAuthorizationCode(ctx, codeHash, now)
			return err
		})
		return code, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := hashKey(codeHash)
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM authorization_codes WHERE hash=?`, k).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.AuthorizationCode{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.AuthorizationCode{}, err
	}
	c, err := dec[idpstore.AuthorizationCode](b)
	if err != nil {
		return idpstore.AuthorizationCode{}, err
	}
	if c.ConsumedAt != nil {
		return idpstore.AuthorizationCode{}, idpstore.ErrAlreadyConsumed
	}
	if !c.ExpiresAt.IsZero() && now.After(c.ExpiresAt) {
		return idpstore.AuthorizationCode{}, idpstore.ErrExpired
	}
	c.ConsumedAt = &now
	nb, _ := enc(c)
	_, err = s.conn().ExecContext(ctx, `UPDATE authorization_codes SET data=? WHERE hash=?`, nb, k)
	return c, err
}

func (s *Store) CreateAccessToken(ctx context.Context, t idpstore.AccessToken) error {
	b, _ := enc(t)
	_, err := s.conn().ExecContext(ctx, `INSERT INTO access_tokens(hash,data) VALUES(?,?)`, hashKey(t.TokenHash), b)
	return mapDup(err)
}
func (s *Store) GetAccessToken(ctx context.Context, tokenHash []byte) (idpstore.AccessToken, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM access_tokens WHERE hash=?`, hashKey(tokenHash)).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.AccessToken{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.AccessToken{}, err
	}
	return dec[idpstore.AccessToken](b)
}
func (s *Store) RevokeAccessToken(ctx context.Context, tokenHash []byte, at time.Time) error {
	t, err := s.GetAccessToken(ctx, tokenHash)
	if err != nil {
		return err
	}
	t.RevokedAt = &at
	b, _ := enc(t)
	_, err = s.conn().ExecContext(ctx, `UPDATE access_tokens SET data=? WHERE hash=?`, b, hashKey(tokenHash))
	return err
}

func (s *Store) CreateRefreshToken(ctx context.Context, t idpstore.RefreshToken) error {
	b, _ := enc(t)
	_, err := s.conn().ExecContext(ctx, `INSERT INTO refresh_tokens(hash,grant_id,data) VALUES(?,?,?)`, hashKey(t.TokenHash), t.GrantID, b)
	return mapDup(err)
}
func (s *Store) GetRefreshToken(ctx context.Context, tokenHash []byte) (idpstore.RefreshToken, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM refresh_tokens WHERE hash=?`, hashKey(tokenHash)).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.RefreshToken{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.RefreshToken{}, err
	}
	return dec[idpstore.RefreshToken](b)
}
func (s *Store) RotateRefreshToken(ctx context.Context, oldHash []byte, next idpstore.RefreshToken, now time.Time) (idpstore.RefreshToken, error) {
	if s.runner == nil {
		var rotated idpstore.RefreshToken
		var outcomeErr error
		err := s.Update(ctx, func(tx idpstore.TxStore) error {
			rotated, outcomeErr = tx.RotateRefreshToken(ctx, oldHash, next, now)
			if errors.Is(outcomeErr, idpstore.ErrRefreshReuseDetected) {
				return nil
			}
			return outcomeErr
		})
		if err != nil {
			return idpstore.RefreshToken{}, err
		}
		return rotated, outcomeErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := s.GetRefreshToken(ctx, oldHash)
	if err != nil {
		return idpstore.RefreshToken{}, err
	}
	if old.RevokedAt != nil || len(old.ReplacedByHash) > 0 || old.ReuseDetectedAt != nil {
		detected := now
		old.ReuseDetectedAt = &detected
		_ = s.putRefresh(ctx, old)
		_ = s.revokeFamily(ctx, old.GrantID, now)
		return idpstore.RefreshToken{}, idpstore.ErrRefreshReuseDetected
	}
	if !old.ExpiresAt.IsZero() && now.After(old.ExpiresAt) {
		return idpstore.RefreshToken{}, idpstore.ErrExpired
	}
	next.ParentTokenHash = append([]byte(nil), oldHash...)
	old.ReplacedByHash = append([]byte(nil), next.TokenHash...)
	if err := s.putRefresh(ctx, old); err != nil {
		return idpstore.RefreshToken{}, err
	}
	if err := s.CreateRefreshToken(ctx, next); err != nil {
		return idpstore.RefreshToken{}, err
	}
	return next, nil
}
func (s *Store) putRefresh(ctx context.Context, t idpstore.RefreshToken) error {
	b, _ := enc(t)
	_, err := s.conn().ExecContext(ctx, `UPDATE refresh_tokens SET grant_id=?, data=? WHERE hash=?`, t.GrantID, b, hashKey(t.TokenHash))
	return err
}
func (s *Store) RevokeRefreshTokenFamily(ctx context.Context, tokenHash []byte, at time.Time) error {
	if s.runner == nil {
		return s.Update(ctx, func(tx idpstore.TxStore) error {
			return tx.RevokeRefreshTokenFamily(ctx, tokenHash, at)
		})
	}
	t, err := s.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return err
	}
	return s.revokeFamily(ctx, t.GrantID, at)
}

// revokeFamily runs only on a transaction-scoped Store.
//
// tinyidp:transaction-scoped
func (s *Store) revokeFamily(ctx context.Context, grantID string, at time.Time) error {
	rows, err := s.conn().QueryContext(ctx, `SELECT data FROM refresh_tokens WHERE grant_id=?`, grantID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var toks []idpstore.RefreshToken
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return err
		}
		t, _ := dec[idpstore.RefreshToken](b)
		toks = append(toks, t)
	}
	for _, t := range toks {
		if t.RevokedAt == nil {
			t.RevokedAt = &at
			if err := s.putRefresh(ctx, t); err != nil {
				return err
			}
		}
	}
	return rows.Err()
}

func consentKey(userID, clientID string, scopes []string) string {
	parts := append([]string{userID, clientID}, idpstore.NormalizeScopes(scopes)...)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func (s *Store) PutConsent(ctx context.Context, consent idpstore.Consent) error {
	consent.Scope = idpstore.NormalizeScopes(consent.Scope)
	b, _ := enc(consent)
	_, err := s.conn().ExecContext(ctx, `INSERT OR REPLACE INTO consents(key,user_id,client_id,data) VALUES(?,?,?,?)`, consentKey(consent.UserID, consent.ClientID, consent.Scope), consent.UserID, consent.ClientID, b)
	return err
}
func (s *Store) GetConsent(ctx context.Context, userID, clientID string, scopes []string) (idpstore.Consent, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM consents WHERE key=?`, consentKey(userID, clientID, scopes)).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.Consent{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.Consent{}, err
	}
	return dec[idpstore.Consent](b)
}
func (s *Store) RevokeConsent(ctx context.Context, userID, clientID string, scopes []string, at time.Time) error {
	c, err := s.GetConsent(ctx, userID, clientID, scopes)
	if err != nil {
		return err
	}
	c.RevokedAt = &at
	b, _ := enc(c)
	_, err = s.conn().ExecContext(ctx, `UPDATE consents SET data=? WHERE key=?`, b, consentKey(userID, clientID, scopes))
	return err
}

func (s *Store) CreateSession(ctx context.Context, sess idpstore.Session) error {
	b, _ := enc(sess)
	_, err := s.conn().ExecContext(ctx, `INSERT INTO sessions(hash,data) VALUES(?,?)`, hashKey(sess.IDHash), b)
	return mapDup(err)
}
func (s *Store) GetSession(ctx context.Context, idHash []byte) (idpstore.Session, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM sessions WHERE hash=?`, hashKey(idHash)).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.Session{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.Session{}, err
	}
	return dec[idpstore.Session](b)
}
func (s *Store) RevokeSession(ctx context.Context, idHash []byte, at time.Time) error {
	sess, err := s.GetSession(ctx, idHash)
	if err != nil {
		return err
	}
	sess.RevokedAt = &at
	b, _ := enc(sess)
	_, err = s.conn().ExecContext(ctx, `UPDATE sessions SET data=? WHERE hash=?`, b, hashKey(idHash))
	return err
}

func (s *Store) CreateBrowserContext(ctx context.Context, browserContext idpstore.BrowserContext) error {
	data, err := enc(browserContext)
	if err != nil {
		return err
	}
	_, err = s.conn().ExecContext(ctx, `INSERT INTO browser_contexts(hash,expires_at,revoked_at,data) VALUES(?,?,?,?)`, hashKey(browserContext.IDHash), browserContext.ExpiresAt.UTC(), browserContext.RevokedAt, data)
	return mapDup(err)
}

func (s *Store) GetBrowserContext(ctx context.Context, contextHash []byte) (idpstore.BrowserContext, error) {
	var data []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM browser_contexts WHERE hash=?`, hashKey(contextHash)).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return idpstore.BrowserContext{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.BrowserContext{}, err
	}
	return dec[idpstore.BrowserContext](data)
}

func (s *Store) CreateRememberedBrowserSession(ctx context.Context, remembered idpstore.RememberedBrowserSession) error {
	data, err := enc(remembered)
	if err != nil {
		return err
	}
	_, err = s.conn().ExecContext(ctx, `INSERT INTO remembered_browser_sessions(hash,context_hash,session_hash,user_id,removed_at,last_used_at,data) VALUES(?,?,?,?,?,?,?)`, hashKey(remembered.IDHash), hashKey(remembered.ContextIDHash), hashKey(remembered.SessionIDHash), remembered.UserID, remembered.RemovedAt, remembered.LastUsedAt.UTC(), data)
	return mapDup(err)
}

func (s *Store) ListRememberedBrowserSessions(ctx context.Context, contextHash []byte, now time.Time) ([]idpstore.RememberedBrowserSession, error) {
	if !s.browserContextActive(ctx, contextHash, now) {
		return nil, idpstore.ErrNotFound
	}
	rows, err := s.conn().QueryContext(ctx, `SELECT data FROM remembered_browser_sessions WHERE context_hash=? AND removed_at IS NULL ORDER BY last_used_at DESC, hash ASC`, hashKey(contextHash))
	if err != nil {
		return nil, err
	}
	candidates := make([]idpstore.RememberedBrowserSession, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		remembered, err := dec[idpstore.RememberedBrowserSession](data)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, remembered)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	entries := make([]idpstore.RememberedBrowserSession, 0, len(candidates))
	for _, remembered := range candidates {
		session, err := s.GetSession(ctx, remembered.SessionIDHash)
		if err != nil || !sessionActiveAt(session, now) || session.UserID != remembered.UserID {
			continue
		}
		user, err := s.GetUser(ctx, remembered.UserID)
		if err != nil || user.Disabled {
			continue
		}
		entries = append(entries, remembered)
	}
	return entries, nil
}

func (s *Store) ActivateRememberedSession(ctx context.Context, contextHash, entryHash, newSessionHash []byte, now time.Time) (idpstore.Session, idpstore.User, error) {
	if s.runner == nil {
		var session idpstore.Session
		var user idpstore.User
		err := s.Update(ctx, func(tx idpstore.TxStore) error {
			scoped, ok := tx.(*Store)
			if !ok {
				return fmt.Errorf("unexpected SQLite transaction implementation")
			}
			var err error
			session, user, err = scoped.activateRememberedSession(ctx, contextHash, entryHash, newSessionHash, now)
			return err
		})
		return session, user, err
	}
	return s.activateRememberedSession(ctx, contextHash, entryHash, newSessionHash, now)
}

// activateRememberedSession is called only through ActivateRememberedSession,
// with a transaction-scoped Store runner.
//
// tinyidp:transaction-scoped
func (s *Store) activateRememberedSession(ctx context.Context, contextHash, entryHash, newSessionHash []byte, now time.Time) (idpstore.Session, idpstore.User, error) {
	now = now.UTC()
	if !s.browserContextActive(ctx, contextHash, now) {
		return idpstore.Session{}, idpstore.User{}, idpstore.ErrNotFound
	}
	var data []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM remembered_browser_sessions WHERE hash=? AND context_hash=? AND removed_at IS NULL`, hashKey(entryHash), hashKey(contextHash)).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return idpstore.Session{}, idpstore.User{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.Session{}, idpstore.User{}, err
	}
	remembered, err := dec[idpstore.RememberedBrowserSession](data)
	if err != nil {
		return idpstore.Session{}, idpstore.User{}, err
	}
	source, err := s.GetSession(ctx, remembered.SessionIDHash)
	if errors.Is(err, idpstore.ErrNotFound) || !sessionActiveAt(source, now) || source.UserID != remembered.UserID {
		return idpstore.Session{}, idpstore.User{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.Session{}, idpstore.User{}, err
	}
	user, err := s.GetUser(ctx, source.UserID)
	if errors.Is(err, idpstore.ErrNotFound) || user.Disabled {
		return idpstore.Session{}, idpstore.User{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.Session{}, idpstore.User{}, err
	}
	active := source
	active.IDHash = append([]byte(nil), newSessionHash...)
	active.CreatedAt = now
	active.LastSeenAt = now
	active.RevokedAt = nil
	if err := s.CreateSession(ctx, active); err != nil {
		return idpstore.Session{}, idpstore.User{}, err
	}
	remembered.LastUsedAt = now
	data, err = enc(remembered)
	if err != nil {
		return idpstore.Session{}, idpstore.User{}, err
	}
	if _, err := s.conn().ExecContext(ctx, `UPDATE remembered_browser_sessions SET last_used_at=?,data=? WHERE hash=? AND context_hash=? AND removed_at IS NULL`, now, data, hashKey(entryHash), hashKey(contextHash)); err != nil {
		return idpstore.Session{}, idpstore.User{}, err
	}
	browserContext, err := s.GetBrowserContext(ctx, contextHash)
	if err != nil {
		return idpstore.Session{}, idpstore.User{}, err
	}
	browserContext.LastSeenAt = now
	contextData, err := enc(browserContext)
	if err != nil {
		return idpstore.Session{}, idpstore.User{}, err
	}
	if _, err := s.conn().ExecContext(ctx, `UPDATE browser_contexts SET data=? WHERE hash=?`, contextData, hashKey(contextHash)); err != nil {
		return idpstore.Session{}, idpstore.User{}, err
	}
	return active, user, nil
}

func (s *Store) RemoveRememberedBrowserSession(ctx context.Context, contextHash, entryHash []byte, at time.Time) error {
	if s.runner == nil {
		return s.Update(ctx, func(tx idpstore.TxStore) error {
			scoped, ok := tx.(*Store)
			if !ok {
				return fmt.Errorf("unexpected SQLite transaction implementation")
			}
			return scoped.RemoveRememberedBrowserSession(ctx, contextHash, entryHash, at)
		})
	}
	if !s.browserContextActive(ctx, contextHash, at) {
		return idpstore.ErrNotFound
	}
	at = at.UTC()
	var data []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM remembered_browser_sessions WHERE hash=? AND context_hash=? AND removed_at IS NULL`, hashKey(entryHash), hashKey(contextHash)).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return idpstore.ErrNotFound
	}
	if err != nil {
		return err
	}
	remembered, err := dec[idpstore.RememberedBrowserSession](data)
	if err != nil {
		return err
	}
	remembered.RemovedAt = &at
	data, err = enc(remembered)
	if err != nil {
		return err
	}
	result, err := s.conn().ExecContext(ctx, `UPDATE remembered_browser_sessions SET removed_at=?,data=? WHERE hash=? AND context_hash=? AND removed_at IS NULL`, at, data, hashKey(entryHash), hashKey(contextHash))
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return idpstore.ErrNotFound
	}
	return nil
}

func (s *Store) RevokeBrowserContext(ctx context.Context, contextHash []byte, at time.Time) error {
	if s.runner == nil {
		return s.Update(ctx, func(tx idpstore.TxStore) error {
			scoped, ok := tx.(*Store)
			if !ok {
				return fmt.Errorf("unexpected SQLite transaction implementation")
			}
			return scoped.RevokeBrowserContext(ctx, contextHash, at)
		})
	}
	at = at.UTC()
	browserContext, err := s.GetBrowserContext(ctx, contextHash)
	if errors.Is(err, idpstore.ErrNotFound) {
		return idpstore.ErrNotFound
	}
	if err != nil {
		return err
	}
	browserContext.RevokedAt = &at
	data, err := enc(browserContext)
	if err != nil {
		return err
	}
	result, err := s.conn().ExecContext(ctx, `UPDATE browser_contexts SET revoked_at=?,data=? WHERE hash=?`, at, data, hashKey(contextHash))
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return idpstore.ErrNotFound
	}
	return nil
}

func (s *Store) browserContextActive(ctx context.Context, contextHash []byte, now time.Time) bool {
	browserContext, err := s.GetBrowserContext(ctx, contextHash)
	return err == nil && browserContext.RevokedAt == nil && (browserContext.ExpiresAt.IsZero() || now.Before(browserContext.ExpiresAt))
}

func sessionActiveAt(session idpstore.Session, now time.Time) bool {
	return session.RevokedAt == nil && (session.ExpiresAt.IsZero() || now.Before(session.ExpiresAt))
}

func (s *Store) CreateInteraction(ctx context.Context, interaction idpstore.InteractionRecord) error {
	data, err := enc(interaction)
	if err != nil {
		return err
	}
	_, err = s.conn().ExecContext(ctx, `INSERT INTO authorization_interactions(hash,expires_at,consumed_at,data) VALUES(?,?,NULL,?)`, hashKey(interaction.IDHash), interaction.ExpiresAt.UTC(), data)
	return mapDup(err)
}

func (s *Store) GetInteraction(ctx context.Context, idHash []byte) (idpstore.InteractionRecord, error) {
	var data []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM authorization_interactions WHERE hash=?`, hashKey(idHash)).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return idpstore.InteractionRecord{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.InteractionRecord{}, err
	}
	return dec[idpstore.InteractionRecord](data)
}

func (s *Store) ConsumeInteraction(ctx context.Context, idHash []byte, now time.Time, outcome idpstore.InteractionOutcome) (idpstore.InteractionRecord, error) {
	if !outcome.Valid() {
		return idpstore.InteractionRecord{}, idpstore.ErrInvalidInteractionOutcome
	}
	if s.runner == nil {
		var consumed idpstore.InteractionRecord
		err := s.Update(ctx, func(tx idpstore.TxStore) error {
			var err error
			consumed, err = tx.ConsumeInteraction(ctx, idHash, now, outcome)
			return err
		})
		return consumed, err
	}
	interaction, err := s.GetInteraction(ctx, idHash)
	if err != nil {
		return idpstore.InteractionRecord{}, err
	}
	if interaction.ConsumedAt != nil {
		return idpstore.InteractionRecord{}, idpstore.ErrAlreadyConsumed
	}
	if !interaction.ExpiresAt.IsZero() && !now.Before(interaction.ExpiresAt) {
		return idpstore.InteractionRecord{}, idpstore.ErrExpired
	}
	now = now.UTC()
	interaction.ConsumedAt = &now
	interaction.Outcome = outcome
	data, err := enc(interaction)
	if err != nil {
		return idpstore.InteractionRecord{}, err
	}
	result, err := s.conn().ExecContext(ctx, `UPDATE authorization_interactions SET consumed_at=?,data=? WHERE hash=? AND consumed_at IS NULL`, now, data, hashKey(idHash))
	if err != nil {
		return idpstore.InteractionRecord{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return idpstore.InteractionRecord{}, err
	}
	if count != 1 {
		return idpstore.InteractionRecord{}, idpstore.ErrAlreadyConsumed
	}
	return interaction, nil
}

const deviceGrantSlowDownIncrement = 5 * time.Second

func (s *Store) CreateDeviceGrant(ctx context.Context, grant idpstore.DeviceGrant) error {
	if err := grant.ValidateForCreate(); err != nil {
		return err
	}
	grant.CreatedAt = grant.CreatedAt.UTC()
	grant.ExpiresAt = grant.ExpiresAt.UTC()
	grant.NextPollAt = grant.NextPollAt.UTC()
	grant.Version = 1
	data, err := enc(grant)
	if err != nil {
		return err
	}
	_, err = s.conn().ExecContext(ctx, `INSERT INTO device_grants(id,device_code_hash,user_code_hash,client_id,status,expires_at,next_poll_at,data) VALUES(?,?,?,?,?,?,?,?)`, grant.ID, hashKey(grant.DeviceCodeHash), hashKey(grant.UserCodeHash), grant.ClientID, grant.Status, grant.ExpiresAt, grant.NextPollAt, data)
	return mapDup(err)
}

func (s *Store) GetDeviceGrantByUserCodeHash(ctx context.Context, userCodeHash []byte) (idpstore.DeviceGrant, error) {
	return s.loadDeviceGrant(ctx, `SELECT data FROM device_grants WHERE user_code_hash=?`, hashKey(userCodeHash))
}

func (s *Store) InspectDeviceGrantByDeviceCodeHash(ctx context.Context, deviceCodeHash []byte, clientID string) (idpstore.DeviceGrant, error) {
	return s.loadDeviceGrant(ctx, `SELECT data FROM device_grants WHERE device_code_hash=? AND client_id=?`, hashKey(deviceCodeHash), clientID)
}

func (s *Store) loadDeviceGrant(ctx context.Context, query string, args ...any) (idpstore.DeviceGrant, error) {
	var data []byte
	err := s.conn().QueryRowContext(ctx, query, args...).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return idpstore.DeviceGrant{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.DeviceGrant{}, err
	}
	return dec[idpstore.DeviceGrant](data)
}

func (s *Store) PollDeviceGrant(ctx context.Context, request idpstore.DevicePollRequest) (idpstore.DevicePollResult, error) {
	if s.runner == nil {
		var result idpstore.DevicePollResult
		err := s.Update(ctx, func(tx idpstore.TxStore) error {
			var err error
			result, err = tx.PollDeviceGrant(ctx, request)
			return err
		})
		return result, err
	}
	if request.Now.IsZero() {
		return idpstore.DevicePollResult{}, idpstore.ErrInvalidDeviceGrant
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, err := s.InspectDeviceGrantByDeviceCodeHash(ctx, request.DeviceCodeHash, request.ClientID)
	if err != nil {
		return idpstore.DevicePollResult{}, err
	}
	now := request.Now.UTC()
	if deviceGrantExpired(grant, now) {
		return idpstore.DevicePollResult{Outcome: idpstore.DevicePollExpired, Grant: grant}, nil
	}
	switch grant.Status {
	case idpstore.DeviceGrantDenied:
		return idpstore.DevicePollResult{Outcome: idpstore.DevicePollDenied, Grant: grant}, nil
	case idpstore.DeviceGrantConsumed:
		return idpstore.DevicePollResult{Outcome: idpstore.DevicePollConsumed, Grant: grant}, nil
	case idpstore.DeviceGrantPending, idpstore.DeviceGrantApproved:
	default:
		return idpstore.DevicePollResult{}, idpstore.ErrInvalidDeviceGrant
	}
	if now.Before(grant.NextPollAt) {
		grant.PollInterval += deviceGrantSlowDownIncrement
		grant.NextPollAt = now.Add(grant.PollInterval)
		grant.SlowDownCount++
		grant.Version++
		if err := s.updateDevicePoll(ctx, grant, request.ClientID, now, "status IN ('pending','approved')"); err != nil {
			return idpstore.DevicePollResult{}, err
		}
		return idpstore.DevicePollResult{Outcome: idpstore.DevicePollSlowDown, Grant: grant}, nil
	}
	if grant.Status == idpstore.DeviceGrantApproved {
		return idpstore.DevicePollResult{Outcome: idpstore.DevicePollApproved, Grant: grant}, nil
	}
	grant.NextPollAt = now.Add(grant.PollInterval)
	grant.Version++
	if err := s.updateDevicePoll(ctx, grant, request.ClientID, now, "status='pending'"); err != nil {
		return idpstore.DevicePollResult{}, err
	}
	return idpstore.DevicePollResult{Outcome: idpstore.DevicePollPending, Grant: grant}, nil
}

func (s *Store) updateDevicePoll(ctx context.Context, grant idpstore.DeviceGrant, clientID string, now time.Time, statusPredicate string) error {
	data, err := enc(grant)
	if err != nil {
		return err
	}
	result, err := s.conn().ExecContext(ctx, `UPDATE device_grants SET next_poll_at=?,data=? WHERE device_code_hash=? AND client_id=? AND `+statusPredicate+` AND expires_at>?`, grant.NextPollAt, data, hashKey(grant.DeviceCodeHash), clientID, now)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return idpstore.ErrDeviceGrantNotPending
	}
	return nil
}

func (s *Store) DecideDeviceGrant(ctx context.Context, request idpstore.DeviceDecisionRequest) (idpstore.DeviceGrant, error) {
	if s.runner == nil {
		var grant idpstore.DeviceGrant
		err := s.Update(ctx, func(tx idpstore.TxStore) error {
			var err error
			grant, err = tx.DecideDeviceGrant(ctx, request)
			return err
		})
		return grant, err
	}
	if request.Now.IsZero() || !request.Decision.Valid() {
		return idpstore.DeviceGrant{}, idpstore.ErrInvalidDeviceDecision
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, err := s.GetDeviceGrantByUserCodeHash(ctx, request.UserCodeHash)
	if err != nil {
		return idpstore.DeviceGrant{}, err
	}
	now := request.Now.UTC()
	if deviceGrantExpired(grant, now) {
		return idpstore.DeviceGrant{}, idpstore.ErrExpired
	}
	if grant.Status != idpstore.DeviceGrantPending {
		return idpstore.DeviceGrant{}, idpstore.ErrDeviceGrantNotPending
	}
	if request.Decision == idpstore.DeviceGrantApprove && (request.UserID == "" || request.Subject == "" || request.AuthTime.IsZero()) {
		return idpstore.DeviceGrant{}, idpstore.ErrInvalidDeviceDecision
	}
	grant.Status = idpstore.DeviceGrantDenied
	grant.DecidedAt = &now
	if request.Decision == idpstore.DeviceGrantApprove {
		grant.Status = idpstore.DeviceGrantApproved
		grant.UserID = request.UserID
		grant.Subject = request.Subject
		grant.AuthTime = request.AuthTime.UTC()
		grant.AuthenticationMethods = append([]string(nil), request.AuthenticationMethods...)
		grant.ApprovedScopes = append([]string(nil), request.ApprovedScopes...)
		grant.ApprovedAudiences = append([]string(nil), request.ApprovedAudiences...)
	}
	grant.Version++
	data, err := enc(grant)
	if err != nil {
		return idpstore.DeviceGrant{}, err
	}
	result, err := s.conn().ExecContext(ctx, `UPDATE device_grants SET status=?,data=? WHERE user_code_hash=? AND status='pending' AND expires_at>?`, grant.Status, data, hashKey(request.UserCodeHash), now)
	if err != nil {
		return idpstore.DeviceGrant{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return idpstore.DeviceGrant{}, err
	}
	if count != 1 {
		return idpstore.DeviceGrant{}, idpstore.ErrDeviceGrantNotPending
	}
	return grant, nil
}

func (s *Store) ConsumeDeviceGrant(ctx context.Context, request idpstore.DeviceConsumeRequest) (idpstore.DeviceGrant, error) {
	if s.runner == nil {
		var grant idpstore.DeviceGrant
		err := s.Update(ctx, func(tx idpstore.TxStore) error {
			var err error
			grant, err = tx.ConsumeDeviceGrant(ctx, request)
			return err
		})
		return grant, err
	}
	if request.Now.IsZero() {
		return idpstore.DeviceGrant{}, idpstore.ErrInvalidDeviceGrant
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, err := s.InspectDeviceGrantByDeviceCodeHash(ctx, request.DeviceCodeHash, request.ClientID)
	if err != nil {
		return idpstore.DeviceGrant{}, err
	}
	now := request.Now.UTC()
	if deviceGrantExpired(grant, now) {
		return idpstore.DeviceGrant{}, idpstore.ErrExpired
	}
	if grant.Status == idpstore.DeviceGrantConsumed {
		return idpstore.DeviceGrant{}, idpstore.ErrAlreadyConsumed
	}
	if grant.Status != idpstore.DeviceGrantApproved {
		return idpstore.DeviceGrant{}, idpstore.ErrDeviceGrantNotApproved
	}
	grant.Status = idpstore.DeviceGrantConsumed
	grant.ConsumedAt = &now
	grant.Version++
	data, err := enc(grant)
	if err != nil {
		return idpstore.DeviceGrant{}, err
	}
	result, err := s.conn().ExecContext(ctx, `UPDATE device_grants SET status=?,data=? WHERE device_code_hash=? AND client_id=? AND status='approved' AND expires_at>?`, grant.Status, data, hashKey(request.DeviceCodeHash), request.ClientID, now)
	if err != nil {
		return idpstore.DeviceGrant{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return idpstore.DeviceGrant{}, err
	}
	if count != 1 {
		return idpstore.DeviceGrant{}, idpstore.ErrDeviceGrantNotApproved
	}
	return grant, nil
}

func deviceGrantExpired(grant idpstore.DeviceGrant, now time.Time) bool {
	return !grant.ExpiresAt.IsZero() && !now.Before(grant.ExpiresAt)
}

func (s *Store) CreateSigningKey(ctx context.Context, k idpstore.SigningKey) error {
	b, _ := enc(k)
	active := 0
	if k.Active {
		active = 1
	}
	_, err := s.conn().ExecContext(ctx, `INSERT INTO signing_keys(id,active,data) VALUES(?,?,?)`, k.ID, active, b)
	return mapDup(err)
}
func (s *Store) ActiveSigningKey(ctx context.Context) (idpstore.SigningKey, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM signing_keys WHERE active=1 LIMIT 1`).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.SigningKey{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.SigningKey{}, err
	}
	return dec[idpstore.SigningKey](b)
}
func (s *Store) VerificationKeys(ctx context.Context) ([]idpstore.SigningKey, error) {
	rows, err := s.conn().QueryContext(ctx, `SELECT data FROM signing_keys ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []idpstore.SigningKey
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		k, err := dec[idpstore.SigningKey](b)
		if err != nil {
			return nil, err
		}
		if k.Active || !k.NotAfter.IsZero() {
			out = append(out, k)
		}
	}
	return out, rows.Err()
}
func (s *Store) ActivateSigningKey(ctx context.Context, kid string) error {
	if s.runner == nil {
		return s.Update(ctx, func(tx idpstore.TxStore) error {
			return tx.ActivateSigningKey(ctx, kid)
		})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.conn().QueryContext(ctx, `SELECT id,data FROM signing_keys`)
	if err != nil {
		return err
	}
	defer rows.Close()
	found := false
	type row struct {
		id  string
		key idpstore.SigningKey
	}
	var all []row
	for rows.Next() {
		var id string
		var b []byte
		if err := rows.Scan(&id, &b); err != nil {
			return err
		}
		k, _ := dec[idpstore.SigningKey](b)
		k.Active = id == kid
		if id == kid {
			found = true
		}
		all = append(all, row{id, k})
	}
	if !found {
		return idpstore.ErrNotFound
	}
	for _, r := range all {
		if _, err := s.conn().ExecContext(ctx, `UPDATE signing_keys SET active=0 WHERE id=?`, r.id); err != nil {
			return err
		}
	}
	for _, r := range all {
		b, _ := enc(r.key)
		active := 0
		if r.key.Active {
			active = 1
		}
		if _, err := s.conn().ExecContext(ctx, `UPDATE signing_keys SET active=?, data=? WHERE id=?`, active, b, r.id); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) RetireSigningKey(ctx context.Context, kid string) error {
	if s.runner == nil {
		return s.Update(ctx, func(tx idpstore.TxStore) error {
			return tx.RetireSigningKey(ctx, kid)
		})
	}
	k, err := s.getSigningKey(ctx, kid)
	if err != nil {
		return err
	}
	if k.Active {
		return idpstore.ErrLastSigningKey
	}
	k.Active = false
	if k.NotAfter.IsZero() {
		k.NotAfter = time.Now()
	}
	b, _ := enc(k)
	_, err = s.conn().ExecContext(ctx, `UPDATE signing_keys SET active=0,data=? WHERE id=?`, b, kid)
	return err
}
func (s *Store) DeleteRetiredSigningKey(ctx context.Context, kid string) error {
	if s.runner == nil {
		return s.Update(ctx, func(tx idpstore.TxStore) error { return tx.DeleteRetiredSigningKey(ctx, kid) })
	}
	key, err := s.getSigningKey(ctx, kid)
	if err != nil {
		return err
	}
	if key.Active {
		return idpstore.ErrActiveSigningKey
	}
	if key.NotAfter.IsZero() {
		return idpstore.ErrSigningKeyNotRetired
	}
	result, err := s.conn().ExecContext(ctx, `DELETE FROM signing_keys WHERE id=? AND active=0`, kid)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return idpstore.ErrNotFound
	}
	return nil
}
func (s *Store) getSigningKey(ctx context.Context, kid string) (idpstore.SigningKey, error) {
	var b []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM signing_keys WHERE id=?`, kid).Scan(&b)
	if err == sql.ErrNoRows {
		return idpstore.SigningKey{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.SigningKey{}, err
	}
	return dec[idpstore.SigningKey](b)
}

func (s *Store) View(ctx context.Context, fn func(idpstore.ReadStore) error) error {
	if fn == nil {
		return fmt.Errorf("view callback is required")
	}
	if s.runner != nil {
		return idpstore.ErrNestedTransaction
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("begin read transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	scoped := &Store{db: s.db, runner: tx, mu: s.mu, path: s.path, backupCopy: s.backupCopy}
	if err := fn(scoped); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit read transaction: %w", err)
	}
	return nil
}

func (s *Store) Update(ctx context.Context, fn func(idpstore.TxStore) error) error {
	if fn == nil {
		return fmt.Errorf("update callback is required")
	}
	if s.runner != nil {
		return idpstore.ErrNestedTransaction
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin write transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	scoped := &Store{db: s.db, runner: tx, mu: s.mu, path: s.path, backupCopy: s.backupCopy}
	if err := fn(scoped); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit write transaction: %w", err)
	}
	return nil
}

func (s *Store) CreateUserWithCredential(ctx context.Context, login string, user idpstore.User, credential idpstore.PasswordCredential) error {
	return s.Update(ctx, func(tx idpstore.TxStore) error {
		if err := tx.PutUser(ctx, login, user); err != nil {
			return err
		}
		return tx.PutPasswordCredential(ctx, credential)
	})
}

func (s *Store) ReplacePasswordAndSecurityState(ctx context.Context, credential idpstore.PasswordCredential, state idpstore.AccountSecurityState) error {
	return s.Update(ctx, func(tx idpstore.TxStore) error {
		if err := tx.PutPasswordCredential(ctx, credential); err != nil {
			return err
		}
		if err := tx.PutAccountSecurityState(ctx, state); err != nil {
			return err
		}
		scoped, ok := tx.(*Store)
		if !ok {
			return fmt.Errorf("unexpected SQLite transaction implementation")
		}
		return scoped.revokeUserSecurityArtifacts(ctx, credential.UserID, credential.PasswordChangedAt)
	})
}

func (s *Store) SetUserDisabled(ctx context.Context, login string, disabled bool, at time.Time) (idpstore.User, error) {
	var user idpstore.User
	err := s.Update(ctx, func(tx idpstore.TxStore) error {
		loaded, err := tx.GetUserByLogin(ctx, login)
		if err != nil {
			return err
		}
		loaded.Disabled = disabled
		loaded.UpdatedAt = at.UTC()
		if err := tx.PutUser(ctx, login, loaded); err != nil {
			return err
		}
		scoped, ok := tx.(*Store)
		if !ok {
			return fmt.Errorf("unexpected SQLite transaction implementation")
		}
		if disabled {
			if err := scoped.revokeUserSecurityArtifacts(ctx, loaded.ID, loaded.UpdatedAt); err != nil {
				return err
			}
		}
		user = loaded
		return nil
	})
	return user, err
}

func (s *Store) RevokeUserSecurityArtifacts(ctx context.Context, userID string, at time.Time) error {
	return s.Update(ctx, func(tx idpstore.TxStore) error {
		scoped, ok := tx.(*Store)
		if !ok {
			return fmt.Errorf("unexpected SQLite transaction implementation")
		}
		return scoped.revokeUserSecurityArtifacts(ctx, userID, at)
	})
}

func (s *Store) RevokeUserSecurityArtifactsTx(ctx context.Context, userID string, at time.Time) error {
	if s.runner == nil {
		return fmt.Errorf("user security artifact revocation requires a transaction")
	}
	return s.revokeUserSecurityArtifacts(ctx, userID, at)
}

// revokeUserSecurityArtifacts runs only on a transaction-scoped Store.
//
// tinyidp:transaction-scoped
func (s *Store) revokeUserSecurityArtifacts(ctx context.Context, userID string, at time.Time) error {
	user, err := s.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	type record struct {
		key  string
		data []byte
	}
	load := func(query string) ([]record, error) {
		rows, err := s.conn().QueryContext(ctx, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var records []record
		for rows.Next() {
			var item record
			if err := rows.Scan(&item.key, &item.data); err != nil {
				return nil, err
			}
			records = append(records, item)
		}
		return records, rows.Err()
	}
	grants, err := load(`SELECT id,data FROM grants`)
	if err != nil {
		return err
	}
	for _, item := range grants {
		grant, err := dec[idpstore.Grant](item.data)
		if err != nil {
			return err
		}
		if grant.UserID == userID && grant.RevokedAt == nil {
			grant.RevokedAt = &at
			data, _ := enc(grant)
			if _, err := s.conn().ExecContext(ctx, `UPDATE grants SET data=? WHERE id=?`, data, item.key); err != nil {
				return err
			}
		}
	}
	for _, table := range []struct {
		name  string
		apply func([]byte) ([]byte, bool, error)
	}{
		{"authorization_codes", func(data []byte) ([]byte, bool, error) {
			value, err := dec[idpstore.AuthorizationCode](data)
			if err != nil || value.UserID != userID || value.ConsumedAt != nil {
				return nil, false, err
			}
			value.ConsumedAt = &at
			encoded, err := enc(value)
			return encoded, true, err
		}},
		{"access_tokens", func(data []byte) ([]byte, bool, error) {
			value, err := dec[idpstore.AccessToken](data)
			if err != nil || value.UserID != userID || value.RevokedAt != nil {
				return nil, false, err
			}
			value.RevokedAt = &at
			encoded, err := enc(value)
			return encoded, true, err
		}},
		{"refresh_tokens", func(data []byte) ([]byte, bool, error) {
			value, err := dec[idpstore.RefreshToken](data)
			if err != nil || value.UserID != userID || value.RevokedAt != nil {
				return nil, false, err
			}
			value.RevokedAt = &at
			encoded, err := enc(value)
			return encoded, true, err
		}},
		{"sessions", func(data []byte) ([]byte, bool, error) {
			value, err := dec[idpstore.Session](data)
			if err != nil || value.UserID != userID || value.RevokedAt != nil {
				return nil, false, err
			}
			value.RevokedAt = &at
			encoded, err := enc(value)
			return encoded, true, err
		}},
	} {
		records, err := load(`SELECT hash,data FROM ` + table.name)
		if err != nil {
			return err
		}
		for _, item := range records {
			data, update, err := table.apply(item.data)
			if err != nil {
				return err
			}
			if update {
				if _, err := s.conn().ExecContext(ctx, `UPDATE `+table.name+` SET data=? WHERE hash=?`, data, item.key); err != nil {
					return err
				}
			}
		}
	}
	statements := []string{
		`UPDATE fosite_authorize_codes SET active=0 WHERE subject=?`,
		`DELETE FROM fosite_pkces WHERE subject=?`,
		`DELETE FROM fosite_oidc_sessions WHERE subject=?`,
		`DELETE FROM fosite_access_tokens WHERE subject=?`,
		`UPDATE fosite_refresh_tokens SET active=0 WHERE subject=?`,
	}
	for _, statement := range statements {
		if _, err := s.conn().ExecContext(ctx, statement, user.Sub); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RecordFailedLogin(ctx context.Context, userID string, now time.Time, policy idpstore.LockoutPolicy) (idpstore.AccountSecurityState, error) {
	var state idpstore.AccountSecurityState
	err := s.Update(ctx, func(tx idpstore.TxStore) error {
		loaded, loadErr := tx.GetAccountSecurityState(ctx, userID)
		if loadErr != nil && !errors.Is(loadErr, idpstore.ErrNotFound) {
			return loadErr
		}
		state = loaded
		state.UserID = userID
		if state.FirstFailedLoginAt == nil || (policy.Window > 0 && now.Sub(*state.FirstFailedLoginAt) > policy.Window) {
			state.FailedLoginCount = 0
			first := now
			state.FirstFailedLoginAt = &first
		}
		state.FailedLoginCount++
		last := now
		state.LastFailedLoginAt = &last
		if policy.Threshold > 0 && state.FailedLoginCount >= policy.Threshold {
			lockedUntil := now.Add(policy.Duration)
			state.LockedUntil = &lockedUntil
		}
		return tx.PutAccountSecurityState(ctx, state)
	})
	return state, err
}

func (s *Store) RecordSuccessfulLogin(ctx context.Context, userID string, now time.Time, session *idpstore.Session) error {
	return s.Update(ctx, func(tx idpstore.TxStore) error {
		if err := tx.ResetAccountSecurityState(ctx, userID, now); err != nil {
			return err
		}
		if session != nil {
			return tx.CreateSession(ctx, *session)
		}
		return nil
	})
}

func (s *Store) RotateSigningKey(ctx context.Context, next idpstore.SigningKey, now time.Time) (idpstore.RotationResult, error) {
	var result idpstore.RotationResult
	err := s.Update(ctx, func(tx idpstore.TxStore) error {
		old, oldErr := tx.ActiveSigningKey(ctx)
		if oldErr != nil && !errors.Is(oldErr, idpstore.ErrNotFound) {
			return oldErr
		}
		next.Active = false
		if err := tx.CreateSigningKey(ctx, next); err != nil {
			return err
		}
		if err := tx.ActivateSigningKey(ctx, next.ID); err != nil {
			return err
		}
		next.Active = true
		if oldErr == nil {
			retired := old
			retired.Active = false
			if retired.NotAfter.IsZero() {
				retired.NotAfter = now
			}
			if err := tx.RetireSigningKey(ctx, old.ID); err != nil {
				return err
			}
			result.Retired = &retired
		}
		result.Active = next
		return nil
	})
	return result, err
}

func mapDup(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(fmt.Sprint(err), "UNIQUE") {
		return idpstore.ErrDuplicate
	}
	return err
}
