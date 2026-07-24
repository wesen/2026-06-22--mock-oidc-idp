package adminweb

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/go-go-golems/rag-evaluation-system/pkg/widgetdsl"
	pkgerrors "github.com/pkg/errors"
)

//go:embed verbs/pages.js
var adminPagesSource string

const (
	maxWidgetIRBytes  = 1 << 20
	maxWidgetIRNodes  = 2_000
	maxWidgetIRDepth  = 64
	maxWidgetString   = 32 << 10
	widgetRenderLimit = 250 * time.Millisecond
)

type WidgetRuntime struct {
	source string
}

func NewWidgetRuntime() (*WidgetRuntime, error) {
	if adminPagesSource == "" {
		return nil, errors.New("embedded admin Widget DSL source is empty")
	}
	return &WidgetRuntime{source: adminPagesSource}, nil
}

func (r *WidgetRuntime) Render(ctx context.Context, pageData any) (map[string]any, error) {
	vm, registry, err := newWidgetVM(pageData)
	if err != nil {
		return nil, err
	}
	registry.Enable(vm)
	renderCtx, cancel := context.WithTimeout(ctx, widgetRenderLimit)
	defer cancel()
	done := make(chan struct{})
	go func() {
		select {
		case <-renderCtx.Done():
			vm.Interrupt(renderCtx.Err())
		case <-done:
		}
	}()
	value, err := vm.RunString(r.source)
	close(done)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "render admin Widget DSL page")
	}
	encoded, err := json.Marshal(value.Export())
	if err != nil {
		return nil, pkgerrors.Wrap(err, "encode admin Widget IR")
	}
	if len(encoded) > maxWidgetIRBytes {
		return nil, errors.New("admin Widget IR exceeds byte limit")
	}
	var page map[string]any
	if err := json.Unmarshal(encoded, &page); err != nil {
		return nil, pkgerrors.Wrap(err, "normalize admin Widget IR")
	}
	if err := validateWidgetIR(page); err != nil {
		return nil, err
	}
	return page, nil
}

func newWidgetVM(pageData any) (*goja.Runtime, *require.Registry, error) {
	vm := goja.New()
	registry := require.NewRegistry()
	registry.RegisterNativeModule(widgetdsl.WidgetV3ModuleName, widgetdsl.NewLoader(widgetdsl.WidgetV3ModuleName))
	registry.RegisterNativeModule("tinyidp.admin", func(runtime *goja.Runtime, module *goja.Object) {
		exports := module.Get("exports").ToObject(runtime)
		_ = exports.Set("pageData", func() any { return pageData })
	})
	return vm, registry, nil
}

func validateWidgetIR(root any) error {
	page, ok := root.(map[string]any)
	if !ok {
		return errors.New("admin Widget IR root must be an object")
	}
	if page["schemaVersion"] != "0.1.0" {
		return errors.New("admin Widget IR schema version is unsupported")
	}
	for _, field := range []string{"id", "title"} {
		value, ok := page[field].(string)
		if !ok || value == "" || len(value) > maxWidgetString {
			return fmt.Errorf("admin Widget IR %s is invalid", field)
		}
	}
	if _, ok := page["root"].(map[string]any); !ok {
		return errors.New("admin Widget IR component root is missing")
	}
	nodes := 0
	var visit func(any, int) error
	visit = func(value any, depth int) error {
		if depth > maxWidgetIRDepth {
			return errors.New("admin Widget IR exceeds depth limit")
		}
		switch typed := value.(type) {
		case map[string]any:
			nodes++
			if nodes > maxWidgetIRNodes {
				return errors.New("admin Widget IR exceeds node limit")
			}
			for key, child := range typed {
				if len(key) > maxWidgetString {
					return errors.New("admin Widget IR key exceeds string limit")
				}
				if unsafeAdminWidgetKey(key) {
					return fmt.Errorf("admin Widget IR contains forbidden property %q", key)
				}
				if err := visit(child, depth+1); err != nil {
					return err
				}
			}
			_, hasType := typed["type"]
			_, hasText := typed["text"]
			_, hasChildren := typed["children"]
			if kind, exists := typed["kind"]; exists && (hasType || hasText || hasChildren) {
				kindValue, ok := kind.(string)
				if !ok || kindValue != "component" && kindValue != "text" {
					return errors.New("admin Widget IR contains unsupported node kind")
				}
				if kindValue == "component" {
					component, ok := typed["type"].(string)
					if !ok || !allowedAdminWidgetComponent(component) {
						return fmt.Errorf("admin Widget IR contains unsupported component %q", component)
					}
				}
			}
		case []any:
			for _, child := range typed {
				if err := visit(child, depth+1); err != nil {
					return err
				}
			}
		case string:
			if len(typed) > maxWidgetString {
				return errors.New("admin Widget IR value exceeds string limit")
			}
			if unsafeAdminWidgetString(typed) {
				return errors.New("admin Widget IR contains forbidden content")
			}
		case nil, bool, float64:
		default:
			return fmt.Errorf("admin Widget IR contains unsupported value %T", value)
		}
		return nil
	}
	return visit(root, 0)
}

func unsafeAdminWidgetKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(
		strings.ToLower(strings.TrimSpace(key)),
	)
	switch normalized {
	case "command", "capability", "scope", "sql", "script", "html",
		"password", "passwordhash", "secrethash", "storedhash",
		"privatekey", "privatekeybytes", "cookie", "csrftoken":
		return true
	default:
		return false
	}
}

func unsafeAdminWidgetString(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(normalized, "http://") ||
		strings.HasPrefix(normalized, "https://") ||
		strings.HasPrefix(normalized, "javascript:") ||
		strings.Contains(normalized, "<script") ||
		strings.Contains(normalized, "-----begin private key-----")
}

func allowedAdminWidgetComponent(component string) bool {
	switch component {
	case "Stack", "SectionBlock", "KeyValueStrip", "Panel", "DataTable":
		return true
	default:
		return false
	}
}
