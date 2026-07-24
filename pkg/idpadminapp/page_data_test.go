package idpadminapp_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
	"github.com/stretchr/testify/require"
)

func TestPageDataServiceAuthorizesAndProjectsSafeUserRows(t *testing.T) {
	ctx := context.Background()
	store := openAdminStore(t)
	now := time.Date(2026, 7, 24, 3, 0, 0, 0, time.UTC)
	grant := executorGrant(now)
	require.NoError(t, store.CreateAdminGrant(ctx, grant))
	require.NoError(t, store.PutUser(ctx, "alice", idpstore.User{
		ID: "user-1", Sub: "subject-1", Name: "Alice", Email: "alice@example.com",
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
	}))
	_, err := store.RebuildAdminUserProjection(ctx, now)
	require.NoError(t, err)
	authorizer, err := idpadmin.NewAuthorizer(store, 5*time.Minute, func() time.Time { return now })
	require.NoError(t, err)
	service, err := idpadminapp.NewPageDataService(store, authorizer, func() time.Time { return now })
	require.NoError(t, err)
	principal := executorPrincipal(now)
	principal.GrantID = grant.ID
	principal.GrantVersion = grant.Version

	data, err := service.PageData(ctx, principal, "users", url.Values{"q": {"alice"}})
	require.NoError(t, err)
	require.Equal(t, "users", data["id"])
	rows, ok := data["rows"].([]map[string]string)
	require.True(t, ok)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["primary"])
	require.NotContains(t, rows[0], "password_hash")

	grant.Capabilities = []idpadmin.Capability{idpadmin.CapabilityOverviewRead}
	grant.ID = "limited-grant"
	require.NoError(t, store.RevokeAdminGrant(ctx, principal.GrantID, principal.GrantVersion, now))
	require.NoError(t, store.CreateAdminGrant(ctx, grant))
	principal.GrantID = grant.ID
	_, err = service.PageData(ctx, principal, "users", nil)
	require.ErrorIs(t, err, idpadmin.ErrCapabilityDenied)
}
