package cmds

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminUserMutationsUseGuardedCommandLayer(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "tinyidp.sqlite")
	ownerPassword := filepath.Join(t.TempDir(), "owner-password")
	actionKey := filepath.Join(t.TempDir(), "admin-action-key")
	require.NoError(t, os.WriteFile(ownerPassword, []byte("owner correct horse battery staple\n"), 0o600))
	require.NoError(t, os.WriteFile(actionKey, []byte("0123456789abcdef0123456789abcdef"), 0o600))

	runAdminCLI(t, ctx,
		"--db", dbPath, "console", "bootstrap",
		"--owner-login", "owner",
		"--owner-password-file", ownerPassword,
		"--public-base-url", "https://id.example",
	)
	runAdminCLI(t, ctx,
		"--db", dbPath, "user", "--admin-action-key-file", actionKey,
		"create", "--login", "alice", "--password", "alice correct horse battery staple",
		"--email", "alice@example.test", "--name", "Alice",
	)
	runAdminCLI(t, ctx,
		"--db", dbPath, "user", "--admin-action-key-file", actionKey,
		"set-password", "--login", "alice", "--password", "alice replacement battery staple",
		"--reason", "Credential rotation",
	)
	runAdminCLI(t, ctx,
		"--db", dbPath, "user", "--admin-action-key-file", actionKey,
		"disable", "--login", "alice", "--reason", "Access review", "--confirm", "DISABLE",
	)
	output := runAdminCLI(t, ctx, "--db", dbPath, "user", "get", "--login", "alice")
	require.Contains(t, output, `"Disabled": true`)
	runAdminCLI(t, ctx,
		"--db", dbPath, "user", "--admin-action-key-file", actionKey,
		"enable", "--login", "alice", "--reason", "Access restored",
	)
	output = runAdminCLI(t, ctx, "--db", dbPath, "user", "get", "--login", "alice")
	require.Contains(t, output, `"Disabled": false`)
}

func runAdminCLI(t *testing.T, ctx context.Context, args ...string) string {
	t.Helper()
	command, err := NewAdminCommand()
	require.NoError(t, err)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)
	require.NoError(t, command.ExecuteContext(ctx), output.String())
	return output.String()
}
