package memory

import (
	"context"
	"strings"
	"time"

	idpstore "github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

func (s *Store) CreateDurableInvitation(_ context.Context, invitation idpstore.DurableInvitation) error {
	if err := validateDurableInvitation(invitation); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hashKey(invitation.CodeHash)
	if _, exists := s.durableInvitations[key]; exists {
		return idpstore.ErrDuplicate
	}
	s.durableInvitations[key] = cloneDurableInvitation(invitation)
	return nil
}

func (s *Store) GetDurableInvitation(_ context.Context, codeHash []byte) (idpstore.DurableInvitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	invitation, ok := s.durableInvitations[hashKey(codeHash)]
	if !ok {
		return idpstore.DurableInvitation{}, idpstore.ErrNotFound
	}
	return cloneDurableInvitation(invitation), nil
}

func (s *Store) GetDurableInvitationByID(_ context.Context, invitationID string) (idpstore.DurableInvitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, invitation := range s.durableInvitations {
		if invitation.ID == strings.TrimSpace(invitationID) {
			return cloneDurableInvitation(invitation), nil
		}
	}
	return idpstore.DurableInvitation{}, idpstore.ErrNotFound
}

func (s *Store) RedeemDurableInvitation(_ context.Context, codeHash []byte, audience string, now time.Time) (idpstore.DurableInvitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hashKey(codeHash)
	invitation, ok := s.durableInvitations[key]
	if !ok {
		return idpstore.DurableInvitation{}, idpstore.ErrNotFound
	}
	if invitation.Audience != audience {
		return idpstore.DurableInvitation{}, idpstore.ErrNotFound
	}
	if invitation.RevokedAt != nil {
		return idpstore.DurableInvitation{}, idpstore.ErrInvitationRevoked
	}
	if !invitation.ExpiresAt.After(now) {
		return idpstore.DurableInvitation{}, idpstore.ErrExpired
	}
	if invitation.RedeemedAt != nil {
		return idpstore.DurableInvitation{}, idpstore.ErrAlreadyConsumed
	}
	at := now.UTC()
	invitation.RedeemedAt = &at
	s.durableInvitations[key] = invitation
	return cloneDurableInvitation(invitation), nil
}

func (s *Store) RevokeDurableInvitation(_ context.Context, codeHash []byte, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hashKey(codeHash)
	invitation, ok := s.durableInvitations[key]
	if !ok {
		return idpstore.ErrNotFound
	}
	if invitation.RedeemedAt != nil {
		return idpstore.ErrAlreadyConsumed
	}
	if invitation.RevokedAt == nil {
		at := now.UTC()
		invitation.RevokedAt = &at
		s.durableInvitations[key] = invitation
	}
	return nil
}

func (s *Store) RevokeDurableInvitationByID(_ context.Context, invitationID string, now time.Time) (idpstore.DurableInvitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, invitation := range s.durableInvitations {
		if invitation.ID != strings.TrimSpace(invitationID) {
			continue
		}
		if invitation.RedeemedAt != nil {
			return idpstore.DurableInvitation{}, idpstore.ErrAlreadyConsumed
		}
		if invitation.RevokedAt == nil {
			at := now.UTC()
			invitation.RevokedAt = &at
			s.durableInvitations[key] = invitation
		}
		return cloneDurableInvitation(invitation), nil
	}
	return idpstore.DurableInvitation{}, idpstore.ErrNotFound
}

func validateDurableInvitation(invitation idpstore.DurableInvitation) error {
	if len(invitation.CodeHash) < 32 || strings.TrimSpace(invitation.ID) == "" || strings.TrimSpace(invitation.Audience) == "" || strings.TrimSpace(invitation.PolicyVersion) == "" || invitation.ExpiresAt.IsZero() || invitation.RevokedAt != nil || invitation.RedeemedAt != nil || invitation.RedeemedEvidence != "" {
		return idpstore.ErrDuplicate
	}
	return nil
}
