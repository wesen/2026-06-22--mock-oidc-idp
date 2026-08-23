package cmds

import (
	"context"
	"fmt"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"

	"github.com/go-go-golems/tiny-idp/internal/sections/oidc"
)

// PrintConfigCommand implements `tinyidp print-config`. It composes the same
// reusable OIDC section as `serve`, resolves it through the full precedence
// chain (defaults < profiles < config < env < flags), and emits the resolved
// configuration as a single Glazed row.
//
// This command exists for two reasons. First, it is a debugging tool: a
// developer can see exactly which issuer, client, and redirect URIs the
// provider would run with, including which source (default / profile / env /
// flag) won each value, without starting a server. Second, it is the second
// consumer of the `oidc` section, which validates that the section is
// genuinely reusable rather than accidentally coupled to `serve`.
type PrintConfigCommand struct {
	*cmds.CommandDescription
}

// NewPrintConfigCommand builds the `print-config` command. It composes the
// OIDC section and the Glazed output section so the result can be rendered
// as json, yaml, or a table via the standard --format flag.
func NewPrintConfigCommand() (*PrintConfigCommand, error) {
	oidcSection, err := oidc.NewSection()
	if err != nil {
		return nil, fmt.Errorf("build oidc section: %w", err)
	}
	glazedSection, err := settings.NewStructuredOutputSection(
		schema.WithDefaults(map[string]interface{}{
			"format": "yaml",
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("build glazed section: %w", err)
	}
	commandSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, fmt.Errorf("build command-settings section: %w", err)
	}

	cmdDesc := cmds.NewCommandDescription(
		"print-config",
		cmds.WithShort("Print the resolved OIDC provider configuration"),
		cmds.WithLong(`Print the resolved OIDC provider configuration.

This command resolves the OIDC section through the full precedence chain
(defaults < profiles < config < env < flags) and emits the result. It is a
debugging tool: use it to confirm which issuer, client, and redirect URIs
the provider would run with before calling `+"`tinyidp serve`"+`.

It composes the same reusable `+"`oidc`"+` field section as `+"`serve`"+`, so
the output is exactly what `+"`serve`"+` would use for the same flags, env,
config file, and profile.

Examples:
  tinyidp print-config
  tinyidp print-config --profile dev
  tinyidp print-config --client-id my-app --output yaml
  tinyidp print-config --users-file ./users.yaml
  TINYIDP_CLIENT_ID=env-app tinyidp print-config
`),
		cmds.WithSections(oidcSection, glazedSection, commandSettingsSection),
	)
	return &PrintConfigCommand{CommandDescription: cmdDesc}, nil
}

// RunIntoGlazeProcessor decodes the OIDC section and emits one row with the
// resolved configuration fields. The Glazed output section (--output) formats
// the row as json, yaml, or a table.
func (c *PrintConfigCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	cfg, err := oidc.GetSettings(vals)
	if err != nil {
		return err
	}

	row := types.NewRow(
		types.MRP("issuer", cfg.Issuer),
		types.MRP("addr", cfg.Addr),
		types.MRP("client_id", cfg.ClientID),
		types.MRP("client_secret", cfg.ClientSecret),
		types.MRP("redirect_uris", cfg.RedirectURIs),
		types.MRP("users_file", cfg.UsersFile),
		types.MRP("engine", cfg.Engine),
	)
	return gp.AddRow(ctx, row)
}
