package adminweb

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
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
				if err := visit(child, depth+1); err != nil {
					return err
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
		case nil, bool, float64:
		default:
			return fmt.Errorf("admin Widget IR contains unsupported value %T", value)
		}
		return nil
	}
	return visit(root, 0)
}
