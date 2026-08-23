package idpscript_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-go-golems/tiny-idp/pkg/idpprogram"
	"github.com/go-go-golems/tiny-idp/pkg/idpscript"
)

func TestPhase0ExampleCompiles(t *testing.T) {
	source, err := os.ReadFile("../../examples/tinyidp-script/phase0.js")
	require.NoError(t, err)
	options := idpscript.DefaultCompileOptions()
	options.Schemas = map[string]idpprogram.Schema{
		"signupStartInput": {
			ID:       "signupStartInput",
			Kind:     idpprogram.SchemaKindObject,
			MaxBytes: 4096,
			Fields: map[string]idpprogram.SchemaField{
				"email": {Ref: "email", Required: true},
			},
		},
		"email": {
			ID:        "email",
			Kind:      idpprogram.SchemaKindString,
			MaxBytes:  320,
			MaxLength: 320,
		},
		"signupResult": {
			ID:       "signupResult",
			Kind:     idpprogram.SchemaKindObject,
			MaxBytes: 4096,
		},
	}
	_, err = idpscript.Compile(context.Background(), string(source), options)
	require.NoError(t, err)
}
