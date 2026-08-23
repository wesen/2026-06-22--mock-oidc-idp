package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
)

type adminInvitationIssueSettings struct {
	Audience      string `glazed:"audience"`
	Label         string `glazed:"label"`
	TTL           string `glazed:"ttl"`
	LookupKeyFile string `glazed:"lookup-key-file"`
	ActionKeyFile string `glazed:"admin-action-key-file"`
}

type adminInvitationRevokeSettings struct {
	InvitationID  string `glazed:"invitation-id"`
	Reason        string `glazed:"reason"`
	Confirmation  string `glazed:"confirm"`
	LookupKeyFile string `glazed:"lookup-key-file"`
	ActionKeyFile string `glazed:"admin-action-key-file"`
}

type AdminInvitationIssueCommand struct {
	*cmds.CommandDescription
	dbPath *string
	now    func() time.Time
}

type AdminInvitationRevokeCommand struct {
	*cmds.CommandDescription
	dbPath *string
	now    func() time.Time
}

func newAdminInvitationCommand(dbPath *string) (*cobra.Command, error) {
	root := &cobra.Command{Use: "invitation", Short: "Issue and revoke durable signup invitations through guarded commands"}
	issue, err := newAdminInvitationIssueCommand(dbPath)
	if err != nil {
		return nil, err
	}
	issueCobra, err := cli.BuildCobraCommand(issue)
	if err != nil {
		return nil, err
	}
	revoke, err := newAdminInvitationRevokeCommand(dbPath)
	if err != nil {
		return nil, err
	}
	revokeCobra, err := cli.BuildCobraCommand(revoke)
	if err != nil {
		return nil, err
	}
	root.AddCommand(issueCobra, revokeCobra)
	return root, nil
}

func newAdminInvitationSections() (schema.Section, schema.Section, error) {
	output, err := settings.NewStructuredOutputSection(schema.WithDefaults(map[string]any{"format": "json"}))
	if err != nil {
		return nil, nil, err
	}
	command, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, nil, err
	}
	return output, command, nil
}

func newAdminInvitationIssueCommand(dbPath *string) (*AdminInvitationIssueCommand, error) {
	output, command, err := newAdminInvitationSections()
	if err != nil {
		return nil, err
	}
	description := cmds.NewCommandDescription("issue",
		cmds.WithShort("Issue a one-time signup invitation and print its raw code once"),
		cmds.WithFlags(
			fields.New("audience", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Exact OIDC client ID allowed to redeem the invitation")),
			fields.New("label", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Non-secret operator label")),
			fields.New("ttl", fields.TypeString, fields.WithDefault("24h"), fields.WithHelp("Invitation lifetime, at most 30 days")),
			fields.New("lookup-key-file", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Owner-only durable invitation HMAC key file")),
			fields.New("admin-action-key-file", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Owner-only action-handle key file")),
		),
		cmds.WithSections(output, command),
	)
	return &AdminInvitationIssueCommand{CommandDescription: description, dbPath: dbPath, now: time.Now}, nil
}

func newAdminInvitationRevokeCommand(dbPath *string) (*AdminInvitationRevokeCommand, error) {
	output, command, err := newAdminInvitationSections()
	if err != nil {
		return nil, err
	}
	description := cmds.NewCommandDescription("revoke",
		cmds.WithShort("Revoke an unused signup invitation by public invitation ID"),
		cmds.WithFlags(
			fields.New("invitation-id", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Public invitation ID from issue/list output")),
			fields.New("reason", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Operator reason recorded in action evidence")),
			fields.New("confirm", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Type REVOKE")),
			fields.New("lookup-key-file", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Owner-only durable invitation HMAC key file")),
			fields.New("admin-action-key-file", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Owner-only action-handle key file")),
		),
		cmds.WithSections(output, command),
	)
	return &AdminInvitationRevokeCommand{CommandDescription: description, dbPath: dbPath, now: time.Now}, nil
}

func (c *AdminInvitationIssueCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	processor middlewares.Processor,
) error {
	var cfg adminInvitationIssueSettings
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &cfg); err != nil {
		return err
	}
	if _, err := time.ParseDuration(cfg.TTL); err != nil {
		return fmt.Errorf("parse invitation ttl: %w", err)
	}
	runtime, err := openAdminCommandRuntime(ctx, valueOf(c.dbPath), cfg.ActionKeyFile, cfg.LookupKeyFile)
	if err != nil {
		return err
	}
	defer runtime.close()
	response, err := runtime.executeCommand(ctx, runtime.invitations,
		idpadminapp.CommandInvitationsIssue, "", map[string]any{
			"audience": strings.TrimSpace(cfg.Audience), "label": strings.TrimSpace(cfg.Label),
			"valid_for": strings.TrimSpace(cfg.TTL),
		})
	if err != nil {
		return err
	}
	var result idpadmin.OneTimeSecretResult
	if err := json.Unmarshal(response, &result); err != nil {
		return err
	}
	return processor.AddRow(ctx, types.NewRow(
		types.MRP("status", "issued"), types.MRP("invitation_id", result.ResourceID),
		types.MRP("audience", cfg.Audience), types.MRP("code", result.Secret),
	))
}

func (c *AdminInvitationRevokeCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	processor middlewares.Processor,
) error {
	var cfg adminInvitationRevokeSettings
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &cfg); err != nil {
		return err
	}
	runtime, err := openAdminCommandRuntime(ctx, valueOf(c.dbPath), cfg.ActionKeyFile, cfg.LookupKeyFile)
	if err != nil {
		return err
	}
	defer runtime.close()
	response, err := runtime.executeCommand(ctx, runtime.invitations,
		idpadminapp.CommandInvitationsRevoke, strings.TrimSpace(cfg.InvitationID),
		map[string]any{"reason": cfg.Reason, "confirmation": cfg.Confirmation})
	if err != nil {
		return err
	}
	var result idpadmin.InvitationResult
	if err := json.Unmarshal(response, &result); err != nil {
		return err
	}
	return processor.AddRow(ctx, types.NewRow(
		types.MRP("status", result.Invitation.Status),
		types.MRP("invitation_id", result.Invitation.ID),
		types.MRP("audience", result.Invitation.Audience),
		types.MRP("revoked_at", result.Invitation.RevokedAt),
	))
}

func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
