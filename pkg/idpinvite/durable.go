package idpinvite

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"time"

	"github.com/pkg/errors"

	idpstore "github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

const durableCodeDomain = "tiny-idp/durable-invitation/v1\x00"

// DurableIssue is the native request to create a one-time invitation. Code is
// immediately transformed into a keyed lookup hash; neither the service nor
// the store retains its browser-visible form.
type DurableIssue struct {
	Code          string
	ID            string
	Audience      string
	PolicyVersion string
	ExpiresAt     time.Time
}

// DurableEvidence is safe to attach to a native signup decision. It contains
// no invitation code or lookup hash and proves only that a particular durable
// invitation was consumed at a specified time.
type DurableEvidence struct {
	InvitationID  string
	Audience      string
	PolicyVersion string
	ExpiresAt     time.Time
	RedeemedAt    time.Time
}

// DurableInspection is the redacted, read-only projection returned while a
// signup program is deciding whether to present or commit. Inspection never
// reserves or redeems the invitation; the native signup transaction must call
// RedeemInTransaction before relying on it.
type DurableInspection struct {
	InvitationID  string    `json:"invitationId"`
	Audience      string    `json:"audience"`
	PolicyVersion string    `json:"policyVersion"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

// DurableRevocation is the redacted, authoritative metadata returned after a
// durable invitation is revoked. Administrative callers use the stored
// audience rather than accepting an operator-supplied audit attribution.
type DurableRevocation struct {
	InvitationID  string
	Audience      string
	PolicyVersion string
	RevokedAt     time.Time
}

// DurableService applies the invitation-code secrecy boundary before asking
// the shared store to create or consume durable state.
type DurableService struct {
	store     idpstore.Store
	lookupKey []byte
}

func NewDurableService(store idpstore.Store, lookupKey []byte) (*DurableService, error) {
	if store == nil || len(lookupKey) < 32 {
		return nil, errors.New("durable invitation store and 32-byte lookup key are required")
	}
	return &DurableService{store: store, lookupKey: append([]byte(nil), lookupKey...)}, nil
}

func (s *DurableService) Issue(ctx context.Context, issue DurableIssue) error {
	if s == nil || s.store == nil {
		return errors.New("durable invitation service is unavailable")
	}
	if err := validateIssue(issue); err != nil {
		return err
	}
	return s.IssueInTransaction(ctx, s.store, issue)
}

// IssueInTransaction derives the private lookup hash and writes through the
// caller-owned transaction. The hash never crosses the idpinvite boundary.
func (s *DurableService) IssueInTransaction(ctx context.Context, store idpstore.DurableInvitationStore, issue DurableIssue) error {
	if s == nil || store == nil {
		return errors.New("durable invitation service is unavailable")
	}
	if err := validateIssue(issue); err != nil {
		return err
	}
	return store.CreateDurableInvitation(ctx, idpstore.DurableInvitation{
		CodeHash:      s.codeHash(issue.Code),
		ID:            issue.ID,
		Audience:      issue.Audience,
		PolicyVersion: issue.PolicyVersion,
		ExpiresAt:     issue.ExpiresAt.UTC(),
	})
}

// RevokeByIDInTransaction resolves the private keyed hash inside the protocol
// store and revokes the invitation using only its public administration ID.
func (s *DurableService) RevokeByIDInTransaction(
	ctx context.Context,
	store idpstore.DurableInvitationStore,
	invitationID string,
	now time.Time,
) (DurableRevocation, error) {
	if s == nil || store == nil || !validText(invitationID) || now.IsZero() {
		return DurableRevocation{}, errors.New("durable invitation revocation request is invalid")
	}
	invitation, err := store.RevokeDurableInvitationByID(ctx, invitationID, now.UTC())
	if err != nil {
		return DurableRevocation{}, err
	}
	if invitation.RevokedAt == nil {
		return DurableRevocation{}, errors.New("durable invitation revocation was not persisted")
	}
	return DurableRevocation{
		InvitationID: invitation.ID, Audience: invitation.Audience,
		PolicyVersion: invitation.PolicyVersion, RevokedAt: invitation.RevokedAt.UTC(),
	}, nil
}

// Inspect validates current invitation state without mutating it. A later
// redemption deliberately repeats every security check inside the caller's
// transaction, closing the time-of-check/time-of-use window.
func (s *DurableService) Inspect(ctx context.Context, code, audience string, now time.Time) (DurableInspection, error) {
	if s == nil || s.store == nil || !validText(code) || !validText(audience) || now.IsZero() {
		return DurableInspection{}, errors.New("durable invitation inspection request is invalid")
	}
	invitation, err := s.store.GetDurableInvitation(ctx, s.codeHash(code))
	if err != nil {
		return DurableInspection{}, err
	}
	if invitation.Audience != audience {
		return DurableInspection{}, idpstore.ErrNotFound
	}
	if invitation.RevokedAt != nil {
		return DurableInspection{}, idpstore.ErrInvitationRevoked
	}
	if !invitation.ExpiresAt.After(now) {
		return DurableInspection{}, idpstore.ErrExpired
	}
	if invitation.RedeemedAt != nil {
		return DurableInspection{}, idpstore.ErrAlreadyConsumed
	}
	return DurableInspection{InvitationID: invitation.ID, Audience: invitation.Audience, PolicyVersion: invitation.PolicyVersion, ExpiresAt: invitation.ExpiresAt.UTC()}, nil
}

// Redeem atomically consumes an invitation in its own transaction. Signup
// committers that need all-or-nothing account creation instead call
// RedeemInTransaction from their already-open store transaction.
func (s *DurableService) Redeem(ctx context.Context, code, audience string, now time.Time) (DurableEvidence, error) {
	if s == nil || s.store == nil {
		return DurableEvidence{}, errors.New("durable invitation service is unavailable")
	}
	var evidence DurableEvidence
	err := s.store.Update(ctx, func(tx idpstore.TxStore) error {
		var err error
		evidence, err = s.RedeemInTransaction(ctx, tx, code, audience, now)
		return err
	})
	return evidence, err
}

// Revoke invalidates an unconsumed invitation by its browser-visible code
// without exposing the derived lookup hash to callers.
func (s *DurableService) Revoke(ctx context.Context, code string, now time.Time) (DurableRevocation, error) {
	if s == nil || s.store == nil || !validText(code) || now.IsZero() {
		return DurableRevocation{}, errors.New("durable invitation revocation request is invalid")
	}
	codeHash := s.codeHash(code)
	if err := s.store.RevokeDurableInvitation(ctx, codeHash, now.UTC()); err != nil {
		return DurableRevocation{}, err
	}
	invitation, err := s.store.GetDurableInvitation(ctx, codeHash)
	if err != nil {
		return DurableRevocation{}, err
	}
	if invitation.RevokedAt == nil {
		return DurableRevocation{}, errors.New("durable invitation revocation was not persisted")
	}
	return DurableRevocation{InvitationID: invitation.ID, Audience: invitation.Audience, PolicyVersion: invitation.PolicyVersion, RevokedAt: invitation.RevokedAt.UTC()}, nil
}

// RedeemInTransaction is the only durable-invitation operation a native
// signup committer needs. It accepts the caller-owned transaction, which
// makes invitation consumption atomic with the rest of that caller's commit.
func (s *DurableService) RedeemInTransaction(ctx context.Context, store idpstore.DurableInvitationStore, code, audience string, now time.Time) (DurableEvidence, error) {
	if s == nil || store == nil || !validText(code) || !validText(audience) || now.IsZero() {
		return DurableEvidence{}, errors.New("durable invitation redemption request is invalid")
	}
	invitation, err := store.RedeemDurableInvitation(ctx, s.codeHash(code), audience, now.UTC())
	if err != nil {
		return DurableEvidence{}, err
	}
	return DurableEvidence{InvitationID: invitation.ID, Audience: invitation.Audience, PolicyVersion: invitation.PolicyVersion, ExpiresAt: invitation.ExpiresAt.UTC(), RedeemedAt: *invitation.RedeemedAt}, nil
}

func (s *DurableService) codeHash(code string) []byte {
	mac := hmac.New(sha256.New, s.lookupKey)
	_, _ = mac.Write([]byte(durableCodeDomain))
	_, _ = mac.Write([]byte(code))
	return mac.Sum(nil)
}

func validateIssue(issue DurableIssue) error {
	if !validText(issue.Code) || len(issue.Code) > 512 || !validText(issue.ID) || !validText(issue.Audience) || !validText(issue.PolicyVersion) || issue.ExpiresAt.IsZero() {
		return errors.New("durable invitation issue is invalid")
	}
	return nil
}
