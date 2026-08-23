package idpaccounts

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	"github.com/go-go-golems/tiny-idp/internal/user"
	"github.com/go-go-golems/tiny-idp/pkg/idp"
	idpstore "github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

// CreateRequest describes a local account and its initial password.
type CreateRequest struct {
	Login             string
	Password          []byte
	ID                string
	Subject           string
	Email             string
	EmailVerified     bool
	Name              string
	PreferredUsername string
	Groups            []string
	Roles             []string
	Tenant            string
	Locale            string
	// RequireUniqueDisplayName asks the native account transaction to reserve
	// the normalized display-name key. It is a policy choice, not a global
	// property of User.Name.
	RequireUniqueDisplayName bool
}

// PreparedCreate is a validated account and password credential ready to be
// written by a caller-owned idpstore transaction. It deliberately contains no
// plaintext password. Callers must commit it immediately and must not retain
// it beyond the request that prepared it.
type PreparedCreate struct {
	Login                    string
	User                     idpstore.User
	Credential               idpstore.PasswordCredential
	RequireUniqueDisplayName bool
}

// NormalizeLogin returns the canonical login representation used by account
// creation and password authentication. Embedding applications can use this
// value for non-secret correlation keys such as rate limits; they must not use
// it as proof that an account exists.
func NormalizeLogin(login string) string {
	return user.Normalize(login)
}

// NormalizeDisplayName returns the comparison key used by optional display
// name claims. It preserves the original User.Name for presentation while
// making surrounding whitespace, repeated whitespace, Unicode composition,
// and case differences compare equal.
func NormalizeDisplayName(name string) string {
	return cases.Fold().String(norm.NFC.String(strings.Join(strings.Fields(name), " ")))
}

// Create atomically persists a user and its initial password credential.
func (s *Service) Create(ctx context.Context, req CreateRequest) (idpstore.User, error) {
	login := NormalizeLogin(req.Login)
	if login == "" {
		return idpstore.User{}, fmt.Errorf("login is required")
	}
	if len(req.Password) == 0 {
		return idpstore.User{}, fmt.Errorf("password is required")
	}
	if _, err := s.store.GetUserByLogin(ctx, login); err == nil {
		return idpstore.User{}, idpstore.ErrDuplicate
	} else if !errors.Is(err, idpstore.ErrNotFound) {
		return idpstore.User{}, err
	}
	if id := strings.TrimSpace(req.ID); id != "" {
		if _, err := s.store.GetUser(ctx, id); err == nil {
			return idpstore.User{}, idpstore.ErrDuplicate
		} else if !errors.Is(err, idpstore.ErrNotFound) {
			return idpstore.User{}, err
		}
	}
	if subject := strings.TrimSpace(req.Subject); subject != "" {
		if _, err := s.store.GetUserBySubject(ctx, subject); err == nil {
			return idpstore.User{}, idpstore.ErrDuplicate
		} else if !errors.Is(err, idpstore.ErrNotFound) {
			return idpstore.User{}, err
		}
	}
	prepared, err := s.prepareCreate(ctx, req, login)
	if err != nil {
		return idpstore.User{}, err
	}
	if err := s.store.Update(ctx, func(tx idpstore.TxStore) error {
		return s.CommitPrepared(ctx, tx, prepared)
	}); err != nil {
		return idpstore.User{}, err
	}
	err = s.auditCommitted(ctx, idp.Event{Time: prepared.User.CreatedAt, Name: "identity.account.created", Subject: prepared.User.Sub, Result: "accepted"})
	return prepared.User, err
}

// PrepareCreate validates and hashes an account creation request without
// writing it. It exists for a higher-level native transaction such as signup,
// which must commit an identity together with its continuation and interaction
// state. It performs no account-existence check because that check must happen
// at the final transaction boundary.
func (s *Service) PrepareCreate(ctx context.Context, req CreateRequest) (PreparedCreate, error) {
	login := NormalizeLogin(req.Login)
	if login == "" {
		return PreparedCreate{}, fmt.Errorf("login is required")
	}
	if len(req.Password) == 0 {
		return PreparedCreate{}, fmt.Errorf("password is required")
	}
	return s.prepareCreate(ctx, req, login)
}

// CommitPrepared writes a prepared identity and credential into the caller's
// transaction. It intentionally does not emit audit output: callers must do
// that only after their complete transaction commits.
func (s *Service) CommitPrepared(ctx context.Context, tx idpstore.TxStore, prepared PreparedCreate) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	if prepared.Login == "" || prepared.User.ID == "" || prepared.Credential.UserID != prepared.User.ID || prepared.Credential.Login != prepared.Login {
		return fmt.Errorf("prepared account is invalid")
	}
	if prepared.RequireUniqueDisplayName {
		claims, ok := tx.(idpstore.DisplayNameStore)
		if !ok {
			return fmt.Errorf("transaction store does not support display-name claims")
		}
		key := NormalizeDisplayName(prepared.User.Name)
		if key == "" {
			return fmt.Errorf("display name is required when uniqueness is requested")
		}
		if err := claims.ReserveDisplayName(ctx, key, prepared.User.ID); err != nil {
			return err
		}
	}
	if err := tx.PutUser(ctx, prepared.Login, prepared.User); err != nil {
		return err
	}
	return tx.PutPasswordCredential(ctx, prepared.Credential)
}

func (s *Service) prepareCreate(ctx context.Context, req CreateRequest, login string) (PreparedCreate, error) {
	now := s.clock().UTC()
	id := strings.TrimSpace(req.ID)
	if id == "" {
		generatedID, err := newID("user")
		if err != nil {
			return PreparedCreate{}, fmt.Errorf("generate user id: %w", err)
		}
		id = generatedID
	}
	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		subject = id
	}
	u := idpstore.User{
		ID: id, Sub: subject, Email: strings.TrimSpace(req.Email), EmailVerified: req.EmailVerified,
		Name: strings.TrimSpace(req.Name), PreferredUsername: firstNonEmpty(strings.TrimSpace(req.PreferredUsername), login),
		Groups: cleanList(req.Groups), Roles: cleanList(req.Roles), Tenant: strings.TrimSpace(req.Tenant),
		Locale: strings.TrimSpace(req.Locale), CreatedAt: now, UpdatedAt: now,
	}
	if err := u.Validate(); err != nil {
		return PreparedCreate{}, err
	}
	credential, err := s.hashCredential(ctx, u.ID, login, req.Password, now)
	if err != nil {
		return PreparedCreate{}, err
	}
	return PreparedCreate{Login: login, User: u, Credential: credential, RequireUniqueDisplayName: req.RequireUniqueDisplayName}, nil
}

// SetPasswordRequest identifies an account and its replacement password.
type SetPasswordRequest struct {
	Login    string
	Password []byte
}

// PreparePassword validates and hashes a replacement password for a caller
// that owns the surrounding protocol/admin transaction.
func (s *Service) PreparePassword(
	ctx context.Context,
	userID, login string,
	password []byte,
) (idpstore.PasswordCredential, error) {
	login = user.Normalize(login)
	if strings.TrimSpace(userID) == "" || login == "" {
		return idpstore.PasswordCredential{}, fmt.Errorf("user id and login are required")
	}
	if len(password) == 0 {
		return idpstore.PasswordCredential{}, fmt.Errorf("password is required")
	}
	return s.hashCredential(ctx, userID, login, password, s.clock().UTC())
}

// SetPassword atomically replaces a credential and clears account lockout state.
func (s *Service) SetPassword(ctx context.Context, req SetPasswordRequest) error {
	login := user.Normalize(req.Login)
	if login == "" {
		return fmt.Errorf("login is required")
	}
	if len(req.Password) == 0 {
		return fmt.Errorf("password is required")
	}
	u, err := s.store.GetUserByLogin(ctx, login)
	if err != nil {
		return err
	}
	now := s.clock().UTC()
	credential, err := s.PreparePassword(ctx, u.ID, login, req.Password)
	if err != nil {
		return err
	}
	state := idpstore.AccountSecurityState{UserID: u.ID}
	if err := s.store.ReplacePasswordAndSecurityState(ctx, credential, state); err != nil {
		return err
	}
	return s.auditCommitted(ctx, idp.Event{Time: now, Name: "identity.account.password_changed", Subject: u.Sub, Result: "accepted"})
}

func (s *Service) auditCommitted(ctx context.Context, event idp.Event) error {
	if err := s.audit.Emit(ctx, event); err != nil {
		return fmt.Errorf("%w: %v", idp.ErrAuditDelivery, err)
	}
	return nil
}

func newID(prefix string) (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + "-" + base64.RawURLEncoding.EncodeToString(b), nil
}

func cleanList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
