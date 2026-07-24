package idpadmin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type grantReaderFunc func(context.Context, string) (Grant, error)

func (f grantReaderFunc) GetAdminGrant(ctx context.Context, id string) (Grant, error) {
	return f(ctx, id)
}

func TestScopeAndCapabilityAreClosed(t *testing.T) {
	require.NoError(t, SystemScope().ValidateMVP())
	require.ErrorIs(t, (AdminScope{Kind: ScopeDomain, ID: "example"}).ValidateMVP(), ErrUnsupportedScope)
	require.ErrorIs(t, Capability("root.everything").Validate(), ErrUnknownCapability)
}

func TestAuthorizerRevalidatesGrantVersionAndFreshness(t *testing.T) {
	now := time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC)
	grant := Grant{
		ID: "grant-1", ActorSubject: "subject-1", Scope: SystemScope(), Role: "owner",
		Capabilities: []Capability{CapabilityUsersWrite}, Version: 2, IssuedAt: now.Add(-time.Hour),
	}
	authorizer, err := NewAuthorizer(grantReaderFunc(func(context.Context, string) (Grant, error) {
		return grant, nil
	}), 5*time.Minute, func() time.Time { return now })
	require.NoError(t, err)
	principal := AdminPrincipal{
		Subject: "subject-1", SessionID: "session-1",
		Authenticated: now.Add(-time.Minute), Assurance: AssuranceFresh,
	}

	got, err := authorizer.Authorize(context.Background(), principal, "grant-1", 2, SystemScope(), CapabilityUsersWrite, true)
	require.NoError(t, err)
	require.Equal(t, grant, got)

	_, err = authorizer.Authorize(context.Background(), principal, "grant-1", 1, SystemScope(), CapabilityUsersWrite, false)
	require.ErrorIs(t, err, ErrGrantChanged)

	principal.Authenticated = now.Add(-6 * time.Minute)
	_, err = authorizer.Authorize(context.Background(), principal, "grant-1", 2, SystemScope(), CapabilityUsersWrite, true)
	require.ErrorIs(t, err, ErrFreshAuthRequired)

	grant.RevokedAt = &now
	_, err = authorizer.Authorize(context.Background(), principal, "grant-1", 2, SystemScope(), CapabilityUsersWrite, false)
	require.ErrorIs(t, err, ErrGrantInactive)
}

func TestActionHandleTamperExpiryAndBinding(t *testing.T) {
	now := time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC)
	service, err := NewHandleService([]byte("0123456789abcdef0123456789abcdef"), 2*time.Minute, func() time.Time { return now })
	require.NoError(t, err)
	principal := AdminPrincipal{
		Subject: "subject-1", SessionID: "session-1", Authenticated: now, Assurance: AssuranceFresh,
	}
	raw, err := service.Mint(ActionClaims{
		SessionID: principal.SessionID, Subject: principal.Subject, GrantID: "grant-1",
		GrantVersion: 3, Scope: SystemScope(), Capability: CapabilityUsersWrite,
		Command: "users.disable", TargetType: "user", TargetID: "user-1", ExpectedVersion: 7,
	})
	require.NoError(t, err)

	claims, err := service.Verify(raw, principal)
	require.NoError(t, err)
	require.Equal(t, int64(7), claims.ExpectedVersion)

	parts := strings.Split(raw, ".")
	replacement := byte('A')
	if parts[1][0] == replacement {
		replacement = 'B'
	}
	parts[1] = string(replacement) + parts[1][1:]
	tampered := strings.Join(parts, ".")
	_, err = service.Verify(tampered, principal)
	require.ErrorIs(t, err, ErrInvalidAction)

	otherSession := principal
	otherSession.SessionID = "session-2"
	_, err = service.Verify(raw, otherSession)
	require.ErrorIs(t, err, ErrActionSession)

	now = now.Add(3 * time.Minute)
	_, err = service.Verify(raw, principal)
	require.ErrorIs(t, err, ErrExpiredAction)
}

func TestAuthorizerPropagatesStoreFailure(t *testing.T) {
	expected := errors.New("database unavailable")
	authorizer, err := NewAuthorizer(grantReaderFunc(func(context.Context, string) (Grant, error) {
		return Grant{}, expected
	}), time.Minute, time.Now)
	require.NoError(t, err)
	principal := AdminPrincipal{Subject: "subject", SessionID: "session", Authenticated: time.Now(), Assurance: AssuranceAuthenticated}
	_, err = authorizer.Authorize(context.Background(), principal, "grant", 1, SystemScope(), CapabilityUsersRead, false)
	require.ErrorIs(t, err, expected)
}
