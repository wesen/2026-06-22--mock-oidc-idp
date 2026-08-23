package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type DoctorCommand struct {
	*cmds.CommandDescription
}

type DoctorSettings struct {
	ProductRoot string `glazed:"product-root"`
	StateRoot   string `glazed:"state-root"`
}

var _ cmds.GlazeCommand = (*DoctorCommand)(nil)

func NewDoctorCommand() (*DoctorCommand, error) {
	glazedSection, err := settings.NewStructuredOutputSection()
	if err != nil {
		return nil, errors.Wrap(err, "create glazed output section")
	}
	commandSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, errors.Wrap(err, "create command settings section")
	}
	return &DoctorCommand{CommandDescription: cmds.NewCommandDescription(
		"doctor",
		cmds.WithShort("Validate the generated-host source skeleton"),
		cmds.WithLong(`Validate that the xgoja specification, trusted route script,
Durable Object bundle, and embedded frontend inputs exist before generation.

This is a source-layout check. Later phases add persistent-state, identity,
runtime, readiness, and backup diagnostics.`),
		cmds.WithFlags(
			fields.New("product-root", fields.TypeString, fields.WithDefault("cmd/tinyidp-xapp"), fields.WithHelp("Product source root containing xgoja.yaml and app/")),
			fields.New("state-root", fields.TypeString, fields.WithDefault(""), fields.WithHelp("Optional initialized state root for an in-process production interaction check")),
		),
		cmds.WithSections(glazedSection, commandSection),
	)}, nil
}

func (c *DoctorCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, processor middlewares.Processor) error {
	var cfg DoctorSettings
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &cfg); err != nil {
		return errors.Wrap(err, "decode doctor settings")
	}
	required := []string{
		"xgoja.yaml",
		"app/routes/site.js",
		"app/objects/objects.js",
		"app/frontend/package.json",
		"app/frontend/index.html",
		"app/frontend/src/App.tsx",
		"app/frontend/src/styles.css",
		"app/frontend/pnpm-lock.yaml",
		"app/types/xgoja-modules.d.ts",
		"internal/loginui/renderer.go",
		"internal/loginui/templates/interaction.html",
		"internal/loginui/static/login.css",
		"internal/xgojaruntime/xgoja_runtime.gen.go",
	}
	for _, relative := range required {
		path := filepath.Join(cfg.ProductRoot, relative)
		info, err := os.Stat(path)
		status := "ok"
		var size int64
		if err != nil {
			status = "missing"
		} else if info.IsDir() {
			status = "not-a-file"
		} else {
			size = info.Size()
		}
		if err := processor.AddRow(ctx, types.NewRow(
			types.MRP("path", path),
			types.MRP("status", status),
			types.MRP("bytes", size),
		)); err != nil {
			return errors.Wrap(err, "emit doctor result")
		}
		if status != "ok" {
			return errors.Errorf("required product file %q has status %s", path, status)
		}
	}
	if cfg.StateRoot != "" {
		app, err := NewInitializedApplication(ctx, cfg.StateRoot)
		if err != nil {
			return errors.Wrap(err, "open initialized product for interaction doctor")
		}
		health, checkErr := app.CheckInteractionUI(ctx)
		closeErr := app.Close(context.Background())
		if checkErr != nil {
			return errors.Wrap(checkErr, "interaction doctor")
		}
		if closeErr != nil {
			return errors.Wrap(closeErr, "close interaction doctor application")
		}
		if err := processor.AddRow(ctx, types.NewRow(
			types.MRP("path", "interaction-document"),
			types.MRP("status", "ok"),
			types.MRP("bytes", health.HTMLBytes),
		)); err != nil {
			return errors.Wrap(err, "emit interaction document result")
		}
		if err := processor.AddRow(ctx, types.NewRow(
			types.MRP("path", "interaction-stylesheet"),
			types.MRP("status", "ok"),
			types.MRP("bytes", health.CSSBytes),
		)); err != nil {
			return errors.Wrap(err, "emit interaction stylesheet result")
		}
	}
	return nil
}
