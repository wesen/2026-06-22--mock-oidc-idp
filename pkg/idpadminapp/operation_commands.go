package idpadminapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

const maxOperationLabel = 80

type OperationCommandInput struct {
	Label        string `json:"label,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Confirmation string `json:"confirmation,omitempty"`
}

type OperationCommandService struct {
	executor *Executor
	now      func() time.Time
	id       func() (string, error)
}

var _ CommandExecutor = (*OperationCommandService)(nil)

func NewOperationCommandService(
	store idpadminstore.Store,
	executor *Executor,
	now func() time.Time,
) (*OperationCommandService, error) {
	if store == nil || executor == nil {
		return nil, errors.New("operation command store and executor are required")
	}
	if now == nil {
		now = time.Now
	}
	return &OperationCommandService{executor: executor, now: now, id: randomID}, nil
}

func (s *OperationCommandService) Execute(
	ctx context.Context,
	request ExecutionRequest,
	rawInput []byte,
) ([]byte, error) {
	var input OperationCommandInput
	decoder := json.NewDecoder(bytes.NewReader(rawInput))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	claims, err := s.executor.Preflight(ctx, request)
	if err != nil {
		return nil, err
	}
	definition, ok := actionDefinition(claims.Command)
	if !ok || !strings.HasPrefix(claims.Command, "operations.") {
		return nil, ErrUnknownCommand
	}
	reason := strings.TrimSpace(input.Reason)
	if definition.RequireReason && reason == "" {
		return nil, ErrReasonRequired
	}
	if len(reason) > maxOperatorReason {
		return nil, fmt.Errorf("operator reason exceeds %d characters", maxOperatorReason)
	}
	if definition.ConfirmationText != "" && input.Confirmation != definition.ConfirmationText {
		return nil, ErrConfirmationFailed
	}
	label := strings.TrimSpace(input.Label)
	if len(label) > maxOperationLabel {
		return nil, fmt.Errorf("operation label exceeds %d characters", maxOperationLabel)
	}
	request.Reason = reason
	if strings.TrimSpace(request.RequestID) == "" {
		request.RequestID, err = randomID()
		if err != nil {
			return nil, err
		}
	}
	operationID := claims.TargetID
	sourceOperationID := ""
	if claims.Command == CommandBackupVerify {
		sourceOperationID = claims.TargetID
		operationID, err = s.id()
		if err != nil {
			return nil, err
		}
	}
	progress, err := json.Marshal(map[string]string{"source_operation_id": sourceOperationID})
	if err != nil {
		return nil, err
	}
	kind, ok := operationKind(claims.Command)
	if !ok {
		return nil, ErrUnknownCommand
	}
	now := s.now().UTC()

	return s.executor.Execute(ctx, request, func(
		ctx context.Context,
		_ idpstore.TxStore,
		admin idpadminstore.TxStore,
		claims idpadmin.ActionClaims,
	) ([]byte, error) {
		if claims.Command == CommandBackupVerify {
			source, err := admin.GetResourceVersion(ctx, "operation", sourceOperationID)
			if err != nil {
				return nil, err
			}
			if source < 1 {
				return nil, idpadminstore.ErrNotFound
			}
		}
		if operationID != claims.TargetID {
			if err := admin.CreateResourceVersion(ctx, "operation", operationID, 1, now); err != nil {
				return nil, err
			}
		} else if _, err := admin.GetResourceVersion(ctx, "operation", operationID); errors.Is(err, idpadminstore.ErrNotFound) {
			if err := admin.CreateResourceVersion(ctx, "operation", operationID, 1, now); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
		operation := idpadminstore.Operation{
			ID: operationID, Kind: kind, Command: claims.Command,
			ActorSubject: claims.Subject, Status: "pending", Label: label,
			Progress: progress, CreatedAt: now, UpdatedAt: now,
		}
		if err := admin.CreateAdminOperation(ctx, operation); err != nil {
			return nil, err
		}
		return json.Marshal(idpadmin.OperationResult{Operation: operationRow(operation)})
	})
}

func operationKind(command string) (string, bool) {
	switch command {
	case CommandOperationsDoctor:
		return "doctor", true
	case CommandBackupCreate:
		return "backup", true
	case CommandBackupVerify:
		return "backup_verify", true
	case CommandDiagnosticsCreate:
		return "diagnostics", true
	default:
		return "", false
	}
}

func operationRow(operation idpadminstore.Operation) idpadmin.OperationRow {
	return idpadmin.OperationRow{
		ID: operation.ID, Kind: operation.Kind, Status: operation.Status,
		CreatedAt: operation.CreatedAt, UpdatedAt: operation.UpdatedAt,
		CompletedAt: operation.CompletedAt, ErrorCode: operation.ErrorCode,
	}
}
