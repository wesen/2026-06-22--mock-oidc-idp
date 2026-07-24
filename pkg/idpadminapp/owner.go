// Package idpadminapp orchestrates administration use cases across the
// protocol and control-plane stores.
package idpadminapp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
	pkgerrors "github.com/pkg/errors"
)

const AdminConsoleClientID = "tinyidp-admin-console"

var ErrAdminClientConflict = errors.New("existing administration console client has incompatible configuration")

type OwnerStore interface {
	idpadminstore.Store
}

type OwnerService struct {
	store OwnerStore
	now   func() time.Time
	id    func() (string, error)
}

type BootstrapOwnerRequest struct {
	OwnerLogin    string
	PublicBaseURL string
}

type OwnerStatus struct {
	Configured   bool       `json:"configured"`
	GrantID      string     `json:"grant_id,omitempty"`
	Subject      string     `json:"subject,omitempty"`
	GrantVersion int64      `json:"grant_version,omitempty"`
	ClientID     string     `json:"client_id"`
	RedirectURI  string     `json:"redirect_uri,omitempty"`
	IssuedAt     *time.Time `json:"issued_at,omitempty"`
}

func NewOwnerService(store OwnerStore, now func() time.Time) (*OwnerService, error) {
	if store == nil {
		return nil, errors.New("owner store is required")
	}
	if now == nil {
		now = time.Now
	}
	return &OwnerService{store: store, now: now, id: randomID}, nil
}

func (s *OwnerService) Bootstrap(ctx context.Context, request BootstrapOwnerRequest) (OwnerStatus, error) {
	login := strings.TrimSpace(request.OwnerLogin)
	if login == "" {
		return OwnerStatus{}, errors.New("owner login is required")
	}
	redirectURI, err := adminRedirectURI(request.PublicBaseURL)
	if err != nil {
		return OwnerStatus{}, err
	}
	now := s.now().UTC()
	grantID, err := s.id()
	if err != nil {
		return OwnerStatus{}, pkgerrors.Wrap(err, "generate owner grant id")
	}
	actionID, err := s.id()
	if err != nil {
		return OwnerStatus{}, pkgerrors.Wrap(err, "generate bootstrap action id")
	}
	nonce, err := s.id()
	if err != nil {
		return OwnerStatus{}, pkgerrors.Wrap(err, "generate bootstrap action nonce")
	}
	var result OwnerStatus
	err = s.store.AdminUpdate(ctx, func(protocol idpstore.TxStore, admin idpadminstore.TxStore) error {
		user, err := protocol.GetUserByLogin(ctx, login)
		if err != nil {
			return pkgerrors.Wrap(err, "resolve owner login")
		}
		client := adminConsoleClient(redirectURI, now)
		existing, err := protocol.GetClient(ctx, AdminConsoleClientID)
		switch {
		case errors.Is(err, idpstore.ErrNotFound):
			if err := protocol.PutClient(ctx, client); err != nil {
				return pkgerrors.Wrap(err, "create administration console client")
			}
		case err != nil:
			return pkgerrors.Wrap(err, "load administration console client")
		case !adminClientsEquivalent(existing, client):
			return ErrAdminClientConflict
		}
		grant := idpadmin.Grant{
			ID: grantID, ActorSubject: user.Sub, Scope: idpadmin.SystemScope(), Role: "owner",
			Capabilities: idpadmin.AllCapabilities(), Version: 1, IssuedAt: now,
		}
		if err := admin.CreateAdminGrant(ctx, grant); err != nil {
			return pkgerrors.Wrap(err, "create owner grant")
		}
		if err := admin.InsertAdminAction(ctx, idpadminstore.Action{
			ID: actionID, Nonce: nonce, Subject: user.Sub, GrantID: grant.ID, GrantVersion: grant.Version,
			Capability: idpadmin.CapabilityOperationsRead, Command: "console.owner.bootstrap",
			TargetType: "admin_grant", TargetID: grant.ID, ExpectedVersion: 0,
			Status: "succeeded", CreatedAt: now, CompletedAt: &now,
		}); err != nil {
			return pkgerrors.Wrap(err, "record bootstrap action")
		}
		if err := admin.EnqueueAudit(ctx, idpadminstore.AuditOutboxRecord{
			ID: actionID + "-audit", ActionID: actionID, EventType: "admin.owner.bootstrapped",
			Payload: []byte(`{"result":"accepted"}`), CreatedAt: now,
		}); err != nil {
			return pkgerrors.Wrap(err, "enqueue bootstrap audit")
		}
		result = OwnerStatus{
			Configured: true, GrantID: grant.ID, Subject: grant.ActorSubject,
			GrantVersion: grant.Version, ClientID: AdminConsoleClientID,
			RedirectURI: redirectURI, IssuedAt: &grant.IssuedAt,
		}
		return nil
	})
	if err != nil {
		return OwnerStatus{}, err
	}
	return result, nil
}

func (s *OwnerService) Status(ctx context.Context) (OwnerStatus, error) {
	now := s.now().UTC()
	grant, err := s.store.GetActiveSystemOwner(ctx, now)
	if errors.Is(err, idpadmin.ErrGrantNotFound) {
		return OwnerStatus{ClientID: AdminConsoleClientID}, nil
	}
	if err != nil {
		return OwnerStatus{}, err
	}
	issued := grant.IssuedAt
	return OwnerStatus{
		Configured: true, GrantID: grant.ID, Subject: grant.ActorSubject,
		GrantVersion: grant.Version, ClientID: AdminConsoleClientID, IssuedAt: &issued,
	}, nil
}

func (s *OwnerService) RevokeGrant(ctx context.Context, grantID string, expectedVersion int64) error {
	if strings.TrimSpace(grantID) == "" || expectedVersion < 1 {
		return errors.New("grant id and positive expected version are required")
	}
	now := s.now().UTC()
	actionID, err := s.id()
	if err != nil {
		return pkgerrors.Wrap(err, "generate revoke action id")
	}
	return s.store.AdminUpdate(ctx, func(_ idpstore.TxStore, admin idpadminstore.TxStore) error {
		grant, err := admin.GetAdminGrant(ctx, grantID)
		if err != nil {
			return err
		}
		if err := admin.RevokeAdminGrant(ctx, grantID, expectedVersion, now); err != nil {
			return err
		}
		return admin.InsertAdminAction(ctx, idpadminstore.Action{
			ID: actionID, Nonce: actionID, Subject: grant.ActorSubject, GrantID: grant.ID,
			GrantVersion: expectedVersion, Capability: idpadmin.CapabilityOperationsRead,
			Command: "console.owner.revoke_grant", TargetType: "admin_grant", TargetID: grant.ID,
			ExpectedVersion: expectedVersion, Status: "succeeded", CreatedAt: now, CompletedAt: &now,
		})
	})
}

func (s *OwnerService) RevokeSession(ctx context.Context, sessionIDHash []byte) error {
	if len(sessionIDHash) < 16 {
		return errors.New("session id hash is required")
	}
	now := s.now().UTC()
	actionID, err := s.id()
	if err != nil {
		return pkgerrors.Wrap(err, "generate session revoke action id")
	}
	targetID := base64.RawURLEncoding.EncodeToString(sessionIDHash)
	return s.store.AdminUpdate(ctx, func(_ idpstore.TxStore, admin idpadminstore.TxStore) error {
		session, err := admin.GetAdminSession(ctx, sessionIDHash)
		if err != nil {
			return err
		}
		if err := admin.RevokeAdminSession(ctx, sessionIDHash, now); err != nil {
			return err
		}
		return admin.InsertAdminAction(ctx, idpadminstore.Action{
			ID: actionID, Nonce: actionID, Subject: session.Subject, GrantID: session.GrantID,
			GrantVersion: session.GrantVersion, Capability: idpadmin.CapabilityOperationsRead,
			Command: "console.owner.revoke_session", TargetType: "admin_session", TargetID: targetID,
			Status: "succeeded", CreatedAt: now, CompletedAt: &now,
		})
	})
}

func adminRedirectURI(rawBaseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil {
		return "", pkgerrors.Wrap(err, "parse public base URL")
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", errors.New("public base URL must be an HTTPS origin without credentials, path, query, or fragment")
	}
	return strings.TrimSuffix(parsed.String(), "/") + "/admin/auth/callback", nil
}

func adminConsoleClient(redirectURI string, now time.Time) idpstore.Client {
	return idpstore.Client{
		ID: AdminConsoleClientID, Public: true, RedirectURIs: []string{redirectURI},
		AllowedScopes:     []string{"openid", "profile", "email"},
		AllowedGrantTypes: []string{idpstore.GrantAuthorizationCode},
		RequirePKCE:       true, AccessTokenTTL: 5 * time.Minute, IDTokenTTL: 5 * time.Minute,
		CreatedAt: now, UpdatedAt: now,
	}
}

func adminClientsEquivalent(left, right idpstore.Client) bool {
	return left.ID == right.ID && left.Public == right.Public && left.RequirePKCE == right.RequirePKCE &&
		stringSlicesEqual(left.RedirectURIs, right.RedirectURIs) &&
		stringSlicesEqual(left.AllowedScopes, right.AllowedScopes) &&
		stringSlicesEqual(left.AllowedGrantTypes, right.AllowedGrantTypes) &&
		len(left.SecretHash) == 0 && !left.Disabled
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func randomID() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("read cryptographic randomness: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
