// Package adminweb owns the authenticated public administration HTTP boundary.
package adminweb

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
	pkgerrors "github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	sessionCookieName = "tinyidp_admin_session"
	bindingCookieName = "tinyidp_admin_auth_binding"
)

type OAuthFlow interface {
	AuthCodeURL(state string, options ...oauth2.AuthCodeOption) string
	Exchange(ctx context.Context, code string, options ...oauth2.AuthCodeOption) (*oauth2.Token, error)
}

type VerifiedIdentity struct {
	Subject  string
	Nonce    string
	AuthTime time.Time
}

type IdentityVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (VerifiedIdentity, error)
}

type AuthConfig struct {
	Store        idpadminstore.Store
	OAuth        OAuthFlow
	Verifier     IdentityVerifier
	SecretKey    []byte
	PublicOrigin string
	Secure       bool
	Now          func() time.Time
	Random       io.Reader
	SessionTTL   time.Duration
	AttemptTTL   time.Duration
	FreshFor     time.Duration
}

type AuthManager struct {
	store      idpadminstore.Store
	oauth      OAuthFlow
	verifier   IdentityVerifier
	key        []byte
	origin     string
	aead       cipher.AEAD
	secure     bool
	now        func() time.Time
	random     io.Reader
	sessionTTL time.Duration
	attemptTTL time.Duration
	freshFor   time.Duration
}

type authAttemptSecret struct {
	PKCEVerifier string `json:"pkce_verifier"`
	Nonce        string `json:"nonce"`
}

type principalContextKey struct{}

func NewAuthManager(config AuthConfig) (*AuthManager, error) {
	if config.Store == nil || config.OAuth == nil || config.Verifier == nil {
		return nil, errors.New("admin auth store, OAuth flow, and verifier are required")
	}
	if len(config.SecretKey) != 32 {
		return nil, errors.New("admin auth key must contain exactly 32 bytes")
	}
	origin, err := issuerOrigin(config.PublicOrigin)
	if err != nil || origin != strings.TrimSuffix(config.PublicOrigin, "/") {
		return nil, errors.New("admin public origin must be an exact scheme and host origin")
	}
	block, err := aes.NewCipher(config.SecretKey)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "construct admin auth cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "construct admin auth AEAD")
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.Random == nil {
		config.Random = rand.Reader
	}
	if config.SessionTTL <= 0 {
		config.SessionTTL = 8 * time.Hour
	}
	if config.AttemptTTL <= 0 {
		config.AttemptTTL = 5 * time.Minute
	}
	if config.FreshFor <= 0 {
		config.FreshFor = 5 * time.Minute
	}
	return &AuthManager{
		store: config.Store, oauth: config.OAuth, verifier: config.Verifier,
		key: append([]byte(nil), config.SecretKey...), origin: origin, aead: aead, secure: config.Secure,
		now: config.Now, random: config.Random, sessionTTL: config.SessionTTL,
		attemptTTL: config.AttemptTTL, freshFor: config.FreshFor,
	}, nil
}

func NewOIDCFlow(ctx context.Context, issuer, redirectURI string, client *http.Client) (OAuthFlow, IdentityVerifier, error) {
	providerCtx := ctx
	if client != nil {
		providerCtx = oidc.ClientContext(ctx, client)
	}
	provider, err := oidc.NewProvider(providerCtx, issuer)
	if err != nil {
		return nil, nil, pkgerrors.Wrap(err, "discover admin OIDC provider")
	}
	config := &oauth2.Config{
		ClientID: idpadminapp.AdminConsoleClientID, Endpoint: provider.Endpoint(),
		RedirectURL: redirectURI, Scopes: []string{oidc.ScopeOpenID, "profile", "email"},
	}
	return config, &oidcIdentityVerifier{verifier: provider.Verifier(&oidc.Config{
		ClientID: idpadminapp.AdminConsoleClientID,
	})}, nil
}

type oidcIdentityVerifier struct {
	verifier *oidc.IDTokenVerifier
}

func (v *oidcIdentityVerifier) Verify(ctx context.Context, rawIDToken string) (VerifiedIdentity, error) {
	token, err := v.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return VerifiedIdentity{}, err
	}
	var claims struct {
		Subject  string `json:"sub"`
		Nonce    string `json:"nonce"`
		AuthTime int64  `json:"auth_time"`
	}
	if err := token.Claims(&claims); err != nil {
		return VerifiedIdentity{}, err
	}
	return VerifiedIdentity{
		Subject: claims.Subject, Nonce: claims.Nonce,
		AuthTime: time.Unix(claims.AuthTime, 0).UTC(),
	}, nil
}

func (m *AuthManager) LoginHandler(writer http.ResponseWriter, request *http.Request) {
	returnPath, err := safeReturnPath(request.URL.Query().Get("return"))
	if err != nil {
		http.Error(writer, "invalid return path", http.StatusBadRequest)
		return
	}
	state, err := m.randomToken(32)
	if err != nil {
		http.Error(writer, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	nonce, err := m.randomToken(32)
	if err != nil {
		http.Error(writer, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	verifier, err := m.randomToken(48)
	if err != nil {
		http.Error(writer, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	binding, err := m.randomToken(32)
	if err != nil {
		http.Error(writer, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	sealed, err := m.seal(authAttemptSecret{PKCEVerifier: verifier, Nonce: nonce})
	if err != nil {
		http.Error(writer, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	now := m.now().UTC()
	err = m.store.CreateAdminAuthAttempt(request.Context(), idpadminstore.AuthAttempt{
		StateHash: m.hash("state", state), NonceHash: m.hash("nonce", nonce),
		PKCEVerifierBox: sealed, ReturnPath: returnPath,
		BrowserBindingHash: m.hash("binding", binding), CreatedAt: now, ExpiresAt: now.Add(m.attemptTTL),
	})
	if err != nil {
		http.Error(writer, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	http.SetCookie(writer, &http.Cookie{
		Name: bindingCookieName, Value: binding, Path: "/admin/auth", HttpOnly: true,
		Secure: m.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(m.attemptTTL.Seconds()),
	})
	challenge := base64.RawURLEncoding.EncodeToString(sha256Sum(verifier))
	http.Redirect(writer, request, m.oauth.AuthCodeURL(
		state, oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	), http.StatusFound)
}

func (m *AuthManager) CallbackHandler(writer http.ResponseWriter, request *http.Request) {
	if oauthError := request.URL.Query().Get("error"); oauthError != "" {
		http.Error(writer, "identity provider rejected authentication", http.StatusUnauthorized)
		return
	}
	state := request.URL.Query().Get("state")
	code := request.URL.Query().Get("code")
	bindingCookie, err := request.Cookie(bindingCookieName)
	if err != nil || state == "" || code == "" {
		http.Error(writer, "invalid authentication callback", http.StatusBadRequest)
		return
	}
	now := m.now().UTC()
	attempt, err := m.store.ConsumeAdminAuthAttempt(
		request.Context(), m.hash("state", state), m.hash("binding", bindingCookie.Value), now,
	)
	if err != nil {
		http.Error(writer, "authentication attempt is invalid or expired", http.StatusUnauthorized)
		return
	}
	secret, err := m.open(attempt.PKCEVerifierBox)
	if err != nil {
		http.Error(writer, "authentication attempt is invalid", http.StatusUnauthorized)
		return
	}
	token, err := m.oauth.Exchange(request.Context(), code, oauth2.SetAuthURLParam("code_verifier", secret.PKCEVerifier))
	if err != nil {
		http.Error(writer, "token exchange failed", http.StatusUnauthorized)
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		http.Error(writer, "identity token is missing", http.StatusUnauthorized)
		return
	}
	identity, err := m.verifier.Verify(request.Context(), rawIDToken)
	if err != nil || identity.Subject == "" ||
		subtle.ConstantTimeCompare(m.hash("nonce", identity.Nonce), attempt.NonceHash) != 1 {
		http.Error(writer, "identity token is invalid", http.StatusUnauthorized)
		return
	}
	grant, err := m.store.FindActiveAdminGrant(request.Context(), identity.Subject, idpadmin.SystemScope(), now)
	if err != nil {
		http.Error(writer, "administration access denied", http.StatusForbidden)
		return
	}
	sessionHandle, err := m.randomToken(32)
	if err != nil {
		http.Error(writer, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	if identity.AuthTime.IsZero() {
		identity.AuthTime = now
	}
	err = m.store.CreateAdminSession(request.Context(), idpadminstore.Session{
		IDHash: m.hash("session", sessionHandle), Subject: identity.Subject,
		GrantID: grant.ID, GrantVersion: grant.Version, CSRFHash: m.hash("csrf", ""),
		AuthenticatedAt: identity.AuthTime, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(m.sessionTTL),
	})
	if err != nil {
		http.Error(writer, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	http.SetCookie(writer, &http.Cookie{
		Name: sessionCookieName, Value: sessionHandle, Path: "/", HttpOnly: true,
		Secure: m.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(m.sessionTTL.Seconds()),
	})
	http.SetCookie(writer, &http.Cookie{
		Name: bindingCookieName, Path: "/admin/auth", HttpOnly: true,
		Secure: m.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	http.Redirect(writer, request, attempt.ReturnPath, http.StatusFound)
}

func (m *AuthManager) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, _, err := m.principal(request)
		if err != nil {
			writeJSONError(writer, http.StatusUnauthorized, "authentication_required")
			return
		}
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), principalContextKey{}, principal)))
	})
}

func (m *AuthManager) RequireCSRF(next http.Handler) http.Handler {
	return m.Authenticate(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Origin") != m.origin {
			writeJSONError(writer, http.StatusForbidden, "origin_rejected")
			return
		}
		_, session, err := m.principal(request)
		if err != nil {
			writeJSONError(writer, http.StatusUnauthorized, "authentication_required")
			return
		}
		token := request.Header.Get("X-CSRF-Token")
		if token == "" || subtle.ConstantTimeCompare(m.hash("csrf", token), session.CSRFHash) != 1 {
			writeJSONError(writer, http.StatusForbidden, "csrf_rejected")
			return
		}
		next.ServeHTTP(writer, request)
	}))
}

func (m *AuthManager) SessionHandler(writer http.ResponseWriter, request *http.Request) {
	principal, session, err := m.principal(request)
	if err != nil {
		writeJSONError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	csrf, err := m.randomToken(32)
	if err != nil {
		writeJSONError(writer, http.StatusInternalServerError, "session_unavailable")
		return
	}
	now := m.now().UTC()
	if err := m.store.RotateAdminSessionCSRF(request.Context(), session.IDHash, m.hash("csrf", csrf), now); err != nil {
		writeJSONError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"subject": principal.Subject, "grant_id": session.GrantID,
		"grant_version": session.GrantVersion, "csrf_token": csrf,
		"expires_at": session.ExpiresAt,
	})
}

func (m *AuthManager) LogoutHandler(writer http.ResponseWriter, request *http.Request) {
	_, session, err := m.principal(request)
	if err == nil {
		_ = m.store.RevokeAdminSession(request.Context(), session.IDHash, m.now().UTC())
	}
	http.SetCookie(writer, &http.Cookie{
		Name: sessionCookieName, Path: "/", HttpOnly: true, Secure: m.secure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	writer.WriteHeader(http.StatusNoContent)
}

func Principal(ctx context.Context) (idpadmin.AdminPrincipal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(idpadmin.AdminPrincipal)
	return principal, ok
}

func (m *AuthManager) principal(request *http.Request) (idpadmin.AdminPrincipal, idpadminstore.Session, error) {
	cookie, err := request.Cookie(sessionCookieName)
	if err != nil {
		return idpadmin.AdminPrincipal{}, idpadminstore.Session{}, err
	}
	session, err := m.store.GetAdminSession(request.Context(), m.hash("session", cookie.Value))
	if err != nil {
		return idpadmin.AdminPrincipal{}, idpadminstore.Session{}, err
	}
	now := m.now().UTC()
	if session.RevokedAt != nil || !now.Before(session.ExpiresAt) {
		return idpadmin.AdminPrincipal{}, idpadminstore.Session{}, errors.New("admin session is inactive")
	}
	grant, err := m.store.GetAdminGrant(request.Context(), session.GrantID)
	if err != nil || !grant.Active(now) || grant.Version != session.GrantVersion || grant.ActorSubject != session.Subject {
		return idpadmin.AdminPrincipal{}, idpadminstore.Session{}, errors.New("admin grant changed")
	}
	assurance := idpadmin.AssuranceAuthenticated
	if now.Sub(session.AuthenticatedAt) <= m.freshFor {
		assurance = idpadmin.AssuranceFresh
	}
	return idpadmin.AdminPrincipal{
		Subject: session.Subject, SessionID: cookie.Value,
		Authenticated: session.AuthenticatedAt, Assurance: assurance,
	}, session, nil
}

func (m *AuthManager) seal(value authAttemptSecret) ([]byte, error) {
	plaintext, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, m.aead.NonceSize())
	if _, err := io.ReadFull(m.random, nonce); err != nil {
		return nil, err
	}
	return m.aead.Seal(nonce, nonce, plaintext, []byte("tinyidp/admin-auth-attempt/v1")), nil
}

func (m *AuthManager) open(value []byte) (authAttemptSecret, error) {
	if len(value) < m.aead.NonceSize() {
		return authAttemptSecret{}, errors.New("auth attempt box is truncated")
	}
	nonce := value[:m.aead.NonceSize()]
	plaintext, err := m.aead.Open(nil, nonce, value[m.aead.NonceSize():], []byte("tinyidp/admin-auth-attempt/v1"))
	if err != nil {
		return authAttemptSecret{}, err
	}
	var secret authAttemptSecret
	err = json.Unmarshal(plaintext, &secret)
	return secret, err
}

func (m *AuthManager) hash(domain, value string) []byte {
	return idpstore.HashSecret(m.key, "tinyidp/admin/"+domain+"/v1\x00"+value)
}

func (m *AuthManager) randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(m.random, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func safeReturnPath(value string) (string, error) {
	if value == "" {
		return "/admin", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/admin") ||
		strings.HasPrefix(parsed.Path, "//") || parsed.Fragment != "" {
		return "", errors.New("unsafe admin return path")
	}
	return parsed.RequestURI(), nil
}

func sha256Sum(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

func writeJSONError(writer http.ResponseWriter, status int, code string) {
	writeJSON(writer, status, map[string]string{"error": code})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func issuerOrigin(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid issuer URL")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}
