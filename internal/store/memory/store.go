package memory

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpcontinuation"
	idpstore "github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

// Store is a concurrency-safe in-memory implementation of idpstore.Store. It is
// intended for tests, examples, and dev-mode strict engine runs.
type Store struct {
	mu            sync.Mutex
	inTransaction bool

	clients            map[string]idpstore.Client
	usersByID          map[string]idpstore.User
	usersByLogin       map[string]string
	usersBySubject     map[string]string
	displayNameClaims  map[string]string
	credentialsByUser  map[string]idpstore.PasswordCredential
	credentialsByLogin map[string]string
	accountSecurity    map[string]idpstore.AccountSecurityState
	grants             map[string]idpstore.Grant
	codes              map[string]idpstore.AuthorizationCode
	access             map[string]idpstore.AccessToken
	refresh            map[string]idpstore.RefreshToken
	consents           map[string]idpstore.Consent
	sessions           map[string]idpstore.Session
	browserContexts    map[string]idpstore.BrowserContext
	rememberedSessions map[string]idpstore.RememberedBrowserSession
	interactions       map[string]idpstore.InteractionRecord
	deviceGrants       map[string]idpstore.DeviceGrant
	deviceByUserCode   map[string]string
	durableInvitations map[string]idpstore.DurableInvitation
	keys               map[string]idpstore.SigningKey
	continuations      map[string]idpcontinuation.WorkflowContinuation
}

var _ idpstore.Store = (*Store)(nil)
var _ idpstore.MaintenanceStore = (*Store)(nil)
var _ idpcontinuation.Store = (*Store)(nil)

func New() *Store {
	return &Store{
		clients:            map[string]idpstore.Client{},
		usersByID:          map[string]idpstore.User{},
		usersByLogin:       map[string]string{},
		usersBySubject:     map[string]string{},
		displayNameClaims:  map[string]string{},
		credentialsByUser:  map[string]idpstore.PasswordCredential{},
		credentialsByLogin: map[string]string{},
		accountSecurity:    map[string]idpstore.AccountSecurityState{},
		grants:             map[string]idpstore.Grant{},
		codes:              map[string]idpstore.AuthorizationCode{},
		access:             map[string]idpstore.AccessToken{},
		refresh:            map[string]idpstore.RefreshToken{},
		consents:           map[string]idpstore.Consent{},
		sessions:           map[string]idpstore.Session{},
		browserContexts:    map[string]idpstore.BrowserContext{},
		rememberedSessions: map[string]idpstore.RememberedBrowserSession{},
		interactions:       map[string]idpstore.InteractionRecord{},
		deviceGrants:       map[string]idpstore.DeviceGrant{},
		deviceByUserCode:   map[string]string{},
		durableInvitations: map[string]idpstore.DurableInvitation{},
		keys:               map[string]idpstore.SigningKey{},
		continuations:      map[string]idpcontinuation.WorkflowContinuation{},
	}
}

func (s *Store) DisplayNameAvailable(_ context.Context, normalized string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, claimed := s.displayNameClaims[normalized]
	return !claimed, nil
}

func (s *Store) ReserveDisplayName(_ context.Context, normalized, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, claimed := s.displayNameClaims[normalized]; claimed && existing != userID {
		return idpstore.ErrDisplayNameTaken
	}
	s.displayNameClaims[normalized] = userID
	return nil
}

func hashKey(b []byte) string { return hex.EncodeToString(b) }

func (s *Store) PutClient(_ context.Context, c idpstore.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[c.ID] = c
	return nil
}

func (s *Store) GetClient(_ context.Context, id string) (idpstore.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clients[id]
	if !ok {
		return idpstore.Client{}, idpstore.ErrNotFound
	}
	return c, nil
}

func (s *Store) ListClients(_ context.Context) ([]idpstore.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]idpstore.Client, 0, len(s.clients))
	for _, c := range s.clients {
		out = append(out, c)
	}
	return out, nil
}

func (s *Store) PutUser(_ context.Context, login string, u idpstore.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existingID, ok := s.usersBySubject[u.Sub]; ok && existingID != u.ID {
		return idpstore.ErrDuplicate
	}
	if login != "" {
		if existingID, ok := s.usersByLogin[login]; ok && existingID != u.ID {
			return idpstore.ErrDuplicate
		}
	}
	if previous, ok := s.usersByID[u.ID]; ok && previous.Sub != u.Sub {
		delete(s.usersBySubject, previous.Sub)
	}
	s.usersByID[u.ID] = u
	s.usersBySubject[u.Sub] = u.ID
	if login != "" {
		s.usersByLogin[login] = u.ID
	}
	return nil
}

func (s *Store) GetUser(_ context.Context, id string) (idpstore.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.usersByID[id]
	if !ok {
		return idpstore.User{}, idpstore.ErrNotFound
	}
	return u, nil
}

func (s *Store) GetUserByLogin(_ context.Context, login string) (idpstore.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.usersByLogin[login]
	if !ok {
		return idpstore.User{}, idpstore.ErrNotFound
	}
	u, ok := s.usersByID[id]
	if !ok {
		return idpstore.User{}, idpstore.ErrNotFound
	}
	return u, nil
}

func (s *Store) GetUserBySubject(_ context.Context, subject string) (idpstore.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.usersBySubject[subject]
	if !ok {
		return idpstore.User{}, idpstore.ErrNotFound
	}
	u, ok := s.usersByID[id]
	if !ok {
		return idpstore.User{}, idpstore.ErrNotFound
	}
	return u, nil
}

func (s *Store) PutPasswordCredential(_ context.Context, credential idpstore.PasswordCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existingUserID, ok := s.credentialsByLogin[credential.Login]; ok && existingUserID != credential.UserID {
		return idpstore.ErrDuplicate
	}
	if old, ok := s.credentialsByUser[credential.UserID]; ok && old.Login != credential.Login {
		delete(s.credentialsByLogin, old.Login)
	}
	s.credentialsByUser[credential.UserID] = credential
	s.credentialsByLogin[credential.Login] = credential.UserID
	return nil
}

func (s *Store) GetPasswordCredentialByLogin(_ context.Context, login string) (idpstore.PasswordCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID, ok := s.credentialsByLogin[login]
	if !ok {
		return idpstore.PasswordCredential{}, idpstore.ErrNotFound
	}
	credential, ok := s.credentialsByUser[userID]
	if !ok {
		return idpstore.PasswordCredential{}, idpstore.ErrNotFound
	}
	return credential, nil
}

func (s *Store) GetPasswordCredentialByUserID(_ context.Context, userID string) (idpstore.PasswordCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	credential, ok := s.credentialsByUser[userID]
	if !ok {
		return idpstore.PasswordCredential{}, idpstore.ErrNotFound
	}
	return credential, nil
}

func (s *Store) DeletePasswordCredential(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	credential, ok := s.credentialsByUser[userID]
	if !ok {
		return idpstore.ErrNotFound
	}
	delete(s.credentialsByUser, userID)
	delete(s.credentialsByLogin, credential.Login)
	return nil
}

func (s *Store) GetAccountSecurityState(_ context.Context, userID string) (idpstore.AccountSecurityState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.accountSecurity[userID]
	if !ok {
		return idpstore.AccountSecurityState{}, idpstore.ErrNotFound
	}
	return state, nil
}

func (s *Store) PutAccountSecurityState(_ context.Context, state idpstore.AccountSecurityState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accountSecurity[state.UserID] = state
	return nil
}

func (s *Store) ResetAccountSecurityState(_ context.Context, userID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.accountSecurity[userID]
	state.UserID = userID
	state.FailedLoginCount = 0
	state.FirstFailedLoginAt = nil
	state.LastFailedLoginAt = nil
	state.LockedUntil = nil
	state.LastSuccessfulLoginAt = &now
	s.accountSecurity[userID] = state
	return nil
}

func (s *Store) CreateGrant(_ context.Context, grant idpstore.Grant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.grants[grant.ID]; ok {
		return idpstore.ErrDuplicate
	}
	s.grants[grant.ID] = grant
	return nil
}

func (s *Store) GetGrant(_ context.Context, id string) (idpstore.Grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.grants[id]
	if !ok {
		return idpstore.Grant{}, idpstore.ErrNotFound
	}
	return g, nil
}

func (s *Store) RevokeGrant(_ context.Context, id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.grants[id]
	if !ok {
		return idpstore.ErrNotFound
	}
	g.RevokedAt = &at
	s.grants[id] = g
	return nil
}

func (s *Store) CreateAuthorizationCode(_ context.Context, code idpstore.AuthorizationCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := hashKey(code.CodeHash)
	if _, ok := s.codes[k]; ok {
		return idpstore.ErrDuplicate
	}
	s.codes[k] = code
	return nil
}

func (s *Store) ConsumeAuthorizationCode(_ context.Context, codeHash []byte, now time.Time) (idpstore.AuthorizationCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := hashKey(codeHash)
	code, ok := s.codes[k]
	if !ok {
		return idpstore.AuthorizationCode{}, idpstore.ErrNotFound
	}
	if code.ConsumedAt != nil {
		return idpstore.AuthorizationCode{}, idpstore.ErrAlreadyConsumed
	}
	if !code.ExpiresAt.IsZero() && now.After(code.ExpiresAt) {
		return idpstore.AuthorizationCode{}, idpstore.ErrExpired
	}
	consumed := now
	code.ConsumedAt = &consumed
	s.codes[k] = code
	return code, nil
}

func (s *Store) CreateAccessToken(_ context.Context, token idpstore.AccessToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := hashKey(token.TokenHash)
	if _, ok := s.access[k]; ok {
		return idpstore.ErrDuplicate
	}
	s.access[k] = token
	return nil
}

func (s *Store) GetAccessToken(_ context.Context, tokenHash []byte) (idpstore.AccessToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.access[hashKey(tokenHash)]
	if !ok {
		return idpstore.AccessToken{}, idpstore.ErrNotFound
	}
	return t, nil
}

func (s *Store) RevokeAccessToken(_ context.Context, tokenHash []byte, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := hashKey(tokenHash)
	t, ok := s.access[k]
	if !ok {
		return idpstore.ErrNotFound
	}
	t.RevokedAt = &at
	s.access[k] = t
	return nil
}

func (s *Store) CreateRefreshToken(_ context.Context, token idpstore.RefreshToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := hashKey(token.TokenHash)
	if _, ok := s.refresh[k]; ok {
		return idpstore.ErrDuplicate
	}
	s.refresh[k] = token
	return nil
}

func (s *Store) GetRefreshToken(_ context.Context, tokenHash []byte) (idpstore.RefreshToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.refresh[hashKey(tokenHash)]
	if !ok {
		return idpstore.RefreshToken{}, idpstore.ErrNotFound
	}
	return t, nil
}

func (s *Store) RotateRefreshToken(_ context.Context, oldHash []byte, next idpstore.RefreshToken, now time.Time) (idpstore.RefreshToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	oldKey := hashKey(oldHash)
	old, ok := s.refresh[oldKey]
	if !ok {
		return idpstore.RefreshToken{}, idpstore.ErrNotFound
	}
	if old.RevokedAt != nil || len(old.ReplacedByHash) > 0 || old.ReuseDetectedAt != nil {
		detected := now
		old.ReuseDetectedAt = &detected
		s.refresh[oldKey] = old
		s.revokeRefreshFamilyLocked(old.GrantID, now)
		return idpstore.RefreshToken{}, idpstore.ErrRefreshReuseDetected
	}
	if !old.ExpiresAt.IsZero() && now.After(old.ExpiresAt) {
		return idpstore.RefreshToken{}, idpstore.ErrExpired
	}
	next.ParentTokenHash = append([]byte(nil), oldHash...)
	old.ReplacedByHash = append([]byte(nil), next.TokenHash...)
	s.refresh[oldKey] = old
	s.refresh[hashKey(next.TokenHash)] = next
	return next, nil
}

func (s *Store) RevokeRefreshTokenFamily(_ context.Context, tokenHash []byte, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.refresh[hashKey(tokenHash)]
	if !ok {
		return idpstore.ErrNotFound
	}
	s.revokeRefreshFamilyLocked(t.GrantID, at)
	return nil
}

func (s *Store) revokeRefreshFamilyLocked(grantID string, at time.Time) {
	for k, t := range s.refresh {
		if t.GrantID == grantID && t.RevokedAt == nil {
			t.RevokedAt = &at
			s.refresh[k] = t
		}
	}
}

func consentKey(userID, clientID string, scopes []string) string {
	return userID + "\x00" + clientID + "\x00" + strings.Join(idpstore.NormalizeScopes(scopes), " ")
}

func (s *Store) PutConsent(_ context.Context, consent idpstore.Consent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	consent.Scope = idpstore.NormalizeScopes(consent.Scope)
	s.consents[consentKey(consent.UserID, consent.ClientID, consent.Scope)] = consent
	return nil
}

func (s *Store) GetConsent(_ context.Context, userID, clientID string, scopes []string) (idpstore.Consent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.consents[consentKey(userID, clientID, scopes)]
	if !ok {
		return idpstore.Consent{}, idpstore.ErrNotFound
	}
	return c, nil
}

func (s *Store) RevokeConsent(_ context.Context, userID, clientID string, scopes []string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := consentKey(userID, clientID, scopes)
	c, ok := s.consents[k]
	if !ok {
		return idpstore.ErrNotFound
	}
	c.RevokedAt = &at
	s.consents[k] = c
	return nil
}

func (s *Store) CreateSession(_ context.Context, session idpstore.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := hashKey(session.IDHash)
	if _, ok := s.sessions[k]; ok {
		return idpstore.ErrDuplicate
	}
	s.sessions[k] = session
	return nil
}

func (s *Store) GetSession(_ context.Context, idHash []byte) (idpstore.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[hashKey(idHash)]
	if !ok {
		return idpstore.Session{}, idpstore.ErrNotFound
	}
	return sess, nil
}

func (s *Store) RevokeSession(_ context.Context, idHash []byte, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := hashKey(idHash)
	sess, ok := s.sessions[k]
	if !ok {
		return idpstore.ErrNotFound
	}
	sess.RevokedAt = &at
	s.sessions[k] = sess
	return nil
}

func (s *Store) CreateBrowserContext(_ context.Context, context idpstore.BrowserContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hashKey(context.IDHash)
	if _, ok := s.browserContexts[key]; ok {
		return idpstore.ErrDuplicate
	}
	s.browserContexts[key] = cloneBrowserContext(context)
	return nil
}

func (s *Store) GetBrowserContext(_ context.Context, contextHash []byte) (idpstore.BrowserContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	context, ok := s.browserContexts[hashKey(contextHash)]
	if !ok {
		return idpstore.BrowserContext{}, idpstore.ErrNotFound
	}
	return cloneBrowserContext(context), nil
}

func (s *Store) CreateRememberedBrowserSession(_ context.Context, remembered idpstore.RememberedBrowserSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hashKey(remembered.IDHash)
	if _, ok := s.rememberedSessions[key]; ok {
		return idpstore.ErrDuplicate
	}
	s.rememberedSessions[key] = cloneRememberedBrowserSession(remembered)
	return nil
}

func (s *Store) ListRememberedBrowserSessions(_ context.Context, contextHash []byte, now time.Time) ([]idpstore.RememberedBrowserSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.browserContextActiveLocked(contextHash, now) {
		return nil, idpstore.ErrNotFound
	}
	entries := make([]idpstore.RememberedBrowserSession, 0)
	for _, remembered := range s.rememberedSessions {
		if !equalHash(remembered.ContextIDHash, contextHash) || remembered.RemovedAt != nil {
			continue
		}
		session, ok := s.sessions[hashKey(remembered.SessionIDHash)]
		if !ok || !sessionActive(session, now) || session.UserID != remembered.UserID {
			continue
		}
		user, ok := s.usersByID[remembered.UserID]
		if !ok || user.Disabled {
			continue
		}
		entries = append(entries, cloneRememberedBrowserSession(remembered))
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].LastUsedAt.Equal(entries[j].LastUsedAt) {
			return bytes.Compare(entries[i].IDHash, entries[j].IDHash) < 0
		}
		return entries[i].LastUsedAt.After(entries[j].LastUsedAt)
	})
	return entries, nil
}

func (s *Store) ActivateRememberedSession(ctx context.Context, contextHash, entryHash, newSessionHash []byte, now time.Time) (idpstore.Session, idpstore.User, error) {
	if s.inTransaction {
		return s.activateRememberedSession(contextHash, entryHash, newSessionHash, now)
	}
	var session idpstore.Session
	var user idpstore.User
	err := s.Update(ctx, func(tx idpstore.TxStore) error {
		scoped, ok := tx.(*Store)
		if !ok {
			return errors.New("unexpected memory transaction implementation")
		}
		var err error
		session, user, err = scoped.activateRememberedSession(contextHash, entryHash, newSessionHash, now)
		return err
	})
	return session, user, err
}

func (s *Store) activateRememberedSession(contextHash, entryHash, newSessionHash []byte, now time.Time) (idpstore.Session, idpstore.User, error) {
	now = now.UTC()
	if !s.browserContextActiveLocked(contextHash, now) {
		return idpstore.Session{}, idpstore.User{}, idpstore.ErrNotFound
	}
	entryKey := hashKey(entryHash)
	remembered, ok := s.rememberedSessions[entryKey]
	if !ok || !equalHash(remembered.ContextIDHash, contextHash) || remembered.RemovedAt != nil {
		return idpstore.Session{}, idpstore.User{}, idpstore.ErrNotFound
	}
	source, ok := s.sessions[hashKey(remembered.SessionIDHash)]
	if !ok || !sessionActive(source, now) || source.UserID != remembered.UserID {
		return idpstore.Session{}, idpstore.User{}, idpstore.ErrNotFound
	}
	user, ok := s.usersByID[source.UserID]
	if !ok || user.Disabled {
		return idpstore.Session{}, idpstore.User{}, idpstore.ErrNotFound
	}
	newKey := hashKey(newSessionHash)
	if _, exists := s.sessions[newKey]; exists {
		return idpstore.Session{}, idpstore.User{}, idpstore.ErrDuplicate
	}
	active := cloneSession(source)
	active.IDHash = append([]byte(nil), newSessionHash...)
	active.CreatedAt = now
	active.LastSeenAt = now
	active.RevokedAt = nil
	s.sessions[newKey] = active
	remembered.LastUsedAt = now
	s.rememberedSessions[entryKey] = remembered
	contextKey := hashKey(contextHash)
	browserContext := s.browserContexts[contextKey]
	browserContext.LastSeenAt = now
	s.browserContexts[contextKey] = browserContext
	return cloneSession(active), user, nil
}

func (s *Store) RemoveRememberedBrowserSession(_ context.Context, contextHash, entryHash []byte, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.browserContextActiveLocked(contextHash, at) {
		return idpstore.ErrNotFound
	}
	entryKey := hashKey(entryHash)
	remembered, ok := s.rememberedSessions[entryKey]
	if !ok || !equalHash(remembered.ContextIDHash, contextHash) || remembered.RemovedAt != nil {
		return idpstore.ErrNotFound
	}
	at = at.UTC()
	remembered.RemovedAt = &at
	s.rememberedSessions[entryKey] = remembered
	return nil
}

func (s *Store) RevokeBrowserContext(_ context.Context, contextHash []byte, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hashKey(contextHash)
	context, ok := s.browserContexts[key]
	if !ok {
		return idpstore.ErrNotFound
	}
	at = at.UTC()
	context.RevokedAt = &at
	s.browserContexts[key] = context
	return nil
}

func (s *Store) browserContextActiveLocked(contextHash []byte, now time.Time) bool {
	context, ok := s.browserContexts[hashKey(contextHash)]
	return ok && context.RevokedAt == nil && (context.ExpiresAt.IsZero() || now.Before(context.ExpiresAt))
}

func sessionActive(session idpstore.Session, now time.Time) bool {
	return session.RevokedAt == nil && (session.ExpiresAt.IsZero() || now.Before(session.ExpiresAt))
}

func equalHash(a, b []byte) bool { return bytes.Equal(a, b) }

func (s *Store) CreateInteraction(_ context.Context, interaction idpstore.InteractionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hashKey(interaction.IDHash)
	if _, ok := s.interactions[key]; ok {
		return idpstore.ErrDuplicate
	}
	s.interactions[key] = cloneInteraction(interaction)
	return nil
}

func (s *Store) GetInteraction(_ context.Context, idHash []byte) (idpstore.InteractionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	interaction, ok := s.interactions[hashKey(idHash)]
	if !ok {
		return idpstore.InteractionRecord{}, idpstore.ErrNotFound
	}
	return cloneInteraction(interaction), nil
}

func (s *Store) ConsumeInteraction(_ context.Context, idHash []byte, now time.Time, outcome idpstore.InteractionOutcome) (idpstore.InteractionRecord, error) {
	if !outcome.Valid() {
		return idpstore.InteractionRecord{}, idpstore.ErrInvalidInteractionOutcome
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hashKey(idHash)
	interaction, ok := s.interactions[key]
	if !ok {
		return idpstore.InteractionRecord{}, idpstore.ErrNotFound
	}
	if interaction.ConsumedAt != nil {
		return idpstore.InteractionRecord{}, idpstore.ErrAlreadyConsumed
	}
	if !interaction.ExpiresAt.IsZero() && !now.Before(interaction.ExpiresAt) {
		return idpstore.InteractionRecord{}, idpstore.ErrExpired
	}
	now = now.UTC()
	interaction.ConsumedAt = &now
	interaction.Outcome = outcome
	s.interactions[key] = interaction
	return cloneInteraction(interaction), nil
}

func cloneInteraction(interaction idpstore.InteractionRecord) idpstore.InteractionRecord {
	interaction.IDHash = append([]byte(nil), interaction.IDHash...)
	interaction.RequestDigest = append([]byte(nil), interaction.RequestDigest...)
	interaction.BrowserBindingHash = append([]byte(nil), interaction.BrowserBindingHash...)
	interaction.SessionIDHash = append([]byte(nil), interaction.SessionIDHash...)
	interaction.BrowserContextHash = append([]byte(nil), interaction.BrowserContextHash...)
	interaction.GenerationHash = append([]byte(nil), interaction.GenerationHash...)
	interaction.DeviceUserCodeHash = append([]byte(nil), interaction.DeviceUserCodeHash...)
	request := make(map[string][]string, len(interaction.CanonicalRequest))
	for key, values := range interaction.CanonicalRequest {
		request[key] = append([]string(nil), values...)
	}
	interaction.CanonicalRequest = request
	if interaction.ConsumedAt != nil {
		consumed := *interaction.ConsumedAt
		interaction.ConsumedAt = &consumed
	}
	return interaction
}

const deviceGrantSlowDownIncrement = 5 * time.Second

func (s *Store) CreateDeviceGrant(_ context.Context, grant idpstore.DeviceGrant) error {
	if s.inTransaction {
		return s.createDeviceGrant(grant)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createDeviceGrant(grant)
}

func (s *Store) createDeviceGrant(grant idpstore.DeviceGrant) error {
	if err := grant.ValidateForCreate(); err != nil {
		return err
	}
	deviceKey := hashKey(grant.DeviceCodeHash)
	userKey := hashKey(grant.UserCodeHash)
	if _, exists := s.deviceGrants[deviceKey]; exists {
		return idpstore.ErrDuplicate
	}
	if _, exists := s.deviceByUserCode[userKey]; exists {
		return idpstore.ErrDuplicate
	}
	grant.CreatedAt = grant.CreatedAt.UTC()
	grant.ExpiresAt = grant.ExpiresAt.UTC()
	grant.NextPollAt = grant.NextPollAt.UTC()
	grant.Version = 1
	s.deviceGrants[deviceKey] = cloneDeviceGrant(grant)
	s.deviceByUserCode[userKey] = deviceKey
	return nil
}

func (s *Store) GetDeviceGrantByUserCodeHash(_ context.Context, userCodeHash []byte) (idpstore.DeviceGrant, error) {
	if s.inTransaction {
		return s.deviceGrantByUserCodeHash(userCodeHash)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deviceGrantByUserCodeHash(userCodeHash)
}

func (s *Store) deviceGrantByUserCodeHash(userCodeHash []byte) (idpstore.DeviceGrant, error) {
	deviceKey, exists := s.deviceByUserCode[hashKey(userCodeHash)]
	if !exists {
		return idpstore.DeviceGrant{}, idpstore.ErrNotFound
	}
	grant, exists := s.deviceGrants[deviceKey]
	if !exists {
		return idpstore.DeviceGrant{}, idpstore.ErrNotFound
	}
	return cloneDeviceGrant(grant), nil
}

func (s *Store) InspectDeviceGrantByDeviceCodeHash(_ context.Context, deviceCodeHash []byte, clientID string) (idpstore.DeviceGrant, error) {
	if s.inTransaction {
		return s.inspectDeviceGrant(deviceCodeHash, clientID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inspectDeviceGrant(deviceCodeHash, clientID)
}

func (s *Store) inspectDeviceGrant(deviceCodeHash []byte, clientID string) (idpstore.DeviceGrant, error) {
	grant, exists := s.deviceGrants[hashKey(deviceCodeHash)]
	if !exists || grant.ClientID != clientID {
		return idpstore.DeviceGrant{}, idpstore.ErrNotFound
	}
	return cloneDeviceGrant(grant), nil
}

func (s *Store) PollDeviceGrant(ctx context.Context, request idpstore.DevicePollRequest) (idpstore.DevicePollResult, error) {
	if !s.inTransaction {
		var result idpstore.DevicePollResult
		err := s.Update(ctx, func(tx idpstore.TxStore) error {
			var err error
			result, err = tx.PollDeviceGrant(ctx, request)
			return err
		})
		return result, err
	}
	if request.Now.IsZero() {
		return idpstore.DevicePollResult{}, idpstore.ErrInvalidDeviceGrant
	}
	grant, err := s.inspectDeviceGrant(request.DeviceCodeHash, request.ClientID)
	if err != nil {
		return idpstore.DevicePollResult{}, err
	}
	now := request.Now.UTC()
	if deviceGrantExpired(grant, now) {
		return idpstore.DevicePollResult{Outcome: idpstore.DevicePollExpired, Grant: grant}, nil
	}
	switch grant.Status {
	case idpstore.DeviceGrantDenied:
		return idpstore.DevicePollResult{Outcome: idpstore.DevicePollDenied, Grant: grant}, nil
	case idpstore.DeviceGrantConsumed:
		return idpstore.DevicePollResult{Outcome: idpstore.DevicePollConsumed, Grant: grant}, nil
	case idpstore.DeviceGrantPending, idpstore.DeviceGrantApproved:
	default:
		return idpstore.DevicePollResult{}, idpstore.ErrInvalidDeviceGrant
	}
	if now.Before(grant.NextPollAt) {
		grant.PollInterval += deviceGrantSlowDownIncrement
		grant.NextPollAt = now.Add(grant.PollInterval)
		grant.SlowDownCount++
		grant.Version++
		s.deviceGrants[hashKey(grant.DeviceCodeHash)] = cloneDeviceGrant(grant)
		return idpstore.DevicePollResult{Outcome: idpstore.DevicePollSlowDown, Grant: cloneDeviceGrant(grant)}, nil
	}
	if grant.Status == idpstore.DeviceGrantApproved {
		return idpstore.DevicePollResult{Outcome: idpstore.DevicePollApproved, Grant: grant}, nil
	}
	grant.NextPollAt = now.Add(grant.PollInterval)
	grant.Version++
	s.deviceGrants[hashKey(grant.DeviceCodeHash)] = cloneDeviceGrant(grant)
	return idpstore.DevicePollResult{Outcome: idpstore.DevicePollPending, Grant: cloneDeviceGrant(grant)}, nil
}

func (s *Store) DecideDeviceGrant(ctx context.Context, request idpstore.DeviceDecisionRequest) (idpstore.DeviceGrant, error) {
	if !s.inTransaction {
		var grant idpstore.DeviceGrant
		err := s.Update(ctx, func(tx idpstore.TxStore) error {
			var err error
			grant, err = tx.DecideDeviceGrant(ctx, request)
			return err
		})
		return grant, err
	}
	if request.Now.IsZero() || !request.Decision.Valid() {
		return idpstore.DeviceGrant{}, idpstore.ErrInvalidDeviceDecision
	}
	grant, err := s.deviceGrantByUserCodeHash(request.UserCodeHash)
	if err != nil {
		return idpstore.DeviceGrant{}, err
	}
	now := request.Now.UTC()
	if deviceGrantExpired(grant, now) {
		return idpstore.DeviceGrant{}, idpstore.ErrExpired
	}
	if grant.Status != idpstore.DeviceGrantPending {
		return idpstore.DeviceGrant{}, idpstore.ErrDeviceGrantNotPending
	}
	if request.Decision == idpstore.DeviceGrantApprove && (request.UserID == "" || request.Subject == "" || request.AuthTime.IsZero()) {
		return idpstore.DeviceGrant{}, idpstore.ErrInvalidDeviceDecision
	}
	grant.Status = idpstore.DeviceGrantDenied
	grant.DecidedAt = &now
	if request.Decision == idpstore.DeviceGrantApprove {
		grant.Status = idpstore.DeviceGrantApproved
		grant.UserID = request.UserID
		grant.Subject = request.Subject
		grant.AuthTime = request.AuthTime.UTC()
		grant.AuthenticationMethods = append([]string(nil), request.AuthenticationMethods...)
		grant.ApprovedScopes = append([]string(nil), request.ApprovedScopes...)
		grant.ApprovedAudiences = append([]string(nil), request.ApprovedAudiences...)
	}
	grant.Version++
	s.deviceGrants[hashKey(grant.DeviceCodeHash)] = cloneDeviceGrant(grant)
	return cloneDeviceGrant(grant), nil
}

func (s *Store) ConsumeDeviceGrant(ctx context.Context, request idpstore.DeviceConsumeRequest) (idpstore.DeviceGrant, error) {
	if !s.inTransaction {
		var grant idpstore.DeviceGrant
		err := s.Update(ctx, func(tx idpstore.TxStore) error {
			var err error
			grant, err = tx.ConsumeDeviceGrant(ctx, request)
			return err
		})
		return grant, err
	}
	if request.Now.IsZero() {
		return idpstore.DeviceGrant{}, idpstore.ErrInvalidDeviceGrant
	}
	grant, err := s.inspectDeviceGrant(request.DeviceCodeHash, request.ClientID)
	if err != nil {
		return idpstore.DeviceGrant{}, err
	}
	now := request.Now.UTC()
	if deviceGrantExpired(grant, now) {
		return idpstore.DeviceGrant{}, idpstore.ErrExpired
	}
	if grant.Status == idpstore.DeviceGrantConsumed {
		return idpstore.DeviceGrant{}, idpstore.ErrAlreadyConsumed
	}
	if grant.Status != idpstore.DeviceGrantApproved {
		return idpstore.DeviceGrant{}, idpstore.ErrDeviceGrantNotApproved
	}
	grant.Status = idpstore.DeviceGrantConsumed
	grant.ConsumedAt = &now
	grant.Version++
	s.deviceGrants[hashKey(grant.DeviceCodeHash)] = cloneDeviceGrant(grant)
	return cloneDeviceGrant(grant), nil
}

func deviceGrantExpired(grant idpstore.DeviceGrant, now time.Time) bool {
	return !grant.ExpiresAt.IsZero() && !now.Before(grant.ExpiresAt)
}

func cloneDeviceGrant(grant idpstore.DeviceGrant) idpstore.DeviceGrant {
	grant.DeviceCodeHash = append([]byte(nil), grant.DeviceCodeHash...)
	grant.UserCodeHash = append([]byte(nil), grant.UserCodeHash...)
	grant.RequestedScopes = append([]string(nil), grant.RequestedScopes...)
	grant.ApprovedScopes = append([]string(nil), grant.ApprovedScopes...)
	grant.RequestedAudiences = append([]string(nil), grant.RequestedAudiences...)
	grant.ApprovedAudiences = append([]string(nil), grant.ApprovedAudiences...)
	grant.AuthenticationMethods = append([]string(nil), grant.AuthenticationMethods...)
	if grant.DecidedAt != nil {
		decidedAt := *grant.DecidedAt
		grant.DecidedAt = &decidedAt
	}
	if grant.ConsumedAt != nil {
		consumedAt := *grant.ConsumedAt
		grant.ConsumedAt = &consumedAt
	}
	return grant
}

func (s *Store) CreateSigningKey(_ context.Context, key idpstore.SigningKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.keys[key.ID]; ok {
		return idpstore.ErrDuplicate
	}
	s.keys[key.ID] = key
	return nil
}

func (s *Store) ActiveSigningKey(_ context.Context) (idpstore.SigningKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.keys {
		if k.Active {
			return k, nil
		}
	}
	return idpstore.SigningKey{}, idpstore.ErrNotFound
}

func (s *Store) VerificationKeys(_ context.Context) ([]idpstore.SigningKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]idpstore.SigningKey, 0, len(s.keys))
	for _, k := range s.keys {
		if k.Active || !k.NotAfter.IsZero() {
			out = append(out, k)
		}
	}
	return out, nil
}

func (s *Store) ActivateSigningKey(_ context.Context, kid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.keys[kid]; !ok {
		return idpstore.ErrNotFound
	}
	for id, k := range s.keys {
		k.Active = id == kid
		s.keys[id] = k
	}
	return nil
}

func (s *Store) RetireSigningKey(_ context.Context, kid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.keys[kid]
	if !ok {
		return idpstore.ErrNotFound
	}
	if k.Active {
		return idpstore.ErrLastSigningKey
	}
	k.Active = false
	if k.NotAfter.IsZero() {
		k.NotAfter = time.Now()
	}
	s.keys[kid] = k
	return nil
}

func (s *Store) DeleteRetiredSigningKey(_ context.Context, kid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, ok := s.keys[kid]
	if !ok {
		return idpstore.ErrNotFound
	}
	if key.Active {
		return idpstore.ErrActiveSigningKey
	}
	if key.NotAfter.IsZero() {
		return idpstore.ErrSigningKeyNotRetired
	}
	delete(s.keys, kid)
	return nil
}

func (s *Store) View(ctx context.Context, fn func(idpstore.ReadStore) error) error {
	if fn == nil {
		return errors.New("view callback is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	snapshot := s.cloneLocked()
	s.mu.Unlock()
	return fn(snapshot)
}

func (s *Store) Update(ctx context.Context, fn func(idpstore.TxStore) error) error {
	if fn == nil {
		return errors.New("update callback is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.inTransaction {
		return idpstore.ErrNestedTransaction
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := s.cloneLocked()
	if err := fn(snapshot); err != nil {
		return err
	}
	s.replaceLocked(snapshot)
	return nil
}

func (s *Store) CreateUserWithCredential(ctx context.Context, login string, user idpstore.User, credential idpstore.PasswordCredential) error {
	return s.Update(ctx, func(tx idpstore.TxStore) error {
		if err := tx.PutUser(ctx, login, user); err != nil {
			return err
		}
		return tx.PutPasswordCredential(ctx, credential)
	})
}

func (s *Store) ReplacePasswordAndSecurityState(ctx context.Context, credential idpstore.PasswordCredential, state idpstore.AccountSecurityState) error {
	return s.Update(ctx, func(tx idpstore.TxStore) error {
		if err := tx.PutPasswordCredential(ctx, credential); err != nil {
			return err
		}
		if err := tx.PutAccountSecurityState(ctx, state); err != nil {
			return err
		}
		scoped, ok := tx.(*Store)
		if !ok {
			return errors.New("unexpected memory transaction implementation")
		}
		scoped.revokeUserSecurityArtifacts(credential.UserID, &credential.PasswordChangedAt)
		return nil
	})
}

func (s *Store) SetUserDisabled(ctx context.Context, login string, disabled bool, at time.Time) (idpstore.User, error) {
	var user idpstore.User
	err := s.Update(ctx, func(tx idpstore.TxStore) error {
		loaded, err := tx.GetUserByLogin(ctx, login)
		if err != nil {
			return err
		}
		loaded.Disabled = disabled
		loaded.UpdatedAt = at.UTC()
		if err := tx.PutUser(ctx, login, loaded); err != nil {
			return err
		}
		if disabled {
			scoped, ok := tx.(*Store)
			if !ok {
				return errors.New("unexpected memory transaction implementation")
			}
			scoped.revokeUserSecurityArtifacts(loaded.ID, &loaded.UpdatedAt)
		}
		user = loaded
		return nil
	})
	return user, err
}

func (s *Store) RevokeUserSecurityArtifacts(ctx context.Context, userID string, at time.Time) error {
	return s.Update(ctx, func(tx idpstore.TxStore) error {
		scoped, ok := tx.(*Store)
		if !ok {
			return errors.New("unexpected memory transaction implementation")
		}
		scoped.revokeUserSecurityArtifacts(userID, &at)
		return nil
	})
}

func (s *Store) RevokeUserSecurityArtifactsTx(_ context.Context, userID string, at time.Time) error {
	if !s.inTransaction {
		return errors.New("user security artifact revocation requires a transaction")
	}
	s.revokeUserSecurityArtifacts(userID, &at)
	return nil
}

func (s *Store) revokeUserSecurityArtifacts(userID string, at *time.Time) {
	when := time.Now().UTC()
	if at != nil && !at.IsZero() {
		when = at.UTC()
	}
	for key, grant := range s.grants {
		if grant.UserID == userID && grant.RevokedAt == nil {
			grant.RevokedAt = &when
			s.grants[key] = grant
		}
	}
	for key, code := range s.codes {
		if code.UserID == userID && code.ConsumedAt == nil {
			code.ConsumedAt = &when
			s.codes[key] = code
		}
	}
	for key, token := range s.access {
		if token.UserID == userID && token.RevokedAt == nil {
			token.RevokedAt = &when
			s.access[key] = token
		}
	}
	for key, token := range s.refresh {
		if token.UserID == userID && token.RevokedAt == nil {
			token.RevokedAt = &when
			s.refresh[key] = token
		}
	}
	for key, session := range s.sessions {
		if session.UserID == userID && session.RevokedAt == nil {
			session.RevokedAt = &when
			s.sessions[key] = session
		}
	}
}

func (s *Store) RecordFailedLogin(ctx context.Context, userID string, now time.Time, policy idpstore.LockoutPolicy) (idpstore.AccountSecurityState, error) {
	var state idpstore.AccountSecurityState
	err := s.Update(ctx, func(tx idpstore.TxStore) error {
		loaded, loadErr := tx.GetAccountSecurityState(ctx, userID)
		if loadErr != nil && !errors.Is(loadErr, idpstore.ErrNotFound) {
			return loadErr
		}
		state = loaded
		state.UserID = userID
		if state.FirstFailedLoginAt == nil || (policy.Window > 0 && now.Sub(*state.FirstFailedLoginAt) > policy.Window) {
			state.FailedLoginCount = 0
			first := now
			state.FirstFailedLoginAt = &first
		}
		state.FailedLoginCount++
		last := now
		state.LastFailedLoginAt = &last
		if policy.Threshold > 0 && state.FailedLoginCount >= policy.Threshold {
			lockedUntil := now.Add(policy.Duration)
			state.LockedUntil = &lockedUntil
		}
		return tx.PutAccountSecurityState(ctx, state)
	})
	return state, err
}

func (s *Store) RecordSuccessfulLogin(ctx context.Context, userID string, now time.Time, session *idpstore.Session) error {
	return s.Update(ctx, func(tx idpstore.TxStore) error {
		if err := tx.ResetAccountSecurityState(ctx, userID, now); err != nil {
			return err
		}
		if session != nil {
			return tx.CreateSession(ctx, *session)
		}
		return nil
	})
}

func (s *Store) RotateSigningKey(ctx context.Context, next idpstore.SigningKey, now time.Time) (idpstore.RotationResult, error) {
	var result idpstore.RotationResult
	err := s.Update(ctx, func(tx idpstore.TxStore) error {
		old, oldErr := tx.ActiveSigningKey(ctx)
		if oldErr != nil && !errors.Is(oldErr, idpstore.ErrNotFound) {
			return oldErr
		}
		next.Active = true
		if err := tx.CreateSigningKey(ctx, next); err != nil {
			return err
		}
		if err := tx.ActivateSigningKey(ctx, next.ID); err != nil {
			return err
		}
		if oldErr == nil {
			retired := old
			retired.Active = false
			if retired.NotAfter.IsZero() {
				retired.NotAfter = now
			}
			if err := tx.RetireSigningKey(ctx, old.ID); err != nil {
				return err
			}
			result.Retired = &retired
		}
		result.Active = next
		return nil
	})
	return result, err
}

func (s *Store) cloneLocked() *Store {
	return &Store{
		inTransaction:      true,
		clients:            cloneMap(s.clients),
		usersByID:          cloneMap(s.usersByID),
		usersByLogin:       cloneMap(s.usersByLogin),
		usersBySubject:     cloneMap(s.usersBySubject),
		displayNameClaims:  cloneMap(s.displayNameClaims),
		credentialsByUser:  cloneMap(s.credentialsByUser),
		credentialsByLogin: cloneMap(s.credentialsByLogin),
		accountSecurity:    cloneMap(s.accountSecurity),
		grants:             cloneMap(s.grants),
		codes:              cloneMap(s.codes),
		access:             cloneMap(s.access),
		refresh:            cloneMap(s.refresh),
		consents:           cloneMap(s.consents),
		sessions:           cloneMap(s.sessions),
		browserContexts:    cloneBrowserContextMap(s.browserContexts),
		rememberedSessions: cloneRememberedBrowserSessionMap(s.rememberedSessions),
		interactions:       cloneInteractionMap(s.interactions),
		deviceGrants:       cloneDeviceGrantMap(s.deviceGrants),
		deviceByUserCode:   cloneMap(s.deviceByUserCode),
		durableInvitations: cloneDurableInvitationMap(s.durableInvitations),
		keys:               cloneMap(s.keys),
		continuations:      cloneContinuationMap(s.continuations),
	}
}

func (s *Store) replaceLocked(next *Store) {
	s.clients = next.clients
	s.usersByID = next.usersByID
	s.usersByLogin = next.usersByLogin
	s.usersBySubject = next.usersBySubject
	s.displayNameClaims = next.displayNameClaims
	s.credentialsByUser = next.credentialsByUser
	s.credentialsByLogin = next.credentialsByLogin
	s.accountSecurity = next.accountSecurity
	s.grants = next.grants
	s.codes = next.codes
	s.access = next.access
	s.refresh = next.refresh
	s.consents = next.consents
	s.sessions = next.sessions
	s.browserContexts = next.browserContexts
	s.rememberedSessions = next.rememberedSessions
	s.interactions = next.interactions
	s.deviceGrants = next.deviceGrants
	s.deviceByUserCode = next.deviceByUserCode
	s.durableInvitations = next.durableInvitations
	s.keys = next.keys
	s.continuations = next.continuations
}

func cloneDeviceGrantMap(source map[string]idpstore.DeviceGrant) map[string]idpstore.DeviceGrant {
	clone := make(map[string]idpstore.DeviceGrant, len(source))
	for key, value := range source {
		clone[key] = cloneDeviceGrant(value)
	}
	return clone
}

func cloneDurableInvitationMap(source map[string]idpstore.DurableInvitation) map[string]idpstore.DurableInvitation {
	clone := make(map[string]idpstore.DurableInvitation, len(source))
	for key, value := range source {
		clone[key] = cloneDurableInvitation(value)
	}
	return clone
}

func cloneDurableInvitation(value idpstore.DurableInvitation) idpstore.DurableInvitation {
	value.CodeHash = append([]byte(nil), value.CodeHash...)
	if value.RevokedAt != nil {
		at := *value.RevokedAt
		value.RevokedAt = &at
	}
	if value.RedeemedAt != nil {
		at := *value.RedeemedAt
		value.RedeemedAt = &at
	}
	return value
}

func cloneInteractionMap(source map[string]idpstore.InteractionRecord) map[string]idpstore.InteractionRecord {
	clone := make(map[string]idpstore.InteractionRecord, len(source))
	for key, value := range source {
		clone[key] = cloneInteraction(value)
	}
	return clone
}

func cloneBrowserContextMap(source map[string]idpstore.BrowserContext) map[string]idpstore.BrowserContext {
	clone := make(map[string]idpstore.BrowserContext, len(source))
	for key, value := range source {
		clone[key] = cloneBrowserContext(value)
	}
	return clone
}

func cloneRememberedBrowserSessionMap(source map[string]idpstore.RememberedBrowserSession) map[string]idpstore.RememberedBrowserSession {
	clone := make(map[string]idpstore.RememberedBrowserSession, len(source))
	for key, value := range source {
		clone[key] = cloneRememberedBrowserSession(value)
	}
	return clone
}

func cloneSession(session idpstore.Session) idpstore.Session {
	session.IDHash = append([]byte(nil), session.IDHash...)
	session.AMR = append([]string(nil), session.AMR...)
	if session.RevokedAt != nil {
		revokedAt := *session.RevokedAt
		session.RevokedAt = &revokedAt
	}
	return session
}

func cloneBrowserContext(context idpstore.BrowserContext) idpstore.BrowserContext {
	context.IDHash = append([]byte(nil), context.IDHash...)
	if context.RevokedAt != nil {
		revokedAt := *context.RevokedAt
		context.RevokedAt = &revokedAt
	}
	return context
}

func cloneRememberedBrowserSession(remembered idpstore.RememberedBrowserSession) idpstore.RememberedBrowserSession {
	remembered.IDHash = append([]byte(nil), remembered.IDHash...)
	remembered.ContextIDHash = append([]byte(nil), remembered.ContextIDHash...)
	remembered.SessionIDHash = append([]byte(nil), remembered.SessionIDHash...)
	if remembered.RemovedAt != nil {
		removedAt := *remembered.RemovedAt
		remembered.RemovedAt = &removedAt
	}
	return remembered
}

func cloneMap[K comparable, V any](source map[K]V) map[K]V {
	clone := make(map[K]V, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func EqualHash(a, b []byte) bool { return bytes.Equal(a, b) }

// Persistent reports whether the store survives process restarts. Memory never
// does, so production validation rejects it unless tests explicitly override.
func (s *Store) Persistent() bool { return false }
