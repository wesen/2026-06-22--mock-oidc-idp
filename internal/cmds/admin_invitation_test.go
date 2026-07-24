package cmds

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	cmd_sources "github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/tiny-idp/pkg/idpinvite"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
	"github.com/stretchr/testify/require"
)

func TestAdminInvitationIssueAndRevokeUseGuardedCommandLayer(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	dbPath := filepath.Join(directory, "tinyidp.sqlite")
	lookupKeyPath := filepath.Join(directory, "invitation.key")
	actionKeyPath := filepath.Join(directory, "admin-action.key")
	ownerPassword := filepath.Join(directory, "owner-password")
	lookupKey := []byte("0123456789abcdef0123456789abcdef")
	require.NoError(t, os.WriteFile(lookupKeyPath, lookupKey, 0o600))
	require.NoError(t, os.WriteFile(actionKeyPath, []byte("abcdef0123456789abcdef0123456789"), 0o600))
	require.NoError(t, os.WriteFile(ownerPassword, []byte("owner correct horse battery staple\n"), 0o600))
	runAdminCLI(t, ctx, "--db", dbPath, "console", "bootstrap",
		"--owner-login", "owner", "--owner-password-file", ownerPassword,
		"--public-base-url", "https://id.example")

	issue, err := newAdminInvitationIssueCommand(&dbPath)
	require.NoError(t, err)
	issueValues := values.New()
	require.NoError(t, cmd_sources.Execute(issue.Schema, issueValues, cmd_sources.FromMap(map[string]map[string]any{
		"default": {
			"lookup-key-file": lookupKeyPath, "admin-action-key-file": actionKeyPath,
			"audience": "tinyidp-admin-console", "label": "CLI onboarding", "ttl": "1h",
		},
	})))
	processor := &captureProcessor{}
	require.NoError(t, issue.RunIntoGlazeProcessor(ctx, issueValues, processor))
	require.Len(t, processor.rows, 1)
	code := rowVal(processor.rows[0], "code")
	invitationID := rowVal(processor.rows[0], "invitation_id")
	require.NotEmpty(t, code)
	require.NotEmpty(t, invitationID)

	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(dbPath))
	require.NoError(t, err)
	service, err := idpinvite.NewDurableService(store, lookupKey)
	require.NoError(t, err)
	_, err = service.Inspect(ctx, code, "tinyidp-admin-console", issue.now())
	require.NoError(t, err)
	require.NoError(t, store.Close())

	revoke, err := newAdminInvitationRevokeCommand(&dbPath)
	require.NoError(t, err)
	revokeValues := values.New()
	require.NoError(t, cmd_sources.Execute(revoke.Schema, revokeValues, cmd_sources.FromMap(map[string]map[string]any{
		"default": {
			"lookup-key-file": lookupKeyPath, "admin-action-key-file": actionKeyPath,
			"invitation-id": invitationID, "reason": "Recipient changed", "confirm": "REVOKE",
		},
	})))
	processor = &captureProcessor{}
	require.NoError(t, revoke.RunIntoGlazeProcessor(ctx, revokeValues, processor))
	require.Equal(t, "revoked", rowVal(processor.rows[0], "status"))

	store, err = sqlitestore.Open(ctx, sqlitestore.DefaultConfig(dbPath))
	require.NoError(t, err)
	defer store.Close()
	service, err = idpinvite.NewDurableService(store, lookupKey)
	require.NoError(t, err)
	_, err = service.Inspect(ctx, code, "tinyidp-admin-console", revoke.now())
	require.Error(t, err)
	var actions int
	require.NoError(t, store.SQLDB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM admin_actions WHERE target_type='invitation'`).Scan(&actions))
	require.Equal(t, 2, actions)
}

func TestAdminCommandBuildsGlazedInvitationChildren(t *testing.T) {
	admin, err := NewAdminCommand()
	require.NoError(t, err)
	invitation, _, err := admin.Find([]string{"invitation"})
	require.NoError(t, err)
	require.NotNil(t, invitation)
	require.Len(t, invitation.Commands(), 2)
}
