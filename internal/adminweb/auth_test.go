package adminweb_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/tiny-idp/internal/adminweb"
	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type fakeOAuthFlow struct {
	config oauth2.Config
}

func (f *fakeOAuthFlow) AuthCodeURL(state string, options ...oauth2.AuthCodeOption) string {
	return f.config.AuthCodeURL(state, options...)
}

func (f *fakeOAuthFlow) Exchange(context.Context, string, ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return (&oauth2.Token{AccessToken: "access"}).WithExtra(map[string]any{"id_token": "raw-id-token"}), nil
}

type fakeIdentityVerifier struct {
	identity adminweb.VerifiedIdentity
}

func (f *fakeIdentityVerifier) Verify(context.Context, string) (adminweb.VerifiedIdentity, error) {
	return f.identity, nil
}

func TestOIDCPKCELoginCallbackSessionAndGrantInvalidation(t *testing.T) {
	ctx := context.Background()
	store := openAuthStore(t)
	now := time.Date(2026, 7, 24, 2, 0, 0, 0, time.UTC)
	grant := idpadmin.Grant{
		ID: "grant-1", ActorSubject: "owner-sub", Scope: idpadmin.SystemScope(), Role: "owner",
		Capabilities: idpadmin.AllCapabilities(), Version: 1, IssuedAt: now.Add(-time.Hour),
	}
	require.NoError(t, store.CreateAdminGrant(ctx, grant))
	oauth := &fakeOAuthFlow{config: oauth2.Config{
		ClientID:    "tinyidp-admin-console",
		Endpoint:    oauth2.Endpoint{AuthURL: "https://issuer.example/authorize"},
		RedirectURL: "https://issuer.example/admin/auth/callback",
	}}
	verifier := &fakeIdentityVerifier{}
	manager, err := adminweb.NewAuthManager(adminweb.AuthConfig{
		Store: store, OAuth: oauth, Verifier: verifier,
		SecretKey:    []byte("0123456789abcdef0123456789abcdef"),
		PublicOrigin: "https://issuer.example",
		Secure:       true, Now: func() time.Time { return now },
	})
	require.NoError(t, err)

	loginRequest := httptest.NewRequest(http.MethodGet, "https://issuer.example/admin/auth/login?return=/admin/users", nil)
	loginResponse := httptest.NewRecorder()
	manager.LoginHandler(loginResponse, loginRequest)
	require.Equal(t, http.StatusFound, loginResponse.Code)
	redirect, err := url.Parse(loginResponse.Header().Get("Location"))
	require.NoError(t, err)
	require.NotEmpty(t, redirect.Query().Get("state"))
	require.NotEmpty(t, redirect.Query().Get("nonce"))
	require.Equal(t, "S256", redirect.Query().Get("code_challenge_method"))
	require.NotEmpty(t, redirect.Query().Get("code_challenge"))
	verifier.identity = adminweb.VerifiedIdentity{
		Subject: "owner-sub", Nonce: redirect.Query().Get("nonce"), AuthTime: now,
	}
	binding := responseCookie(t, loginResponse.Result(), "tinyidp_admin_auth_binding")
	require.True(t, binding.HttpOnly)
	require.True(t, binding.Secure)

	callbackRequest := httptest.NewRequest(http.MethodGet,
		"https://issuer.example/admin/auth/callback?state="+url.QueryEscape(redirect.Query().Get("state"))+"&code=code-1", nil)
	callbackRequest.AddCookie(binding)
	callbackResponse := httptest.NewRecorder()
	manager.CallbackHandler(callbackResponse, callbackRequest)
	require.Equal(t, http.StatusFound, callbackResponse.Code)
	require.Equal(t, "/admin/users", callbackResponse.Header().Get("Location"))
	sessionCookie := responseCookie(t, callbackResponse.Result(), "tinyidp_admin_session")
	require.True(t, sessionCookie.HttpOnly)
	require.True(t, sessionCookie.Secure)

	sessionRequest := httptest.NewRequest(http.MethodGet, "https://issuer.example/api/admin/session", nil)
	sessionRequest.AddCookie(sessionCookie)
	sessionResponse := httptest.NewRecorder()
	manager.SessionHandler(sessionResponse, sessionRequest)
	require.Equal(t, http.StatusOK, sessionResponse.Code)
	require.Contains(t, sessionResponse.Body.String(), `"csrf_token"`)
	require.NotContains(t, sessionResponse.Body.String(), sessionCookie.Value)
	var sessionPayload map[string]any
	require.NoError(t, json.Unmarshal(sessionResponse.Body.Bytes(), &sessionPayload))
	csrfToken, ok := sessionPayload["csrf_token"].(string)
	require.True(t, ok)

	protected := manager.Authenticate(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, ok := adminweb.Principal(request.Context())
		require.True(t, ok)
		require.Equal(t, "owner-sub", principal.Subject)
		writer.WriteHeader(http.StatusNoContent)
	}))
	protectedRequest := httptest.NewRequest(http.MethodGet, "https://issuer.example/api/admin/overview", nil)
	protectedRequest.AddCookie(sessionCookie)
	protectedResponse := httptest.NewRecorder()
	protected.ServeHTTP(protectedResponse, protectedRequest)
	require.Equal(t, http.StatusNoContent, protectedResponse.Code)

	mutation := manager.RequireCSRF(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	mutationRequest := httptest.NewRequest(http.MethodPost, "https://issuer.example/api/widget/actions/execute", nil)
	mutationRequest.AddCookie(sessionCookie)
	mutationRequest.Header.Set("Origin", "https://issuer.example")
	mutationRequest.Header.Set("X-CSRF-Token", csrfToken)
	mutationResponse := httptest.NewRecorder()
	mutation.ServeHTTP(mutationResponse, mutationRequest)
	require.Equal(t, http.StatusNoContent, mutationResponse.Code)
	mutationRequest.Header.Set("Origin", "https://evil.example")
	mutationResponse = httptest.NewRecorder()
	mutation.ServeHTTP(mutationResponse, mutationRequest)
	require.Equal(t, http.StatusForbidden, mutationResponse.Code)

	reauth := manager.Authenticate(http.HandlerFunc(manager.ReauthHandler))
	reauthRequest := httptest.NewRequest(
		http.MethodGet,
		"https://issuer.example/admin/auth/reauth?return=/admin/users",
		nil,
	)
	reauthRequest.AddCookie(sessionCookie)
	reauthResponse := httptest.NewRecorder()
	reauth.ServeHTTP(reauthResponse, reauthRequest)
	require.Equal(t, http.StatusFound, reauthResponse.Code)
	reauthLocation, err := url.Parse(reauthResponse.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "login", reauthLocation.Query().Get("prompt"))
	require.Equal(t, "0", reauthLocation.Query().Get("max_age"))

	require.NoError(t, store.RevokeAdminGrant(ctx, grant.ID, grant.Version, now))
	deniedResponse := httptest.NewRecorder()
	protected.ServeHTTP(deniedResponse, protectedRequest)
	require.Equal(t, http.StatusUnauthorized, deniedResponse.Code)
}

func TestLoginRejectsExternalReturnPath(t *testing.T) {
	store := openAuthStore(t)
	manager, err := adminweb.NewAuthManager(adminweb.AuthConfig{
		Store:        store,
		OAuth:        &fakeOAuthFlow{config: oauth2.Config{Endpoint: oauth2.Endpoint{AuthURL: "https://issuer.example/authorize"}}},
		Verifier:     &fakeIdentityVerifier{},
		SecretKey:    []byte("0123456789abcdef0123456789abcdef"),
		PublicOrigin: "https://issuer.example",
	})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodGet,
		"https://issuer.example/admin/auth/login?return=https://evil.example/steal", nil)
	response := httptest.NewRecorder()
	manager.LoginHandler(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
}

func responseCookie(t *testing.T, response *http.Response, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found", name)
	return nil
}

func openAuthStore(t *testing.T) *sqlitestore.Store {
	t.Helper()
	store, err := sqlitestore.Open(context.Background(), sqlitestore.DefaultConfig(filepath.Join(t.TempDir(), "idp.db")))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}
