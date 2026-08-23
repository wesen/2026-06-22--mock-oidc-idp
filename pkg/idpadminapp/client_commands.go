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

	"golang.org/x/crypto/bcrypt"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

type ClientCommandInput struct {
	Public                 *bool    `json:"public,omitempty"`
	RedirectURIs           []string `json:"redirect_uris,omitempty"`
	PostLogoutRedirectURIs []string `json:"post_logout_redirect_uris,omitempty"`
	AllowedScopes          []string `json:"allowed_scopes,omitempty"`
	AllowedGrantTypes      []string `json:"allowed_grant_types,omitempty"`
	AllowedAudiences       []string `json:"allowed_audiences,omitempty"`
	RequirePKCE            *bool    `json:"require_pkce,omitempty"`
	CanIntrospect          *bool    `json:"can_introspect,omitempty"`
	Reason                 string   `json:"reason,omitempty"`
	Confirmation           string   `json:"confirmation,omitempty"`
}

type ValidationError struct {
	FieldErrors map[string]string
	Cause       error
}

func (e *ValidationError) Error() string {
	if e == nil || e.Cause == nil {
		return "administration validation failed"
	}
	return e.Cause.Error()
}

func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type ClientCommandService struct {
	store    UserCommandStore
	executor *Executor
	now      func() time.Time
}

func NewClientCommandService(store UserCommandStore, executor *Executor, now func() time.Time) (*ClientCommandService, error) {
	if store == nil || executor == nil {
		return nil, errors.New("client command store and executor are required")
	}
	if now == nil {
		now = time.Now
	}
	return &ClientCommandService{store: store, executor: executor, now: now}, nil
}

func (s *ClientCommandService) Execute(
	ctx context.Context,
	request ExecutionRequest,
	rawInput []byte,
) ([]byte, error) {
	input, claims, definition, err := decodeClientCommand(ctx, s.executor, request, rawInput)
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

	now := s.now().UTC()
	var client idpstore.Client
	var secret string
	switch claims.Command {
	case CommandClientsCreate:
		client, secret, err = prepareNewClient(claims.TargetID, input, now)
	case CommandClientsUpdate:
		client, err = s.store.GetClient(ctx, claims.TargetID)
		if err == nil {
			err = applyClientConfiguration(&client, input, now)
		}
	case CommandClientsEnable, CommandClientsDisable, CommandClientsRotateSecret:
		client, err = s.store.GetClient(ctx, claims.TargetID)
		if err == nil && claims.Command == CommandClientsRotateSecret {
			if client.Public {
				err = errors.New("public clients do not have secrets")
			} else {
				secret, err = newClientSecret()
				if err == nil {
					client.SecretHash, err = bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
					client.UpdatedAt = now
				}
			}
		}
	default:
		err = ErrUnknownCommand
	}
	if err != nil {
		return nil, err
	}
	if claims.Command == CommandClientsCreate && !client.Public ||
		claims.Command == CommandClientsRotateSecret {
		request.Policy.SecretBearing = true
	}

	return s.executor.Execute(ctx, request, func(
		ctx context.Context,
		protocol idpstore.TxStore,
		admin idpadminstore.TxStore,
		claims idpadmin.ActionClaims,
	) ([]byte, error) {
		switch claims.Command {
		case CommandClientsCreate:
			if _, err := protocol.GetClient(ctx, claims.TargetID); err == nil {
				return nil, idpstore.ErrDuplicate
			} else if !errors.Is(err, idpstore.ErrNotFound) {
				return nil, err
			}
			if err := protocol.PutClient(ctx, client); err != nil {
				return nil, err
			}
			if err := admin.CreateResourceVersion(ctx, "client", claims.TargetID, 1, now); err != nil {
				return nil, err
			}
		case CommandClientsUpdate:
			if err := protocol.PutClient(ctx, client); err != nil {
				return nil, err
			}
		case CommandClientsEnable, CommandClientsDisable:
			current, err := protocol.GetClient(ctx, claims.TargetID)
			if err != nil {
				return nil, err
			}
			current.Disabled = claims.Command == CommandClientsDisable
			current.UpdatedAt = now
			client = current
			if err := protocol.PutClient(ctx, current); err != nil {
				return nil, err
			}
		case CommandClientsRotateSecret:
			if err := protocol.PutClient(ctx, client); err != nil {
				return nil, err
			}
		default:
			return nil, ErrUnknownCommand
		}
		if secret != "" {
			return json.Marshal(idpadmin.OneTimeSecretResult{ResourceID: claims.TargetID, Secret: secret})
		}
		version, err := admin.GetResourceVersion(ctx, "client", claims.TargetID)
		if err != nil {
			return nil, err
		}
		return json.Marshal(idpadmin.ClientResult{Client: clientDetail(client, version)})
	})
}

func prepareNewClient(id string, input ClientCommandInput, now time.Time) (idpstore.Client, string, error) {
	public := input.Public != nil && *input.Public
	client := idpstore.Client{
		ID: id, Public: public, CreatedAt: now, UpdatedAt: now,
		AccessTokenTTL: time.Hour, IDTokenTTL: time.Hour, RefreshTokenTTL: 30 * 24 * time.Hour,
	}
	if !public {
		// Validation only tests presence; the prepared bcrypt hash replaces this
		// marker before the value can enter a transaction.
		client.SecretHash = []byte("pending")
	}
	if err := applyClientConfiguration(&client, input, now); err != nil {
		return idpstore.Client{}, "", err
	}
	if public {
		return client, "", nil
	}
	secret, err := newClientSecret()
	if err != nil {
		return idpstore.Client{}, "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return idpstore.Client{}, "", err
	}
	client.SecretHash = hash
	if err := client.Validate(idpstore.ProductionMode); err != nil {
		return idpstore.Client{}, "", err
	}
	return client, secret, nil
}

func applyClientConfiguration(client *idpstore.Client, input ClientCommandInput, now time.Time) error {
	if input.Public != nil {
		if *input.Public != client.Public && !client.CreatedAt.Equal(now) {
			return errors.New("client public/confidential type cannot be changed")
		}
		client.Public = *input.Public
	}
	client.RedirectURIs = cleanStringList(input.RedirectURIs)
	client.PostLogoutRedirectURIs = cleanStringList(input.PostLogoutRedirectURIs)
	client.AllowedScopes = cleanStringList(input.AllowedScopes)
	client.AllowedGrantTypes = cleanStringList(input.AllowedGrantTypes)
	client.AllowedAudiences = cleanStringList(input.AllowedAudiences)
	if input.RequirePKCE != nil {
		client.RequirePKCE = *input.RequirePKCE
	}
	if client.Public {
		client.RequirePKCE = true
	}
	if input.CanIntrospect != nil {
		client.CanIntrospect = *input.CanIntrospect
	}
	client.UpdatedAt = now
	if err := client.Validate(idpstore.ProductionMode); err != nil {
		return classifyClientValidation(err)
	}
	return nil
}

func classifyClientValidation(err error) error {
	field := "client"
	switch {
	case errors.Is(err, idpstore.ErrEmptyClientID):
		field = "id"
	case errors.Is(err, idpstore.ErrEmptyRedirectURI),
		errors.Is(err, idpstore.ErrInvalidRedirectURI),
		errors.Is(err, idpstore.ErrWildcardRedirectURI),
		errors.Is(err, idpstore.ErrRedirectURIFragment),
		errors.Is(err, idpstore.ErrProductionRedirectHTTP):
		field = "redirect_uris"
	case errors.Is(err, idpstore.ErrClientMissingGrantTypes),
		errors.Is(err, idpstore.ErrClientGrantTypeInvalid),
		errors.Is(err, idpstore.ErrClientGrantTypeDuplicate):
		field = "allowed_grant_types"
	case errors.Is(err, idpstore.ErrPublicClientRequiresPKCE):
		field = "require_pkce"
	case errors.Is(err, idpstore.ErrPublicClientHasSecret),
		errors.Is(err, idpstore.ErrConfidentialMissingSecret):
		field = "public"
	default:
		if strings.Contains(err.Error(), "audience") {
			field = "allowed_audiences"
		} else if strings.Contains(err.Error(), "introspect") {
			field = "can_introspect"
		}
	}
	return &ValidationError{FieldErrors: map[string]string{field: err.Error()}, Cause: err}
}

func cleanStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func newClientSecret() (string, error) {
	first, err := randomID()
	if err != nil {
		return "", err
	}
	second, err := randomID()
	if err != nil {
		return "", err
	}
	return first + second, nil
}

func clientDetail(client idpstore.Client, version int64) idpadmin.ClientDetail {
	return idpadmin.ClientDetail{
		ClientRow: idpadmin.ClientRow{
			ID: client.ID, Public: client.Public, Disabled: client.Disabled,
			RequirePKCE: client.RequirePKCE, SecretConfigured: len(client.SecretHash) != 0,
			UpdatedAt: client.UpdatedAt, Version: version,
		},
		RedirectURIs:           append([]string(nil), client.RedirectURIs...),
		PostLogoutRedirectURIs: append([]string(nil), client.PostLogoutRedirectURIs...),
		AllowedScopes:          append([]string(nil), client.AllowedScopes...),
		AllowedGrantTypes:      append([]string(nil), client.AllowedGrantTypes...),
		AllowedAudiences:       append([]string(nil), client.AllowedAudiences...),
		CanIntrospect:          client.CanIntrospect,
	}
}

func decodeClientCommand(
	ctx context.Context,
	executor *Executor,
	request ExecutionRequest,
	rawInput []byte,
) (ClientCommandInput, idpadmin.ActionClaims, ActionDefinition, error) {
	if len(rawInput) == 0 || len(rawInput) > maxUserInputBytes {
		return ClientCommandInput{}, idpadmin.ActionClaims{}, ActionDefinition{},
			fmt.Errorf("client command input must contain at most %d bytes", maxUserInputBytes)
	}
	defer clearBytes(rawInput)
	var input ClientCommandInput
	decoder := json.NewDecoder(bytes.NewReader(rawInput))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return ClientCommandInput{}, idpadmin.ActionClaims{}, ActionDefinition{}, fmt.Errorf("decode client command input: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ClientCommandInput{}, idpadmin.ActionClaims{}, ActionDefinition{}, errors.New("client command input contains trailing JSON")
	}
	claims, err := executor.Preflight(ctx, request)
	if err != nil {
		return ClientCommandInput{}, idpadmin.ActionClaims{}, ActionDefinition{}, err
	}
	definition, ok := actionDefinition(claims.Command)
	if !ok || definition.TargetType != "client" ||
		definition.Capability != claims.Capability || definition.RequireFresh != claims.RequireFresh {
		return ClientCommandInput{}, idpadmin.ActionClaims{}, ActionDefinition{}, ErrUnknownCommand
	}
	return input, claims, definition, nil
}
