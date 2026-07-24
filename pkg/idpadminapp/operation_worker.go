package idpadminapp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-go-golems/tiny-idp/internal/admin"
	"github.com/go-go-golems/tiny-idp/pkg/idp"
	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
	pkgerrors "github.com/pkg/errors"
)

const (
	defaultOperationPoll      = time.Second
	defaultOperationBatchSize = 10
	maximumOperationAge       = 5 * time.Minute
)

type managedOperationStore interface {
	idpadminstore.Store
	idpstore.Store
	Backup(context.Context, string) (sqlitestore.BackupResult, error)
}

type OperationRunner interface {
	Execute(context.Context, idpadminstore.Operation) ([]byte, string, error)
}

type ManagedOperationRunner struct {
	store managedOperationStore
	root  string
	now   func() time.Time
}

func NewManagedOperationRunner(
	store managedOperationStore,
	root string,
	now func() time.Time,
) (*ManagedOperationRunner, error) {
	if store == nil {
		return nil, errors.New("managed operation store is required")
	}
	canonical, err := prepareManagedRoot(root)
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &ManagedOperationRunner{store: store, root: canonical, now: now}, nil
}

func (r *ManagedOperationRunner) Execute(
	ctx context.Context,
	operation idpadminstore.Operation,
) ([]byte, string, error) {
	switch operation.Command {
	case CommandOperationsDoctor:
		service, err := admin.NewService(r.store, admin.Options{Clock: r.now, Audit: idp.NoopSink{}})
		if err != nil {
			return nil, "", err
		}
		result, err := json.Marshal(service.Doctor(ctx))
		return result, "", err
	case CommandBackupCreate:
		path, relative, err := r.generatedPath("backups", operation, ".db")
		if err != nil {
			return nil, "", err
		}
		backup, err := r.store.Backup(ctx, path)
		if err != nil {
			return nil, "", err
		}
		result, err := json.Marshal(map[string]any{
			"bytes": backup.Bytes, "schema_version": backup.Manifest.SchemaVersion,
			"relative_path": relative,
		})
		return result, relative, err
	case CommandBackupVerify:
		var progress struct {
			SourceOperationID string `json:"source_operation_id"`
		}
		if err := json.Unmarshal(operation.Progress, &progress); err != nil {
			return nil, "", err
		}
		source, err := r.store.GetAdminOperation(ctx, progress.SourceOperationID)
		if err != nil {
			return nil, "", err
		}
		if source.Command != CommandBackupCreate || source.Status != "completed" ||
			source.RelativeResultPath == "" {
			return nil, "", errors.New("source backup operation is not completed")
		}
		path, err := r.resolveExisting(source.RelativeResultPath)
		if err != nil {
			return nil, "", err
		}
		manifest, err := sqlitestore.VerifyBackup(ctx, path, nil)
		if err != nil {
			return nil, "", err
		}
		result, err := json.Marshal(map[string]any{
			"verified": true, "schema_version": manifest.SchemaVersion,
			"source_operation_id": source.ID,
		})
		return result, "", err
	case CommandDiagnosticsCreate:
		return r.createDiagnostics(ctx, operation)
	default:
		return nil, "", ErrUnknownCommand
	}
}

func (r *ManagedOperationRunner) createDiagnostics(
	ctx context.Context,
	operation idpadminstore.Operation,
) ([]byte, string, error) {
	service, err := admin.NewService(r.store, admin.Options{Clock: r.now, Audit: idp.NoopSink{}})
	if err != nil {
		return nil, "", err
	}
	clients, err := r.store.ListClients(ctx)
	if err != nil {
		return nil, "", err
	}
	clientRows := make([]idpadmin.ClientRow, 0, len(clients))
	for _, client := range clients {
		version, versionErr := r.store.GetResourceVersion(ctx, "client", client.ID)
		if versionErr != nil {
			return nil, "", versionErr
		}
		clientRows = append(clientRows, idpadmin.ClientRow{
			ID: client.ID, Public: client.Public, Disabled: client.Disabled,
			RequirePKCE: client.RequirePKCE, SecretConfigured: len(client.SecretHash) > 0,
			UpdatedAt: client.UpdatedAt, Version: version,
		})
	}
	keys, err := r.store.VerificationKeys(ctx)
	if err != nil {
		return nil, "", err
	}
	keyRows := make([]idpadmin.SigningKeyRow, 0, len(keys))
	for _, key := range keys {
		keyRows = append(keyRows, signingKeyRow(key))
	}
	operations, err := r.store.ListAdminOperations(ctx, idpadmin.MaxPageSize)
	if err != nil {
		return nil, "", err
	}
	payload := map[string]any{
		"generated_at": r.now().UTC(), "doctor": service.Doctor(ctx),
		"clients": clientRows, "keys": keyRows, "operations": operations,
	}
	path, relative, err := r.generatedPath("diagnostics", operation, ".json")
	if err != nil {
		return nil, "", err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, "", err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	writeErr := encoder.Encode(payload)
	syncErr := file.Sync()
	closeErr := file.Close()
	for _, candidate := range []error{writeErr, syncErr, closeErr} {
		if candidate != nil {
			_ = os.Remove(path)
			return nil, "", candidate
		}
	}
	result, err := json.Marshal(map[string]any{
		"download_available": true, "expires_after": "10m",
	})
	return result, relative, err
}

var safeLabelCharacters = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func (r *ManagedOperationRunner) generatedPath(
	directory string,
	operation idpadminstore.Operation,
	extension string,
) (string, string, error) {
	label := strings.Trim(safeLabelCharacters.ReplaceAllString(operation.Label, "-"), ".-_")
	if len(label) > maxOperationLabel {
		label = label[:maxOperationLabel]
	}
	name := operation.ID
	if label != "" {
		name += "-" + label
	}
	relative := filepath.Join(directory, name+extension)
	path, err := r.resolveNew(relative)
	return path, filepath.ToSlash(relative), err
}

func prepareManagedRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("administration backup root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return "", pkgerrors.Wrap(err, "create administration backup root")
	}
	if err := os.Chmod(absolute, 0o700); err != nil {
		return "", pkgerrors.Wrap(err, "secure administration backup root")
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", pkgerrors.Wrap(err, "resolve administration backup root")
	}
	return filepath.Clean(canonical), nil
}

func (r *ManagedOperationRunner) resolveNew(relative string) (string, error) {
	if err := validateRelativeManagedPath(relative); err != nil {
		return "", err
	}
	path := filepath.Join(r.root, filepath.FromSlash(relative))
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		return "", err
	}
	canonicalParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if !withinRoot(r.root, canonicalParent) {
		return "", errors.New("managed path escapes administration backup root")
	}
	return filepath.Join(canonicalParent, filepath.Base(path)), nil
}

func (r *ManagedOperationRunner) resolveExisting(relative string) (string, error) {
	if err := validateRelativeManagedPath(relative); err != nil {
		return "", err
	}
	path := filepath.Join(r.root, filepath.FromSlash(relative))
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if !withinRoot(r.root, canonical) {
		return "", errors.New("managed path escapes administration backup root")
	}
	return canonical, nil
}

func validateRelativeManagedPath(relative string) error {
	if strings.TrimSpace(relative) == "" || filepath.IsAbs(relative) {
		return errors.New("managed path must be relative")
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("managed path escapes administration backup root")
	}
	return nil
}

func withinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

type OperationWorker struct {
	store        idpadminstore.WorkerStore
	runner       OperationRunner
	now          func() time.Time
	pollInterval time.Duration
}

func NewOperationWorker(
	store idpadminstore.WorkerStore,
	runner OperationRunner,
	now func() time.Time,
) (*OperationWorker, error) {
	if store == nil || runner == nil {
		return nil, errors.New("operation worker store and runner are required")
	}
	if now == nil {
		now = time.Now
	}
	return &OperationWorker{
		store: store, runner: runner, now: now, pollInterval: defaultOperationPoll,
	}, nil
}

func (w *OperationWorker) Run(ctx context.Context) error {
	if _, err := w.store.RequeueRunningAdminOperations(ctx, w.now().UTC()); err != nil {
		return pkgerrors.Wrap(err, "requeue interrupted administration operations")
	}
	_, _ = w.DrainOnce(ctx)
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_, _ = w.DrainOnce(ctx)
		}
	}
}

func (w *OperationWorker) DrainOnce(ctx context.Context) (int, error) {
	operations, err := w.store.ListPendingAdminOperations(ctx, defaultOperationBatchSize)
	if err != nil {
		return 0, err
	}
	completed := 0
	for _, operation := range operations {
		started := w.now().UTC()
		claimed, err := w.store.ClaimAdminOperation(ctx, operation.ID, started)
		if err != nil {
			return completed, err
		}
		if !claimed {
			continue
		}
		result, relativePath, runErr := w.runner.Execute(ctx, operation)
		finished := w.now().UTC()
		if runErr != nil {
			if ctx.Err() != nil {
				// Leave the durable row running. Startup requeues interrupted
				// work before accepting the next operation batch.
				return completed, nil
			}
			if err := w.store.FailAdminOperation(
				ctx, operation.ID, operationErrorCode(operation.Command), finished,
			); err != nil {
				return completed, err
			}
			continue
		}
		if err := w.store.CompleteAdminOperation(
			ctx, operation.ID, result, relativePath, finished,
		); err != nil {
			return completed, err
		}
		completed++
	}
	return completed, nil
}

func (w *OperationWorker) Readiness(ctx context.Context) idp.ReadinessCheck {
	now := w.now().UTC()
	check := idp.ReadinessCheck{Name: "admin_operations", Ready: true, CheckedAt: now}
	health, err := w.store.GetAdminOperationHealth(ctx)
	if err != nil {
		check.Ready = false
		check.Reason = "operation_health_failed"
		return check
	}
	if health.Pending == 0 && health.Running == 0 {
		if health.LastErrorCode != "" {
			check.Degraded = true
			check.Reason = health.LastErrorCode
		}
		return check
	}
	check.Degraded = true
	check.Reason = "operation_pending"
	if health.OldestActive != nil && now.Sub(*health.OldestActive) >= maximumOperationAge {
		check.Ready = false
		check.Reason = "operation_stalled"
	}
	return check
}

func operationErrorCode(command string) string {
	switch command {
	case CommandOperationsDoctor:
		return "doctor_failed"
	case CommandBackupCreate:
		return "backup_create_failed"
	case CommandBackupVerify:
		return "backup_verify_failed"
	case CommandDiagnosticsCreate:
		return "diagnostics_failed"
	default:
		return "operation_failed"
	}
}

var _ OperationRunner = (*ManagedOperationRunner)(nil)
