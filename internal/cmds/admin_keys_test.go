package cmds

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
	"github.com/stretchr/testify/require"
)

func TestAdminKeyRotationAndRetirementUseGuardedCommandLayer(t *testing.T) {
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
	runAdminCLI(t, ctx, "--db", dbPath, "keys", "generate",
		"--kid", "initial-signing-key", "--active")

	output := runAdminCLI(t, ctx,
		"--db", dbPath, "keys", "--admin-action-key-file", actionKey,
		"rotate", "--reason", "Scheduled rotation",
	)
	require.Contains(t, output, `"status": "rotated"`)
	require.NotContains(t, output, "PRIVATE KEY")
	runAdminCLI(t, ctx,
		"--db", dbPath, "keys", "--admin-action-key-file", actionKey,
		"retire", "--kid", "initial-signing-key", "--reason", "Overlap complete",
	)

	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(dbPath))
	require.NoError(t, err)
	var actionCount int
	require.NoError(t, store.SQLDB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM admin_actions WHERE command LIKE 'keys.%'`).Scan(&actionCount))
	require.Equal(t, 2, actionCount)
	active, err := store.ActiveSigningKey(ctx)
	require.NoError(t, err)
	require.NotEqual(t, "initial-signing-key", active.ID)
	require.NoError(t, store.Close())

	// Emergency purge deliberately bypasses the browser action registry and
	// remains available only through this explicit CLI command.
	runAdminCLI(t, ctx, "--db", dbPath, "keys",
		"purge-retired", "--kid", "initial-signing-key")
	store, err = sqlitestore.Open(ctx, sqlitestore.DefaultConfig(dbPath))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.SQLDB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM admin_actions WHERE command LIKE 'keys.%'`).Scan(&actionCount))
	require.Equal(t, 2, actionCount)
	verification, err := store.VerificationKeys(ctx)
	require.NoError(t, err)
	for _, key := range verification {
		require.NotEqual(t, "initial-signing-key", key.ID)
	}
	_, err = store.ActiveSigningKey(ctx)
	require.NotErrorIs(t, err, idpstore.ErrNotFound)
}
