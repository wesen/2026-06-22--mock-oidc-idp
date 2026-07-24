package adminweb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedAdminFrontendAssets(t *testing.T) {
	tests := []struct {
		path        string
		handler     http.Handler
		contentType string
		contains    string
		cache       string
	}{
		{
			path:        "/admin/users",
			handler:     SPAHandler(),
			contentType: "text/html; charset=utf-8",
			contains:    "/static/admin/assets/admin.js",
			cache:       "no-store",
		},
		{
			path:        "/assets/admin.js",
			handler:     AssetsHandler(),
			contentType: "text/javascript; charset=utf-8",
			contains:    "TinyIDP Console",
			cache:       "public, max-age=31536000, immutable",
		},
		{
			path:        "/assets/admin.css",
			handler:     AssetsHandler(),
			contentType: "text/css; charset=utf-8",
			contains:    "--bs-body",
			cache:       "public, max-age=31536000, immutable",
		},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d", response.Code)
			}
			if got := response.Header().Get("Content-Type"); got != test.contentType {
				t.Fatalf("content type = %q", got)
			}
			if got := response.Header().Get("Cache-Control"); got != test.cache {
				t.Fatalf("cache control = %q", got)
			}
			if !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("response does not contain %q", test.contains)
			}
		})
	}
}

func TestEmbeddedAdminAssetsRejectUnknownFiles(t *testing.T) {
	response := httptest.NewRecorder()
	AssetsHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/unknown.js", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}
