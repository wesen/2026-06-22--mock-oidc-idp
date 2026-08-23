package cmds

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	cmd_sources "github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
	"github.com/stretchr/testify/require"
)

func TestAdminConsoleBootstrapStatusAndRevokeCommands(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "tinyidp.sqlite")
	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(dbPath))
	require.NoError(t, err)
	require.NoError(t, store.PutUser(ctx, "owner", idpstore.User{ID: "owner-id", Sub: "owner-sub"}))
	require.NoError(t, store.Close())
	now := time.Date(2026, time.July, 23, 22, 0, 0, 0, time.UTC)

	bootstrap, err := newAdminConsoleBootstrapCommand(&dbPath)
	require.NoError(t, err)
	bootstrap.now = func() time.Time { return now }
	bootstrapValues := values.New()
	require.NoError(t, cmd_sources.Execute(bootstrap.Schema, bootstrapValues, cmd_sources.FromMap(map[string]map[string]any{
		"default": {"owner-login": "owner", "public-base-url": "https://id.example"},
	})))
	processor := &captureProcessor{}
	require.NoError(t, bootstrap.RunIntoGlazeProcessor(ctx, bootstrapValues, processor))
	require.Len(t, processor.rows, 1)
	require.Equal(t, true, anyRowVal(processor.rows[0], "configured"))
	grantID, ok := anyRowVal(processor.rows[0], "grant_id").(string)
	require.True(t, ok)
	require.NotEmpty(t, grantID)

	status, err := newAdminConsoleStatusCommand(&dbPath)
	require.NoError(t, err)
	status.now = func() time.Time { return now }
	processor = &captureProcessor{}
	require.NoError(t, status.RunIntoGlazeProcessor(ctx, values.New(), processor))
	require.Equal(t, grantID, rowVal(processor.rows[0], "grant_id"))
	require.Equal(t, int64(1), anyRowVal(processor.rows[0], "grant_version"))

	revoke, err := newAdminConsoleRevokeGrantCommand(&dbPath)
	require.NoError(t, err)
	revoke.now = func() time.Time { return now.Add(time.Minute) }
	revokeValues := values.New()
	require.NoError(t, cmd_sources.Execute(revoke.Schema, revokeValues, cmd_sources.FromMap(map[string]map[string]any{
		"default": {"grant-id": grantID, "expected-version": 1},
	})))
	processor = &captureProcessor{}
	require.NoError(t, revoke.RunIntoGlazeProcessor(ctx, revokeValues, processor))
	require.Equal(t, "revoked", rowVal(processor.rows[0], "status"))

	status.now = revoke.now
	processor = &captureProcessor{}
	require.NoError(t, status.RunIntoGlazeProcessor(ctx, values.New(), processor))
	require.Equal(t, false, anyRowVal(processor.rows[0], "configured"))
}

func TestAdminConsoleBootstrapAtomicallyProvisionsFirstOwner(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "tinyidp.sqlite")
	passwordPath := filepath.Join(t.TempDir(), "owner-password")
	require.NoError(t, os.WriteFile(passwordPath, []byte("correct horse battery staple\n"), 0o600))
	now := time.Date(2026, time.July, 24, 14, 0, 0, 0, time.UTC)
	bootstrap, err := newAdminConsoleBootstrapCommand(&dbPath)
	require.NoError(t, err)
	bootstrap.now = func() time.Time { return now }
	bootstrapValues := values.New()
	require.NoError(t, cmd_sources.Execute(bootstrap.Schema, bootstrapValues, cmd_sources.FromMap(map[string]map[string]any{
		"default": {
			"owner-login": "owner", "owner-password-file": passwordPath,
			"owner-email": "owner@example.test", "owner-display-name": "Installation Owner",
			"public-base-url": "https://id.example",
		},
	})))
	processor := &captureProcessor{}
	require.NoError(t, bootstrap.RunIntoGlazeProcessor(ctx, bootstrapValues, processor))

	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(dbPath))
	require.NoError(t, err)
	defer store.Close()
	owner, err := store.GetUserByLogin(ctx, "owner")
	require.NoError(t, err)
	require.Equal(t, "owner@example.test", owner.Email)
	credential, err := store.GetPasswordCredentialByUserID(ctx, owner.ID)
	require.NoError(t, err)
	require.NotEmpty(t, credential.PasswordHash)
	grant, err := store.GetActiveSystemOwner(ctx, now)
	require.NoError(t, err)
	require.Equal(t, owner.Sub, grant.ActorSubject)
}

func anyRowVal(row types.Row, key string) any {
	value, _ := row.Get(key)
	return value
}

func TestAdminCommandBuildsGlazedConsoleChildren(t *testing.T) {
	admin, err := NewAdminCommand()
	require.NoError(t, err)
	console, _, err := admin.Find([]string{"console"})
	require.NoError(t, err)
	require.NotNil(t, console)
	require.Len(t, console.Commands(), 4)
}
