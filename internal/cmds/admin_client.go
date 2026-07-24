package cmds

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
	idpstore "github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

func newAdminClientCommand(dbPath *string) *cobra.Command {
	var actionKeyFile string
	cmd := &cobra.Command{Use: "client", Short: "Manage OAuth clients through guarded administration commands"}
	cmd.PersistentFlags().StringVar(&actionKeyFile, "admin-action-key-file", "", "Owner-only action-handle key file")
	cmd.AddCommand(newAdminClientCreateCommand(dbPath, &actionKeyFile))
	cmd.AddCommand(newAdminClientUpdateCommand(dbPath, &actionKeyFile))
	cmd.AddCommand(newAdminClientListCommand(dbPath))
	cmd.AddCommand(newAdminClientGetCommand(dbPath))
	cmd.AddCommand(newAdminClientDisableCommand(dbPath, &actionKeyFile, true))
	cmd.AddCommand(newAdminClientDisableCommand(dbPath, &actionKeyFile, false))
	cmd.AddCommand(newAdminClientRotateSecretCommand(dbPath, &actionKeyFile))
	return cmd
}

type clientFlags struct {
	public, requirePKCE, canIntrospect                      bool
	redirectURIs, scopes, grantTypes, audiences, postLogout []string
}

func newAdminClientCreateCommand(dbPath, actionKeyFile *string) *cobra.Command {
	var id string
	var flags clientFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an OAuth client and print a generated confidential secret once",
		RunE: func(cmd *cobra.Command, _ []string) error {
			runtime, err := openAdminCommandRuntime(cmd.Context(), *dbPath, *actionKeyFile, "")
			if err != nil {
				return err
			}
			defer runtime.close()
			response, err := runtime.executeCommand(cmd.Context(), runtime.clients,
				idpadminapp.CommandClientsCreate, id, flags.input())
			if err != nil {
				return err
			}
			return writeClientCommandResponse(cmd, "created", response)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Client ID")
	addClientConfigurationFlags(cmd, &flags)
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newAdminClientUpdateCommand(dbPath, actionKeyFile *string) *cobra.Command {
	var id string
	var flags clientFlags
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Replace an OAuth client's registered configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			runtime, err := openAdminCommandRuntime(cmd.Context(), *dbPath, *actionKeyFile, "")
			if err != nil {
				return err
			}
			defer runtime.close()
			response, err := runtime.executeCommand(cmd.Context(), runtime.clients,
				idpadminapp.CommandClientsUpdate, id, flags.input())
			if err != nil {
				return err
			}
			return writeClientCommandResponse(cmd, "updated", response)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Client ID")
	addClientConfigurationFlags(cmd, &flags)
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func addClientConfigurationFlags(cmd *cobra.Command, flags *clientFlags) {
	cmd.Flags().BoolVar(&flags.public, "public", false, "Configure a public client")
	cmd.Flags().StringArrayVar(&flags.redirectURIs, "redirect-uri", nil, "Allowed redirect URI (repeatable)")
	cmd.Flags().StringArrayVar(&flags.postLogout, "post-logout-redirect-uri", nil, "Allowed post-logout redirect URI (repeatable)")
	cmd.Flags().StringArrayVar(&flags.scopes, "scope", []string{"openid", "profile", "email"}, "Allowed scope (repeatable)")
	cmd.Flags().StringArrayVar(&flags.grantTypes, "grant-type", nil, "Allowed OAuth grant type (repeatable)")
	cmd.Flags().StringArrayVar(&flags.audiences, "audience", nil, "Allowed OAuth resource indicator (repeatable)")
	cmd.Flags().BoolVar(&flags.canIntrospect, "can-introspect", false, "Authorize this confidential client for introspection")
	cmd.Flags().BoolVar(&flags.requirePKCE, "require-pkce", true, "Require PKCE")
}

func (f clientFlags) input() map[string]any {
	return map[string]any{
		"public": f.public, "redirect_uris": f.redirectURIs,
		"post_logout_redirect_uris": f.postLogout, "allowed_scopes": f.scopes,
		"allowed_grant_types": f.grantTypes, "allowed_audiences": f.audiences,
		"can_introspect": f.canIntrospect, "require_pkce": f.requirePKCE,
	}
}

func newAdminClientListCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List clients", RunE: func(cmd *cobra.Command, _ []string) error {
		svc, closeFn, err := openAdminService(*dbPath)
		if err != nil {
			return err
		}
		defer closeFn()
		clients, err := svc.ListClients(cmd.Context())
		if err != nil {
			return err
		}
		out := make([]any, 0, len(clients))
		for _, c := range clients {
			out = append(out, redactClient(c))
		}
		return writeJSONLine(cmd.OutOrStdout(), map[string]any{"clients": out})
	}}
}

func newAdminClientGetCommand(dbPath *string) *cobra.Command {
	var id string
	cmd := &cobra.Command{Use: "get", Short: "Get client", RunE: func(cmd *cobra.Command, _ []string) error {
		svc, closeFn, err := openAdminService(*dbPath)
		if err != nil {
			return err
		}
		defer closeFn()
		client, err := svc.GetClient(cmd.Context(), id)
		if err != nil {
			return err
		}
		return writeJSONLine(cmd.OutOrStdout(), map[string]any{"client": redactClient(client)})
	}}
	cmd.Flags().StringVar(&id, "id", "", "Client ID")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newAdminClientDisableCommand(dbPath, actionKeyFile *string, disabled bool) *cobra.Command {
	name, status, confirmation := "enable", "enabled", ""
	command := idpadminapp.CommandClientsEnable
	if disabled {
		name, status, confirmation = "disable", "disabled", "DISABLE"
		command = idpadminapp.CommandClientsDisable
	}
	var id, reason, confirm string
	cmd := &cobra.Command{Use: name, Short: name + " client", RunE: func(cmd *cobra.Command, _ []string) error {
		runtime, err := openAdminCommandRuntime(cmd.Context(), *dbPath, *actionKeyFile, "")
		if err != nil {
			return err
		}
		defer runtime.close()
		response, err := runtime.executeCommand(cmd.Context(), runtime.clients, command, id,
			map[string]any{"reason": reason, "confirmation": confirm})
		if err != nil {
			return err
		}
		return writeClientCommandResponse(cmd, status, response)
	}}
	cmd.Flags().StringVar(&id, "id", "", "Client ID")
	cmd.Flags().StringVar(&reason, "reason", "", "Required operator reason")
	cmd.Flags().StringVar(&confirm, "confirm", "", fmt.Sprintf("Typed confirmation (%s)", confirmation))
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}

func newAdminClientRotateSecretCommand(dbPath, actionKeyFile *string) *cobra.Command {
	var id, reason, confirmation string
	cmd := &cobra.Command{Use: "rotate-secret", Short: "Rotate a confidential client secret and print it once", RunE: func(cmd *cobra.Command, _ []string) error {
		runtime, err := openAdminCommandRuntime(cmd.Context(), *dbPath, *actionKeyFile, "")
		if err != nil {
			return err
		}
		defer runtime.close()
		response, err := runtime.executeCommand(cmd.Context(), runtime.clients,
			idpadminapp.CommandClientsRotateSecret, id,
			map[string]any{"reason": reason, "confirmation": confirmation})
		if err != nil {
			return err
		}
		return writeClientCommandResponse(cmd, "secret-rotated", response)
	}}
	cmd.Flags().StringVar(&id, "id", "", "Client ID")
	cmd.Flags().StringVar(&reason, "reason", "", "Required operator reason")
	cmd.Flags().StringVar(&confirmation, "confirm", "", "Typed confirmation (ROTATE)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}

func writeClientCommandResponse(cmd *cobra.Command, status string, response []byte) error {
	var secret idpadmin.OneTimeSecretResult
	if err := json.Unmarshal(response, &secret); err == nil && secret.Secret != "" {
		return writeJSONLine(cmd.OutOrStdout(), map[string]any{"status": status, "secret": secret})
	}
	var result idpadmin.ClientResult
	if err := json.Unmarshal(response, &result); err != nil {
		return err
	}
	return writeJSONLine(cmd.OutOrStdout(), map[string]any{"status": status, "client": result.Client})
}

func redactClient(client idpstore.Client) map[string]any {
	return map[string]any{
		"id": client.ID, "public": client.Public, "has_secret": len(client.SecretHash) > 0,
		"redirect_uris":             client.RedirectURIs,
		"post_logout_redirect_uris": client.PostLogoutRedirectURIs,
		"allowed_scopes":            client.AllowedScopes, "allowed_audiences": client.AllowedAudiences,
		"can_introspect": client.CanIntrospect, "require_pkce": client.RequirePKCE,
		"access_token_ttl": client.AccessTokenTTL.String(), "id_token_ttl": client.IDTokenTTL.String(),
		"refresh_token_ttl": client.RefreshTokenTTL.String(), "disabled": client.Disabled,
		"created_at": client.CreatedAt, "updated_at": client.UpdatedAt,
	}
}
