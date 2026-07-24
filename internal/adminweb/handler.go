package adminweb

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

type PageDataProvider interface {
	PageData(ctx context.Context, principal idpadmin.AdminPrincipal, pageID string, query url.Values) (map[string]any, error)
}

type ActionPreparer interface {
	Prepare(context.Context, idpadmin.AdminPrincipal, idpadminapp.PrepareActionRequest) (idpadminapp.PreparedAction, error)
}

type UserActionExecutor interface {
	Execute(context.Context, idpadminapp.ExecutionRequest, []byte) ([]byte, error)
}

type HandlerConfig struct {
	Auth    *AuthManager
	Pages   PageDataProvider
	Widgets *WidgetRuntime
	Actions ActionPreparer
	Users   UserActionExecutor
	SPA     http.Handler
	Assets  http.Handler
}

func NewHandler(config HandlerConfig) (http.Handler, error) {
	if config.Auth == nil || config.Pages == nil || config.Widgets == nil ||
		config.Actions == nil || config.Users == nil ||
		config.SPA == nil || config.Assets == nil {
		return nil, errors.New("admin auth, pages, widgets, actions, users, SPA, and assets are required")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/auth/login", config.Auth.LoginHandler)
	mux.Handle("GET /admin/auth/reauth", config.Auth.Authenticate(http.HandlerFunc(config.Auth.ReauthHandler)))
	mux.HandleFunc("GET /admin/auth/callback", config.Auth.CallbackHandler)
	mux.Handle("GET /api/admin/session", http.HandlerFunc(config.Auth.SessionHandler))
	mux.Handle("POST /api/admin/logout", config.Auth.RequireCSRF(http.HandlerFunc(config.Auth.LogoutHandler)))
	mux.Handle("GET /api/admin/overview", config.Auth.Authenticate(pageDataJSONHandler(config.Pages, "overview")))
	mux.Handle("GET /api/admin/pages/{page}", config.Auth.Authenticate(pageDataPathHandler(config.Pages)))
	mux.Handle("GET /api/widget/pages/{page}", config.Auth.Authenticate(widgetPageHandler(config.Pages, config.Widgets)))
	mux.Handle("POST /api/widget/actions/prepare", config.Auth.RequireCSRF(prepareActionHandler(config.Actions)))
	mux.Handle("POST /api/widget/actions/execute", config.Auth.RequireCSRF(executeActionHandler(config.Users)))
	mux.Handle("/static/admin/", http.StripPrefix("/static/admin/", config.Assets))
	mux.Handle("/admin", config.SPA)
	mux.Handle("/admin/", config.SPA)
	return securityHeaders(mux), nil
}

func pageDataPathHandler(pages PageDataProvider) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, ok := Principal(request.Context())
		if !ok {
			writeJSONError(writer, http.StatusUnauthorized, "authentication_required")
			return
		}
		data, err := pages.PageData(
			request.Context(), principal, request.PathValue("page"), request.URL.Query(),
		)
		if err != nil {
			writeAdminError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, data)
	})
}

func prepareActionHandler(actions ActionPreparer) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, ok := Principal(request.Context())
		if !ok {
			writeJSONError(writer, http.StatusUnauthorized, "authentication_required")
			return
		}
		var input idpadminapp.PrepareActionRequest
		if err := decodeBoundedJSON(writer, request, &input, 8<<10); err != nil {
			writeJSONError(writer, http.StatusBadRequest, "invalid_request")
			return
		}
		prepared, err := actions.Prepare(request.Context(), principal, input)
		if err != nil {
			writeActionError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, prepared)
	})
}

func executeActionHandler(users UserActionExecutor) http.Handler {
	type envelope struct {
		Payload struct {
			ActionHandle string          `json:"actionHandle"`
			Input        json.RawMessage `json:"input"`
		} `json:"payload"`
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, ok := Principal(request.Context())
		if !ok {
			writeJSONError(writer, http.StatusUnauthorized, "authentication_required")
			return
		}
		var input envelope
		if err := decodeBoundedJSON(writer, request, &input, 64<<10); err != nil {
			writeJSONError(writer, http.StatusBadRequest, "invalid_request")
			return
		}
		rawInput := append([]byte(nil), input.Payload.Input...)
		defer clearSensitiveBytes(rawInput)
		hash := sha256.Sum256(append([]byte(input.Payload.ActionHandle+"\x00"), rawInput...))
		response, err := users.Execute(request.Context(), idpadminapp.ExecutionRequest{
			Handle: input.Payload.ActionHandle, Principal: principal,
			RequestID:      request.Header.Get("X-Request-ID"),
			IdempotencyKey: request.Header.Get("Idempotency-Key"),
			RequestHash:    hash[:],
		}, rawInput)
		if err != nil {
			writeActionError(writer, err)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(response)
	})
}

func decodeBoundedJSON(writer http.ResponseWriter, request *http.Request, target any, limit int64) error {
	request.Body = http.MaxBytesReader(writer, request.Body, limit)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func writeActionError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, idpadmin.ErrFreshAuthRequired):
		writeJSONError(writer, http.StatusUnauthorized, "fresh_auth_required")
	case errors.Is(err, idpadmin.ErrCapabilityDenied),
		errors.Is(err, idpadmin.ErrGrantInactive),
		errors.Is(err, idpadmin.ErrGrantChanged):
		writeJSONError(writer, http.StatusForbidden, "capability_denied")
	case errors.Is(err, idpadmin.ErrExpiredAction),
		errors.Is(err, idpadminstore.ErrNonceConsumed):
		writeJSONError(writer, http.StatusGone, "action_expired")
	case errors.Is(err, idpadminstore.ErrVersionConflict),
		errors.Is(err, idpadminstore.ErrIdempotencyConflict):
		writeJSONError(writer, http.StatusConflict, actionConflictCode(err))
	case errors.Is(err, idpadminstore.ErrNotFound),
		errors.Is(err, idpstore.ErrNotFound):
		writeJSONError(writer, http.StatusNotFound, "resource_not_found")
	case errors.Is(err, idpadminapp.ErrReasonRequired),
		errors.Is(err, idpadminapp.ErrConfirmationFailed):
		writeJSONError(writer, http.StatusUnprocessableEntity, "validation_failed")
	default:
		writeJSONError(writer, http.StatusBadRequest, "invalid_request")
	}
}

func actionConflictCode(err error) string {
	if errors.Is(err, idpadminstore.ErrIdempotencyConflict) {
		return "idempotency_conflict"
	}
	return "version_conflict"
}

func clearSensitiveBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
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
