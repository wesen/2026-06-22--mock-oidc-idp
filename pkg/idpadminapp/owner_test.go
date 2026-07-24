package idpadminapp_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
	"github.com/stretchr/testify/require"
)

func TestOwnerBootstrapStatusAndRevoke(t *testing.T) {
	ctx := context.Background()
	store := openAdminStore(t)
	now := time.Date(2026, 7, 23, 21, 0, 0, 0, time.UTC)
	require.NoError(t, store.PutUser(ctx, "owner", idpstore.User{ID: "owner-id", Sub: "owner-sub"}))
	service, err := idpadminapp.NewOwnerService(store, func() time.Time { return now })
	require.NoError(t, err)

	status, err := service.Bootstrap(ctx, idpadminapp.BootstrapOwnerRequest{
		OwnerLogin: "owner", PublicBaseURL: "https://id.example",
	})
	require.NoError(t, err)
	require.True(t, status.Configured)
	require.Equal(t, "owner-sub", status.Subject)
	require.Equal(t, "https://id.example/admin/auth/callback", status.RedirectURI)

	client, err := store.GetClient(ctx, idpadminapp.AdminConsoleClientID)
	require.NoError(t, err)
	require.True(t, client.Public)
	require.True(t, client.RequirePKCE)
	require.Empty(t, client.SecretHash)
	require.Equal(t, []string{idpstore.GrantAuthorizationCode}, client.AllowedGrantTypes)

	current, err := service.Status(ctx)
	require.NoError(t, err)
	require.Equal(t, status.GrantID, current.GrantID)

	sessionHash := []byte("0123456789abcdef0123456789abcdef")
	require.NoError(t, store.CreateAdminSession(ctx, idpadminstore.Session{
		IDHash: sessionHash, Subject: status.Subject, GrantID: status.GrantID,
		GrantVersion: status.GrantVersion, CSRFHash: []byte("csrf"),
		AuthenticatedAt: now, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour),
	}))
	require.NoError(t, service.RevokeSession(ctx, sessionHash))
	session, err := store.GetAdminSession(ctx, sessionHash)
	require.NoError(t, err)
	require.NotNil(t, session.RevokedAt)

	_, err = service.Bootstrap(ctx, idpadminapp.BootstrapOwnerRequest{
		OwnerLogin: "owner", PublicBaseURL: "https://id.example",
	})
	require.ErrorIs(t, err, idpadminstore.ErrDuplicate)

	require.NoError(t, service.RevokeGrant(ctx, status.GrantID, status.GrantVersion))
	current, err = service.Status(ctx)
	require.NoError(t, err)
	require.False(t, current.Configured)

	grant, err := store.GetAdminGrant(ctx, status.GrantID)
	require.NoError(t, err)
	require.NotNil(t, grant.RevokedAt)
	require.Equal(t, int64(2), grant.Version)
}

func TestOwnerBootstrapRejectsUnsafeOriginAndClientDrift(t *testing.T) {
	ctx := context.Background()
	store := openAdminStore(t)
	require.NoError(t, store.PutUser(ctx, "owner", idpstore.User{ID: "owner-id", Sub: "owner-sub"}))
	service, err := idpadminapp.NewOwnerService(store, time.Now)
	require.NoError(t, err)

	_, err = service.Bootstrap(ctx, idpadminapp.BootstrapOwnerRequest{
		OwnerLogin: "owner", PublicBaseURL: "http://id.example",
	})
	require.Error(t, err)

	require.NoError(t, store.PutClient(ctx, idpstore.Client{
		ID: idpadminapp.AdminConsoleClientID, Public: false, SecretHash: []byte("secret"),
	}))
	_, err = service.Bootstrap(ctx, idpadminapp.BootstrapOwnerRequest{
		OwnerLogin: "owner", PublicBaseURL: "https://id.example",
	})
	require.ErrorIs(t, err, idpadminapp.ErrAdminClientConflict)
	require.Empty(t, activeOwnerID(t, store))
}

func activeOwnerID(t *testing.T, store *sqlitestore.Store) string {
	t.Helper()
	grant, err := store.GetActiveSystemOwner(context.Background(), time.Now().Add(time.Hour))
	if errors.Is(err, idpadmin.ErrGrantNotFound) {
		return ""
	}
	require.NoError(t, err)
	return grant.ID
}

func openAdminStore(t *testing.T) *sqlitestore.Store {
	t.Helper()
	store, err := sqlitestore.Open(context.Background(), sqlitestore.DefaultConfig(filepath.Join(t.TempDir(), "idp.db")))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}
