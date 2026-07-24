package adminweb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/tiny-idp/internal/adminweb"
	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type staticPageProvider struct{}

func (staticPageProvider) PageData(
	context.Context,
	idpadmin.AdminPrincipal,
	string,
	url.Values,
) (map[string]any, error) {
	return map[string]any{"id": "overview", "title": "Overview"}, nil
}

func (staticPageProvider) ClientDetail(
	context.Context,
	idpadmin.AdminPrincipal,
	string,
) (idpadmin.ClientDetail, error) {
	return idpadmin.ClientDetail{}, nil
}

type staticActionPreparer struct{}

func (staticActionPreparer) Prepare(
	context.Context,
	idpadmin.AdminPrincipal,
	idpadminapp.PrepareActionRequest,
) (idpadminapp.PreparedAction, error) {
	return idpadminapp.PreparedAction{Handle: "handle"}, nil
}

type staticUserExecutor struct{}

func (staticUserExecutor) Execute(
	context.Context,
	idpadminapp.ExecutionRequest,
	[]byte,
) ([]byte, error) {
	return []byte(`{"ok":true}`), nil
}

type staticDownloadProvider struct{}

func (staticDownloadProvider) Issue(
	context.Context,
	idpadmin.AdminPrincipal,
	string,
) (idpadminapp.DownloadGrant, error) {
	return idpadminapp.DownloadGrant{}, nil
}

func (staticDownloadProvider) Consume(
	context.Context,
	idpadmin.AdminPrincipal,
	string,
) (idpadminapp.ConsumedDownload, error) {
	return idpadminapp.ConsumedDownload{}, nil
}

func TestHandlerMountsPublicSurfaceWithSecurityHeaders(t *testing.T) {
	store := openAuthStore(t)
	auth, err := adminweb.NewAuthManager(adminweb.AuthConfig{
		Store: store, OAuth: &fakeOAuthFlow{
			config: oauth2.Config{Endpoint: oauth2.Endpoint{AuthURL: "https://issuer.example/authorize"}},
		},
		Verifier: &fakeIdentityVerifier{}, SecretKey: []byte("0123456789abcdef0123456789abcdef"),
		PublicOrigin: "https://issuer.example", Secure: true,
	})
	require.NoError(t, err)
	widgets, err := adminweb.NewWidgetRuntime()
	require.NoError(t, err)
	handler, err := adminweb.NewHandler(adminweb.HandlerConfig{
		Auth: auth, Pages: staticPageProvider{}, Widgets: widgets,
		Actions: staticActionPreparer{}, Commands: staticUserExecutor{},
		Downloads: staticDownloadProvider{},
		SPA: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte("spa"))
		}),
		Assets: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte("asset"))
		}),
	})
	require.NoError(t, err)

	for path, expectedBody := range map[string]string{
		"/admin": "spa", "/admin/users": "spa", "/static/admin/app.js": "asset",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://issuer.example"+path, nil))
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, expectedBody, response.Body.String())
		require.Contains(t, response.Header().Get("Content-Security-Policy"), "script-src 'self'")
		require.Equal(t, "DENY", response.Header().Get("X-Frame-Options"))
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://issuer.example/api/widget/pages/overview", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), "authentication_required")
}

type recordingActionPreparer struct {
	request   idpadminapp.PrepareActionRequest
	principal idpadmin.AdminPrincipal
}

func (r *recordingActionPreparer) Prepare(
	_ context.Context,
	principal idpadmin.AdminPrincipal,
	request idpadminapp.PrepareActionRequest,
) (idpadminapp.PreparedAction, error) {
	r.principal = principal
	r.request = request
	return idpadminapp.PreparedAction{
		Handle: "signed-handle", Command: request.Command, TargetID: request.TargetID,
	}, nil
}

type recordingUserExecutor struct {
	request idpadminapp.ExecutionRequest
	input   string
	err     error
}

func (r *recordingUserExecutor) Execute(
	_ context.Context,
	request idpadminapp.ExecutionRequest,
	input []byte,
) ([]byte, error) {
	r.request = request
	r.input = string(input)
	if r.err != nil {
		return nil, r.err
	}
	return []byte(`{"committed":true,"audit_status":"pending"}`), nil
}

func TestHandlerGuardsPrepareAndExecuteWithSessionOriginAndCSRF(t *testing.T) {
	ctx := context.Background()
	store := openAuthStore(t)
	now := time.Date(2026, 7, 24, 15, 0, 0, 0, time.UTC)
	key := []byte("0123456789abcdef0123456789abcdef")
	grant := idpadmin.Grant{
		ID: "owner-grant", ActorSubject: "owner-sub", Scope: idpadmin.SystemScope(),
		Role: "owner", Capabilities: idpadmin.AllCapabilities(), Version: 1,
		IssuedAt: now.Add(-time.Hour),
	}
	require.NoError(t, store.CreateAdminGrant(ctx, grant))
	sessionRaw := "raw-browser-session"
	csrfRaw := "raw-csrf-token"
	hash := func(domain, value string) []byte {
		return idpstore.HashSecret(key, "tinyidp/admin/"+domain+"/v1\x00"+value)
	}
	require.NoError(t, store.CreateAdminSession(ctx, idpadminstore.Session{
		IDHash: hash("session", sessionRaw), Subject: grant.ActorSubject,
		GrantID: grant.ID, GrantVersion: grant.Version, CSRFHash: hash("csrf", csrfRaw),
		AuthenticatedAt: now, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour),
	}))
	auth, err := adminweb.NewAuthManager(adminweb.AuthConfig{
		Store: store, OAuth: &fakeOAuthFlow{
			config: oauth2.Config{Endpoint: oauth2.Endpoint{AuthURL: "https://issuer.example/authorize"}},
		},
		Verifier: &fakeIdentityVerifier{}, SecretKey: key,
		PublicOrigin: "https://issuer.example", Secure: true, Now: func() time.Time { return now },
	})
	require.NoError(t, err)
	widgets, err := adminweb.NewWidgetRuntime()
	require.NoError(t, err)
	actions := &recordingActionPreparer{}
	users := &recordingUserExecutor{}
	handler, err := adminweb.NewHandler(adminweb.HandlerConfig{
		Auth: auth, Pages: staticPageProvider{}, Widgets: widgets,
		Actions: actions, Commands: users,
		Downloads: staticDownloadProvider{},
		SPA:       http.NotFoundHandler(), Assets: http.NotFoundHandler(),
	})
	require.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPost,
		"https://issuer.example/api/widget/actions/prepare",
		strings.NewReader(`{"command":"users.disable","target_id":"user-1"}`),
	)
	request.AddCookie(&http.Cookie{Name: "tinyidp_admin_session", Value: sessionRaw})
	request.Header.Set("Origin", "https://issuer.example")
	request.Header.Set("X-CSRF-Token", csrfRaw)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, idpadminapp.CommandUsersDisable, actions.request.Command)
	require.Equal(t, grant.ActorSubject, actions.principal.Subject)
	require.NotEqual(t, sessionRaw, actions.principal.SessionID)

	request = httptest.NewRequest(
		http.MethodPost,
		"https://issuer.example/api/widget/actions/execute",
		strings.NewReader(`{"payload":{"actionHandle":"signed-handle","input":{"reason":"review","confirmation":"DISABLE"}}}`),
	)
	request.AddCookie(&http.Cookie{Name: "tinyidp_admin_session", Value: sessionRaw})
	request.Header.Set("Origin", "https://issuer.example")
	request.Header.Set("X-CSRF-Token", csrfRaw)
	request.Header.Set("Idempotency-Key", "idem-1")
	request.Header.Set("X-Request-ID", "request-1")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "signed-handle", users.request.Handle)
	require.Equal(t, "idem-1", users.request.IdempotencyKey)
	require.JSONEq(t, `{"reason":"review","confirmation":"DISABLE"}`, users.input)
	require.Contains(t, response.Body.String(), `"audit_status":"pending"`)

	request = httptest.NewRequest(
		http.MethodPost,
		"https://issuer.example/api/widget/actions/prepare",
		strings.NewReader(`{"command":"users.disable","target_id":"user-1"}`),
	)
	request.AddCookie(&http.Cookie{Name: "tinyidp_admin_session", Value: sessionRaw})
	request.Header.Set("Origin", "https://evil.example")
	request.Header.Set("X-CSRF-Token", csrfRaw)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusForbidden, response.Code)

	users.err = &idpadminapp.ValidationError{
		FieldErrors: map[string]string{"redirect_uris": "redirect URI must not contain wildcards"},
		Cause:       idpstore.ErrWildcardRedirectURI,
	}
	request = httptest.NewRequest(
		http.MethodPost,
		"https://issuer.example/api/widget/actions/execute",
		strings.NewReader(`{"payload":{"actionHandle":"signed-handle","input":{"redirect_uris":["https://*.example.test"]}}}`),
	)
	request.AddCookie(&http.Cookie{Name: "tinyidp_admin_session", Value: sessionRaw})
	request.Header.Set("Origin", "https://issuer.example")
	request.Header.Set("X-CSRF-Token", csrfRaw)
	request.Header.Set("Idempotency-Key", "idem-validation")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnprocessableEntity, response.Code)
	require.JSONEq(t, `{
		"error":"validation_failed",
		"field_errors":{"redirect_uris":"redirect URI must not contain wildcards"}
	}`, response.Body.String())
}
