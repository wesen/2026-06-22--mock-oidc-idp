package adminweb

import (
	"context"
	"encoding/json"
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

func TestWidgetRuntimeProducesDeterministicIR(t *testing.T) {
	runtime, err := NewWidgetRuntime()
	require.NoError(t, err)
	data := map[string]any{
		"id": "operations", "title": "Operations", "message": "Health",
		"metrics": []map[string]string{{"label": "Pending audit", "value": "1"}},
		"rows": []map[string]string{
			{"id": "op-1", "primary": "backup", "secondary": "", "status": "failed"},
		},
	}
	first, err := runtime.Render(context.Background(), data)
	require.NoError(t, err)
	second, err := runtime.Render(context.Background(), data)
	require.NoError(t, err)
	firstJSON, err := json.Marshal(first)
	require.NoError(t, err)
	secondJSON, err := json.Marshal(second)
	require.NoError(t, err)
	require.Equal(t, firstJSON, secondJSON)
	require.Contains(t, string(firstJSON), `"op-1"`)
	require.Contains(t, string(firstJSON), `"backup"`)
}

func TestWidgetRuntimeRendersEveryScreenAndStateThroughProductionProvider(t *testing.T) {
	runtime, err := NewWidgetRuntime()
	require.NoError(t, err)
	pages := []string{
		"overview", "users", "invitations", "clients", "keys", "activity", "operations",
	}
	states := []string{
		"loading", "empty", "no-results", "forbidden", "stale",
		"session-expired", "audit-degraded", "operation-failed",
	}
	for _, pageID := range pages {
		for _, state := range states {
			t.Run(pageID+"/"+state, func(t *testing.T) {
				page, err := runtime.Render(context.Background(), map[string]any{
					"id": pageID, "title": pageID, "message": state,
					"metrics": []map[string]string{{"label": "State", "value": state}},
					"rows": []map[string]string{{
						"id": pageID + "-" + state, "primary": state,
						"secondary": pageID, "status": "safe",
					}},
				})
				require.NoError(t, err)
				encoded, err := json.Marshal(page)
				require.NoError(t, err)
				require.Contains(t, string(encoded), pageID+"-"+state)
			})
		}
	}
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

func TestWidgetIRValidatorRejectsUnknownComponentsAndSchema(t *testing.T) {
	require.Error(t, validateWidgetIR(map[string]any{
		"schemaVersion": "9.9.9", "id": "x", "title": "x",
		"root": map[string]any{"kind": "component", "type": "Stack"},
	}))
	require.Error(t, validateWidgetIR(map[string]any{
		"schemaVersion": "0.1.0", "id": "x", "title": "x",
		"root": map[string]any{"kind": "component", "type": "RemoteScript"},
	}))
}

func TestWidgetIRValidatorRejectsAuthoritySecretsCodeAndExternalURLs(t *testing.T) {
	base := func(props map[string]any) map[string]any {
		return map[string]any{
			"schemaVersion": "0.1.0", "id": "x", "title": "x",
			"root": map[string]any{
				"kind": "component", "type": "Panel", "props": props,
			},
		}
	}
	for name, props := range map[string]map[string]any{
		"command":       {"command": "users.disable"},
		"capability":    {"capability": "users.disable"},
		"sql":           {"sql": "SELECT * FROM users"},
		"secret hash":   {"secret_hash": "stored-value"},
		"private key":   {"value": "-----BEGIN PRIVATE KEY-----"},
		"inline script": {"value": "<script>alert(1)</script>"},
		"external URL":  {"href": "https://evil.example/collect"},
	} {
		t.Run(name, func(t *testing.T) {
			require.Error(t, validateWidgetIR(base(props)))
		})
	}
}
