package cmds

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
	"github.com/stretchr/testify/require"
)

func TestAdminClientMutationsUseGuardedCommandLayer(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	dbPath := filepath.Join(directory, "tinyidp.sqlite")
	ownerPassword := filepath.Join(directory, "owner-password")
	actionKey := filepath.Join(directory, "admin-action-key")
	require.NoError(t, os.WriteFile(ownerPassword, []byte("owner correct horse battery staple\n"), 0o600))
	require.NoError(t, os.WriteFile(actionKey, []byte("0123456789abcdef0123456789abcdef"), 0o600))
	runAdminCLI(t, ctx, "--db", dbPath, "console", "bootstrap",
		"--owner-login", "owner", "--owner-password-file", ownerPassword,
		"--public-base-url", "https://id.example")

	output := runAdminCLI(t, ctx,
		"--db", dbPath, "client", "--admin-action-key-file", actionKey,
		"create", "--id", "engineering-app",
		"--redirect-uri", "https://app.example.test/callback",
		"--scope", "openid", "--grant-type", "authorization_code",
	)
	require.Contains(t, output, `"secret"`)
	require.NotContains(t, output, `"SecretHash"`)
	runAdminCLI(t, ctx,
		"--db", dbPath, "client", "--admin-action-key-file", actionKey,
		"update", "--id", "engineering-app",
		"--redirect-uri", "https://app.example.test/callback/v2",
		"--scope", "openid", "--grant-type", "authorization_code",
	)
	runAdminCLI(t, ctx,
		"--db", dbPath, "client", "--admin-action-key-file", actionKey,
		"disable", "--id", "engineering-app", "--reason", "Maintenance", "--confirm", "DISABLE",
	)
	output = runAdminCLI(t, ctx,
		"--db", dbPath, "client", "--admin-action-key-file", actionKey,
		"rotate-secret", "--id", "engineering-app", "--reason", "Scheduled", "--confirm", "ROTATE",
	)
	require.Contains(t, output, `"secret-rotated"`)

	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(dbPath))
	require.NoError(t, err)
	defer store.Close()
	version, err := store.GetResourceVersion(ctx, "client", "engineering-app")
	require.NoError(t, err)
	require.Equal(t, int64(4), version)
	var actionCount int
	require.NoError(t, store.SQLDB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM admin_actions WHERE target_type='client'`).Scan(&actionCount))
	require.Equal(t, 4, actionCount)
}
