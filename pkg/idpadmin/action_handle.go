package idpadmin

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const actionHandleVersion = "v1"

type ActionClaims struct {
	Version         int        `json:"version"`
	Nonce           string     `json:"nonce"`
	SessionID       string     `json:"session_id"`
	Subject         string     `json:"subject"`
	GrantID         string     `json:"grant_id"`
	GrantVersion    int64      `json:"grant_version"`
	Scope           AdminScope `json:"scope"`
	Capability      Capability `json:"capability"`
	Command         string     `json:"command"`
	TargetType      string     `json:"target_type,omitempty"`
	TargetID        string     `json:"target_id,omitempty"`
	ExpectedVersion int64      `json:"expected_version,omitempty"`
	RequireFresh    bool       `json:"require_fresh,omitempty"`
	IssuedAt        time.Time  `json:"issued_at"`
	ExpiresAt       time.Time  `json:"expires_at"`
}

type HandleService struct {
	key []byte
	now func() time.Time
	rng io.Reader
	ttl time.Duration
}

func NewHandleService(key []byte, ttl time.Duration, now func() time.Time) (*HandleService, error) {
	if len(key) < 32 {
		return nil, errors.New("action handle key must contain at least 32 bytes")
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if ttl > 5*time.Minute {
		return nil, errors.New("action handle TTL must not exceed five minutes")
	}
	if now == nil {
		now = time.Now
	}
	return &HandleService{key: bytes.Clone(key), now: now, rng: rand.Reader, ttl: ttl}, nil
}

func (s *HandleService) Mint(claims ActionClaims) (string, error) {
	now := s.now().UTC()
	if claims.Nonce == "" {
		nonce := make([]byte, 32)
		if _, err := io.ReadFull(s.rng, nonce); err != nil {
			return "", errors.Wrap(err, "generate action nonce")
		}
		claims.Nonce = base64.RawURLEncoding.EncodeToString(nonce)
	}
	claims.Version = 1
	claims.IssuedAt = now
	claims.ExpiresAt = now.Add(s.ttl)
	if err := validateClaims(claims); err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", errors.Wrap(err, "encode action claims")
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(payload)
	signature := s.sign(actionHandleVersion + "." + payloadPart)
	return actionHandleVersion + "." + payloadPart + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (s *HandleService) Verify(raw string, principal AdminPrincipal) (ActionClaims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 || parts[0] != actionHandleVersion {
		return ActionClaims{}, ErrInvalidAction
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, s.sign(parts[0]+"."+parts[1])) {
		return ActionClaims{}, ErrInvalidAction
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ActionClaims{}, ErrInvalidAction
	}
	var claims ActionClaims
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&claims); err != nil {
		return ActionClaims{}, errors.Wrap(ErrInvalidAction, err.Error())
	}
	if err := validateClaims(claims); err != nil {
		return ActionClaims{}, err
	}
	now := s.now().UTC()
	if !now.Before(claims.ExpiresAt) {
		return ActionClaims{}, ErrExpiredAction
	}
	if claims.IssuedAt.After(now) || claims.ExpiresAt.Sub(claims.IssuedAt) > 5*time.Minute {
		return ActionClaims{}, ErrInvalidAction
	}
	if claims.SessionID != principal.SessionID {
		return ActionClaims{}, ErrActionSession
	}
	if claims.Subject != principal.Subject {
		return ActionClaims{}, ErrActionSubject
	}
	return claims, nil
}

func (s *HandleService) sign(value string) []byte {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func validateClaims(c ActionClaims) error {
	if c.Version != 1 || c.Nonce == "" || c.SessionID == "" || c.Subject == "" ||
		c.GrantID == "" || c.GrantVersion < 1 || strings.TrimSpace(c.Command) == "" ||
		c.IssuedAt.IsZero() || c.ExpiresAt.IsZero() || !c.ExpiresAt.After(c.IssuedAt) {
		return ErrInvalidAction
	}
	if err := c.Scope.ValidateMVP(); err != nil {
		return err
	}
	if err := c.Capability.Validate(); err != nil {
		return err
	}
	if c.TargetID != "" && c.TargetType == "" {
		return fmt.Errorf("%w: target type is required", ErrInvalidAction)
	}
	return nil
}
