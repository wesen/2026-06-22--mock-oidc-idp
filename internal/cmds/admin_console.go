package cmds

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/spf13/cobra"

	"github.com/go-go-golems/tiny-idp/pkg/idp"
	"github.com/go-go-golems/tiny-idp/pkg/idpaccounts"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
)

type consoleBootstrapSettings struct {
	OwnerLogin        string `glazed:"owner-login"`
	OwnerPasswordFile string `glazed:"owner-password-file"`
	OwnerEmail        string `glazed:"owner-email"`
	OwnerDisplayName  string `glazed:"owner-display-name"`
	PublicBaseURL     string `glazed:"public-base-url"`
}

type consoleRevokeGrantSettings struct {
	GrantID         string `glazed:"grant-id"`
	ExpectedVersion int64  `glazed:"expected-version"`
}

type consoleRevokeSessionSettings struct {
	SessionIDHash string `glazed:"session-id-hash"`
}

type AdminConsoleBootstrapCommand struct {
	*cmds.CommandDescription
	dbPath *string
	now    func() time.Time
}

type AdminConsoleStatusCommand struct {
	*cmds.CommandDescription
	dbPath *string
	now    func() time.Time
}

type AdminConsoleRevokeGrantCommand struct {
	*cmds.CommandDescription
	dbPath *string
	now    func() time.Time
}

type AdminConsoleRevokeSessionCommand struct {
	*cmds.CommandDescription
	dbPath *string
	now    func() time.Time
}

func newAdminConsoleCommand(dbPath *string) (*cobra.Command, error) {
	root := &cobra.Command{Use: "console", Short: "Bootstrap and recover TinyIDP Console ownership"}
	commands := []cmds.GlazeCommand{}
	bootstrap, err := newAdminConsoleBootstrapCommand(dbPath)
	if err != nil {
		return nil, err
	}
	status, err := newAdminConsoleStatusCommand(dbPath)
	if err != nil {
		return nil, err
	}
	revokeGrant, err := newAdminConsoleRevokeGrantCommand(dbPath)
	if err != nil {
		return nil, err
	}
	revokeSession, err := newAdminConsoleRevokeSessionCommand(dbPath)
	if err != nil {
		return nil, err
	}
	commands = append(commands, bootstrap, status, revokeGrant, revokeSession)
	for _, command := range commands {
		cobraCommand, err := cli.BuildCobraCommandFromCommand(command,
			cli.WithParserConfig(cli.CobraParserConfig{
				ShortHelpSections: []string{schema.DefaultSlug},
				MiddlewaresFunc:   cli.CobraCommandDefaultMiddlewares,
			}))
		if err != nil {
			return nil, err
		}
		root.AddCommand(cobraCommand)
	}
	return root, nil
}

func newAdminConsoleSections() (schema.Section, schema.Section, error) {
	output, err := settings.NewStructuredOutputSection(
		schema.WithDefaults(map[string]any{"format": "json"}),
	)
	if err != nil {
		return nil, nil, err
	}
	command, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, nil, err
	}
	return output, command, nil
}

func newAdminConsoleBootstrapCommand(dbPath *string) (*AdminConsoleBootstrapCommand, error) {
	output, command, err := newAdminConsoleSections()
	if err != nil {
		return nil, err
	}
	description := cmds.NewCommandDescription("bootstrap",
		cmds.WithShort("Grant the existing installation owner access to TinyIDP Console"),
		cmds.WithLong(`Create the fixed public PKCE console client and exactly one active system owner grant.

Example:
  tinyidp admin --db /var/lib/tinyidp/idp.db console bootstrap \
    --owner-login owner --owner-password-file /run/secrets/tinyidp-owner-password \
    --public-base-url https://id.example`),
		cmds.WithFlags(
			fields.New("owner-login", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Existing or first-install TinyIDP login that will own the console")),
			fields.New("owner-password-file", fields.TypeString, fields.WithHelp("Owner-only password file used only when atomically creating the first owner")),
			fields.New("owner-email", fields.TypeString, fields.WithHelp("Email claim for a newly provisioned owner")),
			fields.New("owner-display-name", fields.TypeString, fields.WithHelp("Display name for a newly provisioned owner")),
			fields.New("public-base-url", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Canonical HTTPS issuer origin")),
		),
		cmds.WithSections(output, command),
	)
	return &AdminConsoleBootstrapCommand{CommandDescription: description, dbPath: dbPath, now: time.Now}, nil
}

func newAdminConsoleStatusCommand(dbPath *string) (*AdminConsoleStatusCommand, error) {
	output, command, err := newAdminConsoleSections()
	if err != nil {
		return nil, err
	}
	description := cmds.NewCommandDescription("status",
		cmds.WithShort("Show the active system owner grant without exposing secrets"),
		cmds.WithSections(output, command),
	)
	return &AdminConsoleStatusCommand{CommandDescription: description, dbPath: dbPath, now: time.Now}, nil
}

func newAdminConsoleRevokeGrantCommand(dbPath *string) (*AdminConsoleRevokeGrantCommand, error) {
	output, command, err := newAdminConsoleSections()
	if err != nil {
		return nil, err
	}
	description := cmds.NewCommandDescription("revoke-grant",
		cmds.WithShort("Revoke the active owner grant using compare-and-set"),
		cmds.WithFlags(
			fields.New("grant-id", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Owner grant ID returned by bootstrap or status")),
			fields.New("expected-version", fields.TypeInteger, fields.WithRequired(true), fields.WithHelp("Current grant version")),
		),
		cmds.WithSections(output, command),
	)
	return &AdminConsoleRevokeGrantCommand{CommandDescription: description, dbPath: dbPath, now: time.Now}, nil
}

func newAdminConsoleRevokeSessionCommand(dbPath *string) (*AdminConsoleRevokeSessionCommand, error) {
	output, command, err := newAdminConsoleSections()
	if err != nil {
		return nil, err
	}
	description := cmds.NewCommandDescription("revoke-session",
		cmds.WithShort("Revoke one admin session by its Base64URL-encoded stored hash"),
		cmds.WithFlags(
			fields.New("session-id-hash", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Base64URL-encoded admin session ID hash")),
		),
		cmds.WithSections(output, command),
	)
	return &AdminConsoleRevokeSessionCommand{CommandDescription: description, dbPath: dbPath, now: time.Now}, nil
}

func (c *AdminConsoleBootstrapCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, processor middlewares.Processor) error {
	var cfg consoleBootstrapSettings
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &cfg); err != nil {
		return err
	}
	dbPath := valueOf(c.dbPath)
	if dbPath == "" {
		return fmt.Errorf("--db is required")
	}
	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(dbPath))
	if err != nil {
		return err
	}
	defer store.Close()
	service, err := idpadminapp.NewOwnerService(store, c.now)
	if err != nil {
		return err
	}
	var preparedOwner *idpaccounts.PreparedCreate
	if cfg.OwnerPasswordFile != "" {
		password, err := readOwnerOnlyFile(cfg.OwnerPasswordFile, "owner password", 1)
		if err != nil {
			return err
		}
		defer clearProductionSecret(password)
		accounts, err := idpaccounts.NewService(store, idpaccounts.Options{
			Clock: c.now, Audit: idp.NoopSink{},
		})
		if err != nil {
			return err
		}
		prepared, err := accounts.PrepareCreate(ctx, idpaccounts.CreateRequest{
			Login: cfg.OwnerLogin, Password: password, Email: cfg.OwnerEmail,
			Name: cfg.OwnerDisplayName,
		})
		if err != nil {
			return err
		}
		preparedOwner = &prepared
	}
	status, err := service.Bootstrap(ctx, idpadminapp.BootstrapOwnerRequest{
		OwnerLogin: cfg.OwnerLogin, PublicBaseURL: cfg.PublicBaseURL,
		PreparedOwner: preparedOwner,
	})
	if err != nil {
		return err
	}
	if err := emitAdminAudit(ctx, valueOf(c.dbPath), idp.Event{
		Time: c.now().UTC(), Name: "admin.owner.bootstrapped", Subject: status.Subject,
		ClientID: status.ClientID, Result: "accepted", Fields: map[string]string{"grant_id": status.GrantID},
	}); err != nil {
		return err
	}
	return processor.AddRow(ctx, ownerStatusRow(status))
}

func (c *AdminConsoleStatusCommand) RunIntoGlazeProcessor(ctx context.Context, _ *values.Values, processor middlewares.Processor) error {
	service, closeFn, err := openOwnerService(valueOf(c.dbPath), c.now)
	if err != nil {
		return err
	}
	defer closeFn()
	status, err := service.Status(ctx)
	if err != nil {
		return err
	}
	return processor.AddRow(ctx, ownerStatusRow(status))
}

func (c *AdminConsoleRevokeGrantCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, processor middlewares.Processor) error {
	var cfg consoleRevokeGrantSettings
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &cfg); err != nil {
		return err
	}
	service, closeFn, err := openOwnerService(valueOf(c.dbPath), c.now)
	if err != nil {
		return err
	}
	defer closeFn()
	if err := service.RevokeGrant(ctx, cfg.GrantID, cfg.ExpectedVersion); err != nil {
		return err
	}
	if err := emitAdminAudit(ctx, valueOf(c.dbPath), idp.Event{
		Time: c.now().UTC(), Name: "admin.owner.grant_revoked", Result: "accepted",
		Fields: map[string]string{"grant_id": cfg.GrantID},
	}); err != nil {
		return err
	}
	return processor.AddRow(ctx, types.NewRow(
		types.MRP("status", "revoked"), types.MRP("grant_id", cfg.GrantID),
		types.MRP("previous_version", cfg.ExpectedVersion),
	))
}

func (c *AdminConsoleRevokeSessionCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, processor middlewares.Processor) error {
	var cfg consoleRevokeSessionSettings
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &cfg); err != nil {
		return err
	}
	hash, err := base64.RawURLEncoding.DecodeString(cfg.SessionIDHash)
	if err != nil {
		return fmt.Errorf("decode session-id-hash: %w", err)
	}
	service, closeFn, err := openOwnerService(valueOf(c.dbPath), c.now)
	if err != nil {
		return err
	}
	defer closeFn()
	if err := service.RevokeSession(ctx, hash); err != nil {
		return err
	}
	if err := emitAdminAudit(ctx, valueOf(c.dbPath), idp.Event{
		Time: c.now().UTC(), Name: "admin.owner.session_revoked", Result: "accepted",
		Fields: map[string]string{"session_id_hash": cfg.SessionIDHash},
	}); err != nil {
		return err
	}
	return processor.AddRow(ctx, types.NewRow(
		types.MRP("status", "revoked"), types.MRP("session_id_hash", cfg.SessionIDHash),
	))
}

func ownerStatusRow(status idpadminapp.OwnerStatus) types.Row {
	return types.NewRow(
		types.MRP("configured", status.Configured),
		types.MRP("grant_id", status.GrantID),
		types.MRP("subject", status.Subject),
		types.MRP("grant_version", status.GrantVersion),
		types.MRP("client_id", status.ClientID),
		types.MRP("redirect_uri", status.RedirectURI),
		types.MRP("issued_at", status.IssuedAt),
	)
}

func openOwnerService(dbPath string, now func() time.Time) (*idpadminapp.OwnerService, func(), error) {
	if dbPath == "" {
		return nil, nil, fmt.Errorf("--db is required")
	}
	store, err := sqlitestore.Open(context.Background(), sqlitestore.DefaultConfig(dbPath))
	if err != nil {
		return nil, nil, err
	}
	service, err := idpadminapp.NewOwnerService(store, now)
	if err != nil {
		_ = store.Close()
		return nil, nil, err
	}
	return service, func() { _ = store.Close() }, nil
}
