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

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpinvite"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

const (
	invitationPolicyVersion = "signup-invite-v1"
	maxInvitationLifetime   = 30 * 24 * time.Hour
)

type InvitationCommandInput struct {
	Label        string `json:"label,omitempty"`
	Audience     string `json:"audience,omitempty"`
	ValidFor     string `json:"valid_for,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Confirmation string `json:"confirmation,omitempty"`
}

type InvitationCommandService struct {
	store       UserCommandStore
	executor    *Executor
	invitations *idpinvite.DurableService
	now         func() time.Time
}

func NewInvitationCommandService(
	store UserCommandStore,
	executor *Executor,
	invitations *idpinvite.DurableService,
	now func() time.Time,
) (*InvitationCommandService, error) {
	if store == nil || executor == nil || invitations == nil {
		return nil, errors.New("invitation command store, executor, and durable service are required")
	}
	if now == nil {
		now = time.Now
	}
	return &InvitationCommandService{store: store, executor: executor, invitations: invitations, now: now}, nil
}

func (s *InvitationCommandService) Execute(
	ctx context.Context,
	request ExecutionRequest,
	rawInput []byte,
) ([]byte, error) {
	input, claims, definition, err := decodeInvitationCommand(ctx, s.executor, request, rawInput)
	if err != nil {
		return nil, err
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
	request.Reason = reason
	if strings.TrimSpace(request.RequestID) == "" {
		request.RequestID, err = randomID()
		if err != nil {
			return nil, err
		}
	}

	var code string
	var validFor time.Duration
	if claims.Command == CommandInvitationsIssue {
		code, err = randomID()
		if err != nil {
			return nil, err
		}
		// Two random identifiers provide 288 bits of browser-visible entropy.
		second, randomErr := randomID()
		if randomErr != nil {
			return nil, randomErr
		}
		code += second
		validFor, err = time.ParseDuration(strings.TrimSpace(input.ValidFor))
		if err != nil || validFor <= 0 || validFor > maxInvitationLifetime {
			return nil, fmt.Errorf("valid_for must be a positive duration no greater than %s", maxInvitationLifetime)
		}
		if strings.TrimSpace(input.Audience) == "" {
			return nil, errors.New("audience is required")
		}
		if _, err := s.store.GetClient(ctx, strings.TrimSpace(input.Audience)); err != nil {
			return nil, fmt.Errorf("load invitation audience: %w", err)
		}
		request.Policy.SecretBearing = true
	}

	return s.executor.Execute(ctx, request, func(
		ctx context.Context,
		protocol idpstore.TxStore,
		admin idpadminstore.TxStore,
		claims idpadmin.ActionClaims,
	) ([]byte, error) {
		now := s.now().UTC()
		switch claims.Command {
		case CommandInvitationsIssue:
			expiresAt := now.Add(validFor)
			if err := s.invitations.IssueInTransaction(ctx, protocol, idpinvite.DurableIssue{
				Code: code, ID: claims.TargetID, Audience: strings.TrimSpace(input.Audience),
				PolicyVersion: invitationPolicyVersion, ExpiresAt: expiresAt,
			}); err != nil {
				return nil, err
			}
			if err := admin.CreateAdminInvitation(ctx, idpadminstore.InvitationRecord{
				InvitationID: claims.TargetID, Label: input.Label,
				CreatedBySubject: claims.Subject, CreatedAt: now, LastIssuedAt: now,
			}); err != nil {
				return nil, err
			}
			if err := admin.CreateResourceVersion(ctx, "invitation", claims.TargetID, 1, now); err != nil {
				return nil, err
			}
			return json.Marshal(idpadmin.OneTimeSecretResult{ResourceID: claims.TargetID, Secret: code})
		case CommandInvitationsRevoke:
			if _, err := s.invitations.RevokeByIDInTransaction(ctx, protocol, claims.TargetID, now); err != nil {
				return nil, err
			}
			invitation, err := protocol.GetDurableInvitationByID(ctx, claims.TargetID)
			if err != nil {
				return nil, err
			}
			version, err := admin.GetResourceVersion(ctx, "invitation", claims.TargetID)
			if err != nil {
				return nil, err
			}
			return json.Marshal(idpadmin.InvitationResult{Invitation: idpadmin.InvitationRow{
				ID: invitation.ID, Audience: invitation.Audience, Status: "revoked",
				ExpiresAt: invitation.ExpiresAt, RevokedAt: invitation.RevokedAt,
				RedeemedAt: invitation.RedeemedAt, Version: version,
			}})
		default:
			return nil, ErrUnknownCommand
		}
	})
}

func decodeInvitationCommand(
	ctx context.Context,
	executor *Executor,
	request ExecutionRequest,
	rawInput []byte,
) (InvitationCommandInput, idpadmin.ActionClaims, ActionDefinition, error) {
	if len(rawInput) == 0 || len(rawInput) > maxUserInputBytes {
		return InvitationCommandInput{}, idpadmin.ActionClaims{}, ActionDefinition{},
			fmt.Errorf("invitation command input must contain at most %d bytes", maxUserInputBytes)
	}
	defer clearBytes(rawInput)
	var input InvitationCommandInput
	decoder := json.NewDecoder(bytes.NewReader(rawInput))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return InvitationCommandInput{}, idpadmin.ActionClaims{}, ActionDefinition{}, fmt.Errorf("decode invitation command input: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return InvitationCommandInput{}, idpadmin.ActionClaims{}, ActionDefinition{}, errors.New("invitation command input contains trailing JSON")
	}
	claims, err := executor.Preflight(ctx, request)
	if err != nil {
		return InvitationCommandInput{}, idpadmin.ActionClaims{}, ActionDefinition{}, err
	}
	definition, ok := actionDefinition(claims.Command)
	if !ok || definition.TargetType != "invitation" ||
		definition.Capability != claims.Capability || definition.RequireFresh != claims.RequireFresh {
		return InvitationCommandInput{}, idpadmin.ActionClaims{}, ActionDefinition{}, ErrUnknownCommand
	}
	return input, claims, definition, nil
}
