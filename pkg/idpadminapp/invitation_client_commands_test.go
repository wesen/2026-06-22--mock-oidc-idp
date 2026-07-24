package idpadminapp_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
	"github.com/go-go-golems/tiny-idp/pkg/idpinvite"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
	"github.com/stretchr/testify/require"
)

type resourceCommandFixture struct {
	ctx         context.Context
	now         time.Time
	store       *sqlitestore.Store
	principal   idpadmin.AdminPrincipal
	actions     *idpadminapp.ActionService
	invitations *idpadminapp.InvitationCommandService
	clients     *idpadminapp.ClientCommandService
	sequence    int
}

func TestInvitationIssueIsOneTimeAndRevokesByPublicID(t *testing.T) {
	fixture := newResourceCommandFixture(t)
	fixture.seedAudience(t)

	prepared := fixture.prepare(t, idpadminapp.CommandInvitationsIssue, "")
	input := map[string]any{
		"label": "Engineering onboarding", "audience": "browser-app", "valid_for": "24h",
	}
	response, request := fixture.execute(t, fixture.invitations, prepared, input)
	var issued idpadmin.OneTimeSecretResult
	require.NoError(t, json.Unmarshal(response, &issued))
	require.Equal(t, prepared.TargetID, issued.ResourceID)
	require.NotEmpty(t, issued.Secret)

	stored, err := fixture.store.GetDurableInvitationByID(fixture.ctx, issued.ResourceID)
	require.NoError(t, err)
	require.Equal(t, "browser-app", stored.Audience)
	require.NotEqual(t, []byte(issued.Secret), stored.CodeHash)
	var label, creator string
	require.NoError(t, fixture.store.SQLDB().QueryRowContext(fixture.ctx, `
		SELECT label, created_by_subject FROM admin_invitation_records WHERE invitation_id=?`,
		issued.ResourceID).Scan(&label, &creator))
	require.Equal(t, "Engineering onboarding", label)
	require.Equal(t, fixture.principal.Subject, creator)
	assertSecretAbsentFromControlPlane(t, fixture.store, issued.Secret)

	replayInput, err := json.Marshal(input)
	require.NoError(t, err)
	_, err = fixture.invitations.Execute(fixture.ctx, request, replayInput)
	require.ErrorIs(t, err, idpadminapp.ErrSecretAlreadyIssued)

	revokedResponse, _ := fixture.execute(t, fixture.invitations,
		fixture.prepare(t, idpadminapp.CommandInvitationsRevoke, issued.ResourceID),
		map[string]any{"reason": "Recipient changed teams", "confirmation": "REVOKE"})
	var revoked idpadmin.InvitationResult
	require.NoError(t, json.Unmarshal(revokedResponse, &revoked))
	require.Equal(t, "revoked", revoked.Invitation.Status)
	require.Equal(t, int64(2), revoked.Invitation.Version)
	durable, err := idpinvite.NewDurableService(fixture.store, []byte("invitation-lookup-key-32-bytes!!"))
	require.NoError(t, err)
	_, err = durable.Inspect(fixture.ctx, issued.Secret, "browser-app", fixture.now)
	require.ErrorIs(t, err, idpstore.ErrInvitationRevoked)
}

func TestClientLifecycleRotatesSecretOnceAndInvalidatesOldSecret(t *testing.T) {
	fixture := newResourceCommandFixture(t)
	createInput := map[string]any{
		"public":                    false,
		"redirect_uris":             []string{"https://app.example.test/callback"},
		"post_logout_redirect_uris": []string{"https://app.example.test/logout"},
		"allowed_scopes":            []string{"openid", "profile"},
		"allowed_grant_types":       []string{idpstore.GrantAuthorizationCode, idpstore.GrantRefreshToken},
		"allowed_audiences":         []string{"https://api.example.test"},
		"require_pkce":              true,
	}
	prepared := fixture.prepare(t, idpadminapp.CommandClientsCreate, "engineering-app")
	createResponse, createRequest := fixture.execute(t, fixture.clients, prepared, createInput)
	var created idpadmin.OneTimeSecretResult
	require.NoError(t, json.Unmarshal(createResponse, &created))
	require.Equal(t, "engineering-app", created.ResourceID)
	require.NotEmpty(t, created.Secret)
	client, err := fixture.store.GetClient(fixture.ctx, created.ResourceID)
	require.NoError(t, err)
	require.NoError(t, bcrypt.CompareHashAndPassword(client.SecretHash, []byte(created.Secret)))
	assertSecretAbsentFromControlPlane(t, fixture.store, created.Secret)

	replayInput, err := json.Marshal(createInput)
	require.NoError(t, err)
	_, err = fixture.clients.Execute(fixture.ctx, createRequest, replayInput)
	require.ErrorIs(t, err, idpadminapp.ErrSecretAlreadyIssued)

	fixture.execute(t, fixture.clients,
		fixture.prepare(t, idpadminapp.CommandClientsUpdate, created.ResourceID),
		map[string]any{
			"public":              false,
			"redirect_uris":       []string{"https://app.example.test/callback", "https://app.example.test/callback/two"},
			"allowed_scopes":      []string{"openid"},
			"allowed_grant_types": []string{idpstore.GrantAuthorizationCode},
			"allowed_audiences":   []string{"https://api.example.test"},
			"require_pkce":        true,
		})
	updated, err := fixture.store.GetClient(fixture.ctx, created.ResourceID)
	require.NoError(t, err)
	require.Len(t, updated.RedirectURIs, 2)
	require.NoError(t, bcrypt.CompareHashAndPassword(updated.SecretHash, []byte(created.Secret)))

	fixture.execute(t, fixture.clients,
		fixture.prepare(t, idpadminapp.CommandClientsDisable, created.ResourceID),
		map[string]any{"reason": "Application maintenance", "confirmation": "DISABLE"})
	disabled, err := fixture.store.GetClient(fixture.ctx, created.ResourceID)
	require.NoError(t, err)
	require.True(t, disabled.Disabled)

	rotatePrepared := fixture.prepare(t, idpadminapp.CommandClientsRotateSecret, created.ResourceID)
	rotateResponse, rotateRequest := fixture.execute(t, fixture.clients, rotatePrepared,
		map[string]any{"reason": "Scheduled rotation", "confirmation": "ROTATE"})
	var rotated idpadmin.OneTimeSecretResult
	require.NoError(t, json.Unmarshal(rotateResponse, &rotated))
	require.NotEqual(t, created.Secret, rotated.Secret)
	current, err := fixture.store.GetClient(fixture.ctx, created.ResourceID)
	require.NoError(t, err)
	require.Error(t, bcrypt.CompareHashAndPassword(current.SecretHash, []byte(created.Secret)))
	require.NoError(t, bcrypt.CompareHashAndPassword(current.SecretHash, []byte(rotated.Secret)))
	assertSecretAbsentFromControlPlane(t, fixture.store, rotated.Secret)

	rotateInput := []byte(`{"reason":"Scheduled rotation","confirmation":"ROTATE"}`)
	_, err = fixture.clients.Execute(fixture.ctx, rotateRequest, rotateInput)
	require.ErrorIs(t, err, idpadminapp.ErrSecretAlreadyIssued)
}

func TestClientValidationRejectsUnsafeRedirectBeforeMutation(t *testing.T) {
	fixture := newResourceCommandFixture(t)
	prepared := fixture.prepare(t, idpadminapp.CommandClientsCreate, "unsafe-app")
	_, err := fixture.executeRaw(fixture.clients, prepared, map[string]any{
		"public":              true,
		"redirect_uris":       []string{"https://*.example.test/callback"},
		"allowed_scopes":      []string{"openid"},
		"allowed_grant_types": []string{idpstore.GrantAuthorizationCode},
		"require_pkce":        true,
	})
	require.ErrorIs(t, err, idpstore.ErrWildcardRedirectURI)
	var validation *idpadminapp.ValidationError
	require.ErrorAs(t, err, &validation)
	require.Equal(t, idpstore.ErrWildcardRedirectURI.Error(), validation.FieldErrors["redirect_uris"])
	_, err = fixture.store.GetClient(fixture.ctx, "unsafe-app")
	require.ErrorIs(t, err, idpstore.ErrNotFound)
}

func newResourceCommandFixture(t *testing.T) *resourceCommandFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(filepath.Join(t.TempDir(), "idp.db")))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	grant := idpadmin.Grant{
		ID: "owner-grant", ActorSubject: "owner-sub", Scope: idpadmin.SystemScope(),
		Role: "owner", Capabilities: idpadmin.AllCapabilities(), Version: 1,
		IssuedAt: now.Add(-time.Hour),
	}
	require.NoError(t, store.CreateAdminGrant(ctx, grant))
	clock := func() time.Time { return now }
	handles, err := idpadmin.NewHandleService([]byte("0123456789abcdef0123456789abcdef"), time.Minute, clock)
	require.NoError(t, err)
	authorizer, err := idpadmin.NewAuthorizer(store, 5*time.Minute, clock)
	require.NoError(t, err)
	executor, err := idpadminapp.NewExecutor(store, handles, authorizer, clock)
	require.NoError(t, err)
	actions, err := idpadminapp.NewActionService(store, handles, authorizer, clock)
	require.NoError(t, err)
	durable, err := idpinvite.NewDurableService(store, []byte("invitation-lookup-key-32-bytes!!"))
	require.NoError(t, err)
	invitations, err := idpadminapp.NewInvitationCommandService(store, executor, durable, clock)
	require.NoError(t, err)
	clients, err := idpadminapp.NewClientCommandService(store, executor, clock)
	require.NoError(t, err)
	return &resourceCommandFixture{
		ctx: ctx, now: now, store: store, actions: actions,
		principal: idpadmin.AdminPrincipal{
			Subject: "owner-sub", SessionID: "session-binding", Authenticated: now,
			Assurance: idpadmin.AssuranceFresh, GrantID: grant.ID, GrantVersion: grant.Version,
		},
		invitations: invitations, clients: clients,
	}
}

func (f *resourceCommandFixture) seedAudience(t *testing.T) {
	t.Helper()
	now := f.now
	client := idpstore.Client{
		ID: "browser-app", Public: true, RequirePKCE: true,
		RedirectURIs:  []string{"https://browser.example.test/callback"},
		AllowedScopes: []string{"openid"}, AllowedGrantTypes: []string{idpstore.GrantAuthorizationCode},
		AccessTokenTTL: time.Hour, IDTokenTTL: time.Hour, RefreshTokenTTL: 24 * time.Hour,
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, client.Validate(idpstore.ProductionMode))
	require.NoError(t, f.store.PutClient(f.ctx, client))
	require.NoError(t, f.store.CreateResourceVersion(f.ctx, "client", client.ID, 1, now))
}

func (f *resourceCommandFixture) prepare(t *testing.T, command, targetID string) idpadminapp.PreparedAction {
	t.Helper()
	prepared, err := f.actions.Prepare(f.ctx, f.principal, idpadminapp.PrepareActionRequest{
		Command: command, TargetID: targetID,
	})
	require.NoError(t, err)
	return prepared
}

func (f *resourceCommandFixture) execute(
	t *testing.T,
	service idpadminapp.CommandExecutor,
	prepared idpadminapp.PreparedAction,
	input map[string]any,
) ([]byte, idpadminapp.ExecutionRequest) {
	t.Helper()
	response, request, err := f.executeRawWithRequest(service, prepared, input)
	require.NoError(t, err)
	return response, request
}

func (f *resourceCommandFixture) executeRaw(
	service idpadminapp.CommandExecutor,
	prepared idpadminapp.PreparedAction,
	input map[string]any,
) ([]byte, error) {
	response, _, err := f.executeRawWithRequest(service, prepared, input)
	return response, err
}

func (f *resourceCommandFixture) executeRawWithRequest(
	service idpadminapp.CommandExecutor,
	prepared idpadminapp.PreparedAction,
	input map[string]any,
) ([]byte, idpadminapp.ExecutionRequest, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, idpadminapp.ExecutionRequest{}, err
	}
	sum := sha256.Sum256(raw)
	f.sequence++
	request := idpadminapp.ExecutionRequest{
		Handle: prepared.Handle, Principal: f.principal,
		RequestID:      fmt.Sprintf("request-%d", f.sequence),
		IdempotencyKey: fmt.Sprintf("idempotency-%d", f.sequence),
		RequestHash:    sum[:],
	}
	response, err := service.Execute(f.ctx, request, append([]byte(nil), raw...))
	return response, request, err
}

func assertSecretAbsentFromControlPlane(t *testing.T, store *sqlitestore.Store, secret string) {
	t.Helper()
	for _, table := range []string{"admin_actions", "admin_audit_outbox", "admin_idempotency"} {
		var count int
		switch table {
		case "admin_actions":
			require.NoError(t, store.SQLDB().QueryRow(
				`SELECT COUNT(*) FROM admin_actions WHERE command LIKE ?`,
				"%"+secret+"%").Scan(&count))
		case "admin_audit_outbox":
			require.NoError(t, store.SQLDB().QueryRow(
				`SELECT COUNT(*) FROM admin_audit_outbox WHERE CAST(payload_json AS TEXT) LIKE ?`,
				"%"+secret+"%").Scan(&count))
		case "admin_idempotency":
			require.NoError(t, store.SQLDB().QueryRow(
				`SELECT COUNT(*) FROM admin_idempotency WHERE CAST(response_json AS TEXT) LIKE ?`,
				"%"+secret+"%").Scan(&count))
		}
		require.Zero(t, count, table)
	}
}
