package adminweb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-go-golems/tiny-idp/internal/adminweb"
	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
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
