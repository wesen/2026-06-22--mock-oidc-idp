// Package idpadmin owns the administration control-plane security model.
// Presentation layers may request operations through this package, but they
// must never make authorization decisions themselves.
package idpadmin

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

type ScopeKind string

const (
	ScopeSystem ScopeKind = "system"
	ScopeDomain ScopeKind = "domain"
)

// AdminScope is intentionally distinct from the OIDC tenant claim.
type AdminScope struct {
	Kind ScopeKind `json:"kind"`
	ID   string    `json:"id"`
}

func SystemScope() AdminScope { return AdminScope{Kind: ScopeSystem, ID: "system"} }

func (s AdminScope) Validate() error {
	switch s.Kind {
	case ScopeSystem:
		if s.ID != "system" {
			return fmt.Errorf("%w: system scope id must be system", ErrInvalidScope)
		}
	case ScopeDomain:
		if strings.TrimSpace(s.ID) == "" || s.ID == "system" {
			return fmt.Errorf("%w: domain scope requires a non-system id", ErrInvalidScope)
		}
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidScope, s.Kind)
	}
	return nil
}

func (s AdminScope) ValidateMVP() error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s != SystemScope() {
		return ErrUnsupportedScope
	}
	return nil
}

type Capability string

const (
	CapabilityOverviewRead       Capability = "overview.read"
	CapabilityUsersRead          Capability = "users.read"
	CapabilityUsersCreate        Capability = "users.create"
	CapabilityUsersUpdate        Capability = "users.update"
	CapabilityUsersDisable       Capability = "users.disable"
	CapabilityUsersPasswordSet   Capability = "users.password.set"
	CapabilityUsersUnlock        Capability = "users.unlock"
	CapabilityUsersAccessRevoke  Capability = "users.access.revoke"
	CapabilityInvitationsRead    Capability = "invitations.read"
	CapabilityInvitationsCreate  Capability = "invitations.create"
	CapabilityInvitationsRevoke  Capability = "invitations.revoke"
	CapabilityClientsRead        Capability = "clients.read"
	CapabilityClientsCreate      Capability = "clients.create"
	CapabilityClientsUpdate      Capability = "clients.update"
	CapabilityClientsDisable     Capability = "clients.disable"
	CapabilityClientSecretRotate Capability = "clients.secret.rotate"
	CapabilityKeysRead           Capability = "keys.read"
	CapabilityKeysGenerate       Capability = "keys.generate"
	CapabilityKeysRotate         Capability = "keys.rotate"
	CapabilityKeysRetire         Capability = "keys.retire"
	CapabilityActivityRead       Capability = "activity.read"
	CapabilityOperationsRead     Capability = "operations.read"
	CapabilityOperationsDoctor   Capability = "operations.doctor"
	CapabilityBackupCreate       Capability = "operations.backup.create"
	CapabilityBackupVerify       Capability = "operations.backup.verify"
	CapabilityDiagnosticsRead    Capability = "operations.diagnostics"
)

var capabilities = []Capability{
	CapabilityOverviewRead, CapabilityUsersRead, CapabilityUsersCreate,
	CapabilityUsersUpdate, CapabilityUsersDisable, CapabilityUsersPasswordSet,
	CapabilityUsersUnlock, CapabilityUsersAccessRevoke, CapabilityInvitationsRead,
	CapabilityInvitationsCreate, CapabilityInvitationsRevoke, CapabilityClientsRead,
	CapabilityClientsCreate, CapabilityClientsUpdate, CapabilityClientsDisable,
	CapabilityClientSecretRotate, CapabilityKeysRead, CapabilityKeysGenerate,
	CapabilityKeysRotate, CapabilityKeysRetire, CapabilityActivityRead,
	CapabilityOperationsRead, CapabilityOperationsDoctor, CapabilityBackupCreate,
	CapabilityBackupVerify, CapabilityDiagnosticsRead,
}

func AllCapabilities() []Capability { return slices.Clone(capabilities) }

func (c Capability) Validate() error {
	if !slices.Contains(capabilities, c) {
		return fmt.Errorf("%w: %q", ErrUnknownCapability, c)
	}
	return nil
}

type Assurance string

const (
	AssuranceAuthenticated Assurance = "authenticated"
	AssuranceFresh         Assurance = "fresh"
)

type AdminPrincipal struct {
	Subject       string
	SessionID     string
	Authenticated time.Time
	Assurance     Assurance
	GrantID       string
	GrantVersion  int64
}

func (p AdminPrincipal) Validate() error {
	if strings.TrimSpace(p.Subject) == "" || strings.TrimSpace(p.SessionID) == "" || p.Authenticated.IsZero() {
		return ErrInvalidPrincipal
	}
	switch p.Assurance {
	case AssuranceAuthenticated, AssuranceFresh:
		return nil
	default:
		return ErrInvalidPrincipal
	}
}

type Grant struct {
	ID           string
	ActorSubject string
	Scope        AdminScope
	Role         string
	Capabilities []Capability
	Version      int64
	IssuedAt     time.Time
	ExpiresAt    *time.Time
	RevokedAt    *time.Time
}

func (g Grant) Validate() error {
	if strings.TrimSpace(g.ID) == "" || strings.TrimSpace(g.ActorSubject) == "" || strings.TrimSpace(g.Role) == "" ||
		g.Version < 1 || g.IssuedAt.IsZero() {
		return ErrInvalidGrant
	}
	if err := g.Scope.ValidateMVP(); err != nil {
		return err
	}
	if len(g.Capabilities) == 0 {
		return ErrInvalidGrant
	}
	for _, capability := range g.Capabilities {
		if err := capability.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (g Grant) Active(now time.Time) bool {
	return g.RevokedAt == nil && (g.ExpiresAt == nil || now.Before(*g.ExpiresAt))
}

func (g Grant) Has(capability Capability) bool {
	return slices.Contains(g.Capabilities, capability)
}

var (
	ErrInvalidScope      = errors.New("invalid administration scope")
	ErrUnsupportedScope  = errors.New("administration scope is not supported by this release")
	ErrUnknownCapability = errors.New("unknown administration capability")
	ErrInvalidPrincipal  = errors.New("invalid administration principal")
	ErrInvalidGrant      = errors.New("invalid administration grant")
	ErrGrantNotFound     = errors.New("administration grant not found")
	ErrGrantInactive     = errors.New("administration grant is inactive")
	ErrGrantChanged      = errors.New("administration grant changed")
	ErrGrantSubject      = errors.New("administration grant subject mismatch")
	ErrGrantScope        = errors.New("administration grant scope mismatch")
	ErrCapabilityDenied  = errors.New("administration capability denied")
	ErrFreshAuthRequired = errors.New("fresh authentication required")
	ErrInvalidAction     = errors.New("invalid administration action handle")
	ErrExpiredAction     = errors.New("administration action handle expired")
	ErrActionSession     = errors.New("administration action session mismatch")
	ErrActionSubject     = errors.New("administration action subject mismatch")
)

type GrantReader interface {
	GetAdminGrant(ctx context.Context, id string) (Grant, error)
}

type Authorizer struct {
	grants   GrantReader
	now      func() time.Time
	freshFor time.Duration
}

func NewAuthorizer(grants GrantReader, freshFor time.Duration, now func() time.Time) (*Authorizer, error) {
	if grants == nil {
		return nil, errors.New("grant reader is required")
	}
	if freshFor <= 0 {
		freshFor = 5 * time.Minute
	}
	if now == nil {
		now = time.Now
	}
	return &Authorizer{grants: grants, freshFor: freshFor, now: now}, nil
}

func (a *Authorizer) Authorize(
	ctx context.Context,
	principal AdminPrincipal,
	grantID string,
	grantVersion int64,
	scope AdminScope,
	capability Capability,
	requireFresh bool,
) (Grant, error) {
	if err := principal.Validate(); err != nil {
		return Grant{}, err
	}
	if err := scope.ValidateMVP(); err != nil {
		return Grant{}, err
	}
	if err := capability.Validate(); err != nil {
		return Grant{}, err
	}
	grant, err := a.grants.GetAdminGrant(ctx, grantID)
	if err != nil {
		return Grant{}, err
	}
	now := a.now()
	if err := grant.Validate(); err != nil {
		return Grant{}, err
	}
	if !grant.Active(now) {
		return Grant{}, ErrGrantInactive
	}
	if grant.Version != grantVersion {
		return Grant{}, ErrGrantChanged
	}
	if grant.ActorSubject != principal.Subject {
		return Grant{}, ErrGrantSubject
	}
	if grant.Scope != scope {
		return Grant{}, ErrGrantScope
	}
	if !grant.Has(capability) {
		return Grant{}, ErrCapabilityDenied
	}
	if requireFresh && (principal.Assurance != AssuranceFresh || now.Sub(principal.Authenticated) > a.freshFor) {
		return Grant{}, ErrFreshAuthRequired
	}
	return grant, nil
}
