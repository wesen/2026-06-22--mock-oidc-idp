package adminweb

import (
	"bytes"
	_ "embed"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed frontend_dist/index.html
var frontendIndex []byte

//go:embed frontend_dist/assets/admin.js
var frontendJavaScript []byte

//go:embed frontend_dist/assets/admin.css
var frontendCSS []byte

func SPAHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		http.ServeContent(writer, request, "index.html", time.Time{}, bytes.NewReader(frontendIndex))
	})
}

func AssetsHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+request.URL.Path), "/")
		var content []byte
		switch name {
		case "assets/admin.js":
			writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			content = frontendJavaScript
		case "assets/admin.css":
			writer.Header().Set("Content-Type", "text/css; charset=utf-8")
			content = frontendCSS
		default:
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeContent(writer, request, path.Base(name), time.Time{}, bytes.NewReader(content))
	})
}
