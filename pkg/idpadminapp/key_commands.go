package idpadminapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-go-golems/tiny-idp/internal/keys"
	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

type KeyCommandInput struct {
	Algorithm    string `json:"algorithm,omitempty"`
	Reason       string `json:"reason"`
	Confirmation string `json:"confirmation"`
}

type KeyCommandService struct {
	store    UserCommandStore
	executor *Executor
	now      func() time.Time
	id       func() (string, error)
}

var _ CommandExecutor = (*KeyCommandService)(nil)

func NewKeyCommandService(
	store UserCommandStore,
	executor *Executor,
	now func() time.Time,
) (*KeyCommandService, error) {
	if store == nil || executor == nil {
		return nil, errors.New("key command store and executor are required")
	}
	if now == nil {
		now = time.Now
	}
	return &KeyCommandService{store: store, executor: executor, now: now, id: randomID}, nil
}

func (s *KeyCommandService) Execute(
	ctx context.Context,
	request ExecutionRequest,
	rawInput []byte,
) ([]byte, error) {
	var input KeyCommandInput
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
	if !ok || !strings.HasPrefix(claims.Command, "keys.") {
		return nil, ErrUnknownCommand
	}
	reason := strings.TrimSpace(input.Reason)
	if definition.RequireReason && reason == "" {
		return nil, ErrReasonRequired
	}
	if len(reason) > maxOperatorReason {
		return nil, fmt.Errorf("operator reason exceeds %d characters", maxOperatorReason)
	}
	if input.Confirmation != definition.ConfirmationText {
		return nil, ErrConfirmationFailed
	}
	request.Reason = reason
	if strings.TrimSpace(request.RequestID) == "" {
		request.RequestID, err = randomID()
		if err != nil {
			return nil, err
		}
	}

	now := s.now().UTC()
	var next idpstore.SigningKey
	if claims.Command == CommandKeysRotate {
		algorithm := strings.TrimSpace(input.Algorithm)
		if algorithm != "" && algorithm != "RS256" {
			return nil, fmt.Errorf("unsupported signing algorithm %q", algorithm)
		}
		id, err := s.id()
		if err != nil {
			return nil, err
		}
		next, err = keys.GenerateRSA("rsa-"+id, now)
		if err != nil {
			return nil, err
		}
	} else if claims.Command != CommandKeysRetire {
		return nil, ErrUnknownCommand
	}

	return s.executor.Execute(ctx, request, func(
		ctx context.Context,
		protocol idpstore.TxStore,
		admin idpadminstore.TxStore,
		claims idpadmin.ActionClaims,
	) ([]byte, error) {
		var key idpstore.SigningKey
		switch claims.Command {
		case CommandKeysRotate:
			old, oldErr := protocol.ActiveSigningKey(ctx)
			if oldErr != nil && !errors.Is(oldErr, idpstore.ErrNotFound) {
				return nil, oldErr
			}
			next.Active = false
			if err := protocol.CreateSigningKey(ctx, next); err != nil {
				return nil, err
			}
			if err := protocol.ActivateSigningKey(ctx, next.ID); err != nil {
				return nil, err
			}
			next.Active = true
			key = next
			if oldErr == nil {
				if err := protocol.RetireSigningKey(ctx, old.ID); err != nil {
					return nil, err
				}
				if _, err := admin.GetResourceVersion(ctx, "signing_key", old.ID); errors.Is(err, idpadminstore.ErrNotFound) {
					if err := admin.CreateResourceVersion(ctx, "signing_key", old.ID, 1, now); err != nil {
						return nil, err
					}
				} else if err != nil {
					return nil, err
				}
			}
			if err := admin.CreateResourceVersion(ctx, "signing_key", next.ID, 1, now); err != nil {
				return nil, err
			}
		case CommandKeysRetire:
			keys, err := protocol.VerificationKeys(ctx)
			if err != nil {
				return nil, err
			}
			found := false
			for _, candidate := range keys {
				if candidate.ID == claims.TargetID {
					key = candidate
					found = true
					break
				}
			}
			if !found {
				return nil, idpstore.ErrNotFound
			}
			if key.Active {
				return nil, idpstore.ErrLastSigningKey
			}
			if err := protocol.RetireSigningKey(ctx, claims.TargetID); err != nil {
				return nil, err
			}
			key.Active = false
			if key.NotAfter.IsZero() {
				key.NotAfter = now
			}
		default:
			return nil, ErrUnknownCommand
		}
		return json.Marshal(idpadmin.KeyResult{Key: signingKeyRow(key)})
	})
}

func signingKeyRow(key idpstore.SigningKey) idpadmin.SigningKeyRow {
	var notAfter *time.Time
	if !key.NotAfter.IsZero() {
		value := key.NotAfter.UTC()
		notAfter = &value
	}
	return idpadmin.SigningKeyRow{
		ID: key.ID, Algorithm: key.Algorithm, Active: key.Active,
		CreatedAt: key.CreatedAt.UTC(), NotAfter: notAfter,
	}
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("request contains multiple JSON values")
}
