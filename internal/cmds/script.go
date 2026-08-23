package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/spf13/cobra"

	"github.com/go-go-golems/tiny-idp/pkg/idpprogram"
	"github.com/go-go-golems/tiny-idp/pkg/idpscript"
	"github.com/go-go-golems/tiny-idp/pkg/idpsignup"
)

// ScriptSettings is deliberately narrow for the first operational profile.
// The `signup` profile compiles exactly the host-owned schema catalog used by
// the production signup executor; arbitrary script profiles are not silently
// treated as production-ready.
type ScriptSettings struct {
	Source  string `glazed:"source"`
	Profile string `glazed:"profile"`
}

type ScriptValidateCommand struct{ *cmds.CommandDescription }
type ScriptExplainCommand struct{ *cmds.CommandDescription }
type ScriptTestCommand struct{ *cmds.CommandDescription }

func NewScriptCommand() (*cobra.Command, error) {
	root := &cobra.Command{Use: "script", Short: "Validate and explain bounded Tiny-IDP JavaScript programs"}
	validate, err := NewScriptValidateCommand()
	if err != nil {
		return nil, err
	}
	validateCobra, err := cli.BuildCobraCommand(validate)
	if err != nil {
		return nil, err
	}
	explain, err := NewScriptExplainCommand()
	if err != nil {
		return nil, err
	}
	explainCobra, err := cli.BuildCobraCommand(explain)
	if err != nil {
		return nil, err
	}
	testCommand, err := NewScriptTestCommand()
	if err != nil {
		return nil, err
	}
	testCobra, err := cli.BuildCobraCommand(testCommand)
	if err != nil {
		return nil, err
	}
	root.AddCommand(validateCobra, explainCobra, testCobra)
	return root, nil
}

func newScriptSections() (schema.Section, schema.Section, error) {
	glazedSection, err := settings.NewStructuredOutputSection()
	if err != nil {
		return nil, nil, fmt.Errorf("build output settings: %w", err)
	}
	commandSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, nil, fmt.Errorf("build command settings: %w", err)
	}
	return glazedSection, commandSection, nil
}

func NewScriptValidateCommand() (*ScriptValidateCommand, error) {
	glazedSection, commandSection, err := newScriptSections()
	if err != nil {
		return nil, err
	}
	description := cmds.NewCommandDescription("validate",
		cmds.WithShort("Compile and validate a Tiny-IDP JavaScript program"), cmds.WithLong(`Compile a Tiny-IDP JavaScript source file without starting an HTTP server.

Validation materializes the program in an isolated runtime, applies the host-owned
signup schema profile, and prints stable source, program, callback-registry, and
schema fingerprints. A nonzero exit reports compilation or contract diagnostics.

Example:
  tinyidp script validate --source ./signup.js --output json`),
		cmds.WithFlags(
			fields.New("source", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Path to the JavaScript program source")),
			fields.New("profile", fields.TypeString, fields.WithDefault("signup"), fields.WithHelp("Host-owned production schema profile (currently: signup)")),
		),
		cmds.WithSections(glazedSection, commandSection),
	)
	return &ScriptValidateCommand{CommandDescription: description}, nil
}

func NewScriptExplainCommand() (*ScriptExplainCommand, error) {
	glazedSection, commandSection, err := newScriptSections()
	if err != nil {
		return nil, err
	}
	description := cmds.NewCommandDescription("explain",
		cmds.WithShort("Explain workflows and native capabilities in a Tiny-IDP program"), cmds.WithLong(`Compile and explain a Tiny-IDP JavaScript source file without executing browser requests.

The output is a stable, secret-free projection of workflows, handlers, schemas,
effects, continuation edges, budgets, and provider contracts.

Example:
  tinyidp script explain --source ./signup.js --output json`),
		cmds.WithFlags(
			fields.New("source", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Path to the JavaScript program source")),
			fields.New("profile", fields.TypeString, fields.WithDefault("signup"), fields.WithHelp("Host-owned production schema profile (currently: signup)")),
		),
		cmds.WithSections(glazedSection, commandSection),
	)
	return &ScriptExplainCommand{CommandDescription: description}, nil
}

func NewScriptTestCommand() (*ScriptTestCommand, error) {
	glazedSection, commandSection, err := newScriptSections()
	if err != nil {
		return nil, err
	}
	description := cmds.NewCommandDescription("test",
		cmds.WithShort("Run declarative embedded Tiny-IDP program tests"), cmds.WithLong(`Compile a Tiny-IDP JavaScript source file and run its declarative embedded tests.

Each test names a registered lambda, supplies bounded JSON input, and expects a
declared outcome kind. Tests receive no ambient capabilities or secrets; a
capability-dependent test fails unless a future explicit deterministic fake is
registered by the host.

Example:
  tinyidp script test --source ./signup.js --output json`),
		cmds.WithFlags(
			fields.New("source", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Path to the JavaScript program source")),
			fields.New("profile", fields.TypeString, fields.WithDefault("signup"), fields.WithHelp("Host-owned production schema profile (currently: signup)")),
		),
		cmds.WithSections(glazedSection, commandSection),
	)
	return &ScriptTestCommand{CommandDescription: description}, nil
}

func (c *ScriptValidateCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	settings, artifact, err := loadScriptArtifact(ctx, vals)
	if err != nil {
		return err
	}
	fingerprints := artifact.Fingerprints()
	return gp.AddRow(ctx, types.NewRow(
		types.MRP("status", "valid"),
		types.MRP("source", settings.Source),
		types.MRP("profile", settings.Profile),
		types.MRP("source_fingerprint", fingerprints.Source),
		types.MRP("program_fingerprint", fingerprints.Program),
		types.MRP("callback_registry_fingerprint", fingerprints.CallbackRegistry),
		types.MRP("schema_fingerprint", fingerprints.Schemas),
	))
}

func (c *ScriptExplainCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	settings, artifact, err := loadScriptArtifact(ctx, vals)
	if err != nil {
		return err
	}
	program := artifact.Program()
	workflowIDs := sortedKeys(program.Workflows)
	lambdaIDs := sortedKeys(program.Lambdas)
	schemas := sortedKeys(program.Schemas)
	providers := idpprogram.ExplainProviders(program)
	encodedProviders, err := json.Marshal(providers)
	if err != nil {
		return err
	}
	contract, err := idpprogram.CanonicalJSON(program)
	if err != nil {
		return err
	}
	fingerprints := artifact.Fingerprints()
	return gp.AddRow(ctx, types.NewRow(
		types.MRP("source", settings.Source),
		types.MRP("profile", settings.Profile),
		types.MRP("program_fingerprint", fingerprints.Program),
		types.MRP("workflows", workflowIDs),
		types.MRP("lambdas", lambdaIDs),
		types.MRP("schemas", schemas),
		types.MRP("providers", string(encodedProviders)),
		types.MRP("program_contract", string(contract)),
	))
}

func (c *ScriptTestCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	settings := &ScriptSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, settings); err != nil {
		return err
	}
	if settings.Profile != "signup" {
		return fmt.Errorf("unsupported production script profile %q (want signup)", settings.Profile)
	}
	source, err := os.ReadFile(settings.Source)
	if err != nil {
		return fmt.Errorf("read script source: %w", err)
	}
	executor, err := idpsignup.New(ctx, string(source), 1)
	if err != nil {
		return err
	}
	defer executor.Close(context.Background()) //nolint:errcheck // outcome below is authoritative.
	results := executor.RunTests(ctx)
	for _, result := range results {
		if err := gp.AddRow(ctx, types.NewRow(types.MRP("id", result.ID), types.MRP("passed", result.Passed), types.MRP("expected_kind", string(result.Expected)), types.MRP("actual_kind", string(result.Actual)))); err != nil {
			return err
		}
		if !result.Passed {
			if result.Err != nil {
				return fmt.Errorf("script test %q failed: %w", result.ID, result.Err)
			}
			return fmt.Errorf("script test %q failed: expected outcome %q, got %q", result.ID, result.Expected, result.Actual)
		}
	}
	return nil
}

func loadScriptArtifact(ctx context.Context, vals *values.Values) (*ScriptSettings, *idpscript.Artifact, error) {
	settings := &ScriptSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, settings); err != nil {
		return nil, nil, err
	}
	if settings.Profile != "signup" {
		return nil, nil, fmt.Errorf("unsupported production script profile %q (want signup)", settings.Profile)
	}
	source, err := os.ReadFile(settings.Source)
	if err != nil {
		return nil, nil, fmt.Errorf("read script source: %w", err)
	}
	artifact, err := idpsignup.Compile(ctx, string(source))
	if err != nil {
		return nil, nil, err
	}
	return settings, artifact, nil
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
