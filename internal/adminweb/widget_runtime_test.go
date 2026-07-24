package adminweb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWidgetRuntimeRendersV3IRThroughSafeAdminModule(t *testing.T) {
	runtime, err := NewWidgetRuntime()
	require.NoError(t, err)
	page, err := runtime.Render(context.Background(), map[string]any{
		"id": "users", "title": "Users", "message": "Read-only phase",
		"metrics": []map[string]string{{"label": "Users", "value": "2"}},
		"rows": []map[string]string{
			{"id": "u1", "primary": "Alice", "secondary": "alice@example.com", "status": "active"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "users", page["id"])
	require.Equal(t, "Users", page["title"])
	require.NotNil(t, page["root"])
}

func TestWidgetRuntimeDoesNotRegisterHostModules(t *testing.T) {
	vm, registry, err := newWidgetVM(map[string]any{})
	require.NoError(t, err)
	registry.Enable(vm)
	_, err = vm.RunString(`require("fs")`)
	require.Error(t, err)
	_, err = vm.RunString(`require("db")`)
	require.Error(t, err)
	_, err = vm.RunString(`require("http")`)
	require.Error(t, err)
	value, err := vm.RunString(`require("tinyidp.admin").pageData()`)
	require.NoError(t, err)
	require.NotNil(t, value.Export())
}

func TestWidgetIRValidatorRejectsExcessiveDepth(t *testing.T) {
	var value any = "leaf"
	for range maxWidgetIRDepth + 1 {
		value = map[string]any{"child": value}
	}
	require.Error(t, validateWidgetIR(value))
}
