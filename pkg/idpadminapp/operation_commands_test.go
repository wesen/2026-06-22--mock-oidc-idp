package idpadminapp_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/stretchr/testify/require"
)

func TestManagedOperationsRunWithinRootAndDiagnosticsDownloadOnce(t *testing.T) {
	fixture := newResourceCommandFixture(t)
	root := filepath.Join(t.TempDir(), "managed")
	runner, err := idpadminapp.NewManagedOperationRunner(fixture.store, root, func() time.Time {
		return fixture.now
	})
	require.NoError(t, err)
	worker, err := idpadminapp.NewOperationWorker(fixture.store, runner, func() time.Time {
		return fixture.now
	})
	require.NoError(t, err)

	doctor := enqueueOperation(t, fixture, idpadminapp.CommandOperationsDoctor, "", map[string]any{})
	require.Equal(t, "pending", doctor.Operation.Status)
	drainOperation(t, fixture.ctx, worker)
	doctorRecord, err := fixture.store.GetAdminOperation(fixture.ctx, doctor.Operation.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", doctorRecord.Status)
	require.Empty(t, doctorRecord.RelativeResultPath)

	backup := enqueueOperation(t, fixture, idpadminapp.CommandBackupCreate, "", map[string]any{
		"label": "../../nightly owner backup", "reason": "Before maintenance", "confirmation": "BACKUP",
	})
	drainOperation(t, fixture.ctx, worker)
	backupRecord, err := fixture.store.GetAdminOperation(fixture.ctx, backup.Operation.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", backupRecord.Status)
	require.NotContains(t, backupRecord.RelativeResultPath, "..")
	backupPath := filepath.Join(root, filepath.FromSlash(backupRecord.RelativeResultPath))
	info, err := os.Stat(backupPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	verify := enqueueOperation(t, fixture, idpadminapp.CommandBackupVerify, backup.Operation.ID, map[string]any{
		"reason": "Post-backup verification", "confirmation": "VERIFY",
	})
	drainOperation(t, fixture.ctx, worker)
	verifyRecord, err := fixture.store.GetAdminOperation(fixture.ctx, verify.Operation.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", verifyRecord.Status)
	require.Contains(t, string(verifyRecord.Result), `"verified":true`)

	diagnostics := enqueueOperation(t, fixture, idpadminapp.CommandDiagnosticsCreate, "", map[string]any{
		"label": "support", "reason": "Investigate readiness", "confirmation": "DIAGNOSTICS",
	})
	drainOperation(t, fixture.ctx, worker)
	diagnosticsRecord, err := fixture.store.GetAdminOperation(fixture.ctx, diagnostics.Operation.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", diagnosticsRecord.Status)
	diagnosticsPath := filepath.Join(root, filepath.FromSlash(diagnosticsRecord.RelativeResultPath))
	content, err := os.ReadFile(diagnosticsPath)
	require.NoError(t, err)
	require.NotContains(t, string(content), "PRIVATE KEY")
	require.NotContains(t, string(content), "SecretHash")

	authorizer, err := idpadmin.NewAuthorizer(fixture.store, 5*time.Minute, func() time.Time {
		return fixture.now
	})
	require.NoError(t, err)
	downloads, err := idpadminapp.NewDownloadService(
		fixture.store, authorizer, root, func() time.Time { return fixture.now },
	)
	require.NoError(t, err)
	grant, err := downloads.Issue(fixture.ctx, fixture.principal, diagnostics.Operation.ID)
	require.NoError(t, err)
	require.NotEmpty(t, grant.Handle)
	consumed, err := downloads.Consume(fixture.ctx, fixture.principal, grant.Handle)
	require.NoError(t, err)
	require.Equal(t, diagnosticsPath, consumed.Path)
	_, err = downloads.Consume(fixture.ctx, fixture.principal, grant.Handle)
	require.ErrorIs(t, err, idpadminstore.ErrNotFound)
}

func TestManagedOperationRejectsStoredPathEscapeAndSymlinkParent(t *testing.T) {
	fixture := newResourceCommandFixture(t)
	root := filepath.Join(t.TempDir(), "managed")
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(root, 0o700))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "backups")))
	runner, err := idpadminapp.NewManagedOperationRunner(fixture.store, root, fixture.nowTime)
	require.NoError(t, err)
	_, _, err = runner.Execute(fixture.ctx, idpadminstore.Operation{
		ID: "operation-escape", Command: idpadminapp.CommandBackupCreate,
		Label: "../../outside",
	})
	require.Error(t, err)
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, entries)

	now := fixture.now
	source := idpadminstore.Operation{
		ID: "source-escape", Kind: "backup", Command: idpadminapp.CommandBackupCreate,
		ActorSubject: fixture.principal.Subject, Status: "pending",
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, fixture.store.CreateAdminOperation(fixture.ctx, source))
	claimed, err := fixture.store.ClaimAdminOperation(fixture.ctx, source.ID, now)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, fixture.store.CompleteAdminOperation(
		fixture.ctx, source.ID, []byte(`{}`), "../escape.db", now,
	))
	progress, err := json.Marshal(map[string]string{"source_operation_id": source.ID})
	require.NoError(t, err)
	_, _, err = runner.Execute(fixture.ctx, idpadminstore.Operation{
		ID: "verify-escape", Command: idpadminapp.CommandBackupVerify, Progress: progress,
	})
	require.Error(t, err)
}

func TestBrowserActionRegistryExcludesBreakGlassOperations(t *testing.T) {
	fixture := newResourceCommandFixture(t)
	for _, command := range []string{
		"operations.backup.restore",
		"operations.migrate",
		"keys.purge",
	} {
		_, err := fixture.actions.Prepare(
			fixture.ctx,
			fixture.principal,
			idpadminapp.PrepareActionRequest{Command: command, TargetID: "target"},
		)
		require.ErrorIs(t, err, idpadminapp.ErrUnknownCommand, command)
	}
}

func (f *resourceCommandFixture) nowTime() time.Time { return f.now }

func enqueueOperation(
	t *testing.T,
	fixture *resourceCommandFixture,
	command, targetID string,
	input map[string]any,
) idpadmin.OperationResult {
	t.Helper()
	response, _ := fixture.execute(
		t, fixture.operations, fixture.prepare(t, command, targetID), input,
	)
	var result idpadmin.OperationResult
	require.NoError(t, json.Unmarshal(response, &result))
	return result
}

func drainOperation(
	t *testing.T,
	ctx context.Context,
	worker *idpadminapp.OperationWorker,
) {
	t.Helper()
	completed, err := worker.DrainOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, completed)
}
