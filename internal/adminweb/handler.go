package adminweb

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
)

type PageDataProvider interface {
	PageData(ctx context.Context, principal idpadmin.AdminPrincipal, pageID string, query url.Values) (map[string]any, error)
}

type HandlerConfig struct {
	Auth    *AuthManager
	Pages   PageDataProvider
	Widgets *WidgetRuntime
	SPA     http.Handler
	Assets  http.Handler
}

func NewHandler(config HandlerConfig) (http.Handler, error) {
	if config.Auth == nil || config.Pages == nil || config.Widgets == nil ||
		config.SPA == nil || config.Assets == nil {
		return nil, errors.New("admin auth, pages, widgets, SPA, and assets are required")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/auth/login", config.Auth.LoginHandler)
	mux.HandleFunc("GET /admin/auth/callback", config.Auth.CallbackHandler)
	mux.Handle("GET /api/admin/session", http.HandlerFunc(config.Auth.SessionHandler))
	mux.Handle("POST /api/admin/logout", config.Auth.RequireCSRF(http.HandlerFunc(config.Auth.LogoutHandler)))
	mux.Handle("GET /api/admin/overview", config.Auth.Authenticate(pageDataJSONHandler(config.Pages, "overview")))
	mux.Handle("GET /api/widget/pages/{page}", config.Auth.Authenticate(widgetPageHandler(config.Pages, config.Widgets)))
	mux.Handle("POST /api/widget/actions/execute", config.Auth.RequireCSRF(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSONError(writer, http.StatusNotImplemented, "mutations_not_enabled")
	})))
	mux.Handle("/static/admin/", http.StripPrefix("/static/admin/", config.Assets))
	mux.Handle("/admin", config.SPA)
	mux.Handle("/admin/", config.SPA)
	return securityHeaders(mux), nil
}

func pageDataJSONHandler(pages PageDataProvider, pageID string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, ok := Principal(request.Context())
		if !ok {
			writeJSONError(writer, http.StatusUnauthorized, "authentication_required")
			return
		}
		data, err := pages.PageData(request.Context(), principal, pageID, request.URL.Query())
		if err != nil {
			writeAdminError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, data)
	})
}

func widgetPageHandler(pages PageDataProvider, widgets *WidgetRuntime) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, ok := Principal(request.Context())
		if !ok {
			writeJSONError(writer, http.StatusUnauthorized, "authentication_required")
			return
		}
		pageID := request.PathValue("page")
		data, err := pages.PageData(request.Context(), principal, pageID, request.URL.Query())
		if err != nil {
			writeAdminError(writer, err)
			return
		}
		page, err := widgets.Render(request.Context(), data)
		if err != nil {
			writeJSONError(writer, http.StatusInternalServerError, "widget_render_failed")
			return
		}
		writeJSON(writer, http.StatusOK, page)
	})
}

func writeAdminError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, idpadminapp.ErrUnknownAdminPage):
		writeJSONError(writer, http.StatusNotFound, "page_not_found")
	case errors.Is(err, idpadmin.ErrCapabilityDenied),
		errors.Is(err, idpadmin.ErrGrantInactive),
		errors.Is(err, idpadmin.ErrGrantChanged):
		writeJSONError(writer, http.StatusForbidden, "administration_denied")
	default:
		writeJSONError(writer, http.StatusInternalServerError, "administration_unavailable")
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Security-Policy", strings.Join([]string{
			"default-src 'none'",
			"script-src 'self'",
			"style-src 'self'",
			"img-src 'self' data:",
			"font-src 'self'",
			"connect-src 'self'",
			"base-uri 'none'",
			"frame-ancestors 'none'",
			"form-action 'self'",
		}, "; "))
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(writer, request)
	})
}
