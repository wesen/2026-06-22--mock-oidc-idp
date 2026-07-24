package sqlitestore

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/pkg/errors"

	idpstore "github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

func (s *Store) CreateDurableInvitation(ctx context.Context, invitation idpstore.DurableInvitation) error {
	if err := validateDurableInvitation(invitation); err != nil {
		return err
	}
	if s.runner == nil {
		return s.Update(ctx, func(tx idpstore.TxStore) error { return tx.CreateDurableInvitation(ctx, invitation) })
	}
	data, err := enc(invitation)
	if err != nil {
		return errors.Wrap(err, "encode durable invitation")
	}
	_, err = s.conn().ExecContext(ctx, `INSERT INTO durable_invitations(code_hash, invitation_id, expires_at_ns, data) VALUES(?, ?, ?, ?)`,
		invitation.CodeHash, invitation.ID, invitation.ExpiresAt.UTC().UnixNano(), data)
	if err != nil {
		return idpstore.ErrDuplicate
	}
	return nil
}

func (s *Store) GetDurableInvitationByID(ctx context.Context, invitationID string) (idpstore.DurableInvitation, error) {
	var data []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM durable_invitations WHERE invitation_id=?`,
		strings.TrimSpace(invitationID)).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return idpstore.DurableInvitation{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.DurableInvitation{}, errors.Wrap(err, "load durable invitation by id")
	}
	invitation, err := dec[idpstore.DurableInvitation](data)
	if err != nil {
		return idpstore.DurableInvitation{}, errors.Wrap(err, "decode durable invitation")
	}
	return invitation, nil
}

func (s *Store) GetDurableInvitation(ctx context.Context, codeHash []byte) (idpstore.DurableInvitation, error) {
	var data []byte
	err := s.conn().QueryRowContext(ctx, `SELECT data FROM durable_invitations WHERE code_hash=?`, codeHash).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return idpstore.DurableInvitation{}, idpstore.ErrNotFound
	}
	if err != nil {
		return idpstore.DurableInvitation{}, errors.Wrap(err, "load durable invitation")
	}
	invitation, err := dec[idpstore.DurableInvitation](data)
	if err != nil {
		return idpstore.DurableInvitation{}, errors.Wrap(err, "decode durable invitation")
	}
	return invitation, nil
}

func (s *Store) RedeemDurableInvitation(ctx context.Context, codeHash []byte, audience string, now time.Time) (idpstore.DurableInvitation, error) {
	if s.runner == nil {
		var invitation idpstore.DurableInvitation
		err := s.Update(ctx, func(tx idpstore.TxStore) error {
			var err error
			invitation, err = tx.RedeemDurableInvitation(ctx, codeHash, audience, now)
			return err
		})
		return invitation, err
	}
	invitation, err := s.GetDurableInvitation(ctx, codeHash)
	if err != nil {
		return idpstore.DurableInvitation{}, err
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
	data, err := enc(invitation)
	if err != nil {
		return idpstore.DurableInvitation{}, errors.Wrap(err, "encode redeemed durable invitation")
	}
	result, err := s.conn().ExecContext(ctx, `UPDATE durable_invitations SET redeemed_at_ns=?, data=? WHERE code_hash=? AND redeemed_at_ns IS NULL AND revoked_at_ns IS NULL AND expires_at_ns>?`, at.UnixNano(), data, codeHash, now.UTC().UnixNano())
	if err != nil {
		return idpstore.DurableInvitation{}, errors.Wrap(err, "redeem durable invitation")
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return idpstore.DurableInvitation{}, idpstore.ErrAlreadyConsumed
	}
	return invitation, nil
}

func (s *Store) RevokeDurableInvitation(ctx context.Context, codeHash []byte, now time.Time) error {
	if s.runner == nil {
		return s.Update(ctx, func(tx idpstore.TxStore) error { return tx.RevokeDurableInvitation(ctx, codeHash, now) })
	}
	invitation, err := s.GetDurableInvitation(ctx, codeHash)
	if err != nil {
		return err
	}
	if invitation.RedeemedAt != nil {
		return idpstore.ErrAlreadyConsumed
	}
	if invitation.RevokedAt != nil {
		return nil
	}
	at := now.UTC()
	invitation.RevokedAt = &at
	data, err := enc(invitation)
	if err != nil {
		return errors.Wrap(err, "encode revoked durable invitation")
	}
	result, err := s.conn().ExecContext(ctx, `UPDATE durable_invitations SET revoked_at_ns=?, data=? WHERE code_hash=? AND redeemed_at_ns IS NULL AND revoked_at_ns IS NULL`, at.UnixNano(), data, codeHash)
	if err != nil {
		return errors.Wrap(err, "revoke durable invitation")
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return idpstore.ErrAlreadyConsumed
	}
	return nil
}

func (s *Store) RevokeDurableInvitationByID(ctx context.Context, invitationID string, now time.Time) (idpstore.DurableInvitation, error) {
	if s.runner == nil {
		var invitation idpstore.DurableInvitation
		err := s.Update(ctx, func(tx idpstore.TxStore) error {
			var err error
			invitation, err = tx.RevokeDurableInvitationByID(ctx, invitationID, now)
			return err
		})
		return invitation, err
	}
	invitation, err := s.GetDurableInvitationByID(ctx, invitationID)
	if err != nil {
		return idpstore.DurableInvitation{}, err
	}
	if err := s.RevokeDurableInvitation(ctx, invitation.CodeHash, now); err != nil {
		return idpstore.DurableInvitation{}, err
	}
	return s.GetDurableInvitationByID(ctx, invitationID)
}

func validateDurableInvitation(invitation idpstore.DurableInvitation) error {
	if len(invitation.CodeHash) < 32 || strings.TrimSpace(invitation.ID) == "" || strings.TrimSpace(invitation.Audience) == "" || strings.TrimSpace(invitation.PolicyVersion) == "" || invitation.ExpiresAt.IsZero() || invitation.RevokedAt != nil || invitation.RedeemedAt != nil || invitation.RedeemedEvidence != "" {
		return idpstore.ErrDuplicate
	}
	return nil
}
