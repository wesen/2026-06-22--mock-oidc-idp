package cmds

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/go-go-golems/tiny-idp/internal/admin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
)

func newAdminKeysCommand(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{Use: "keys", Short: "Manage signing keys"}
	var actionKeyFile string
	cmd.PersistentFlags().StringVar(&actionKeyFile, "admin-action-key-file", "", "Owner-only action-handle key file")
	cmd.AddCommand(newAdminKeysGenerateCommand(dbPath))
	cmd.AddCommand(newAdminKeysRotateCommand(dbPath, &actionKeyFile))
	cmd.AddCommand(newAdminKeysListCommand(dbPath))
	cmd.AddCommand(newAdminKeysRetireCommand(dbPath, &actionKeyFile))
	cmd.AddCommand(newAdminKeysPurgeRetiredCommand(dbPath))
	return cmd
}

func newAdminKeysPurgeRetiredCommand(dbPath *string) *cobra.Command {
	var kid string
	cmd := &cobra.Command{Use: "purge-retired", Short: "Emergency-remove a retired key from JWKS trust", Long: "Permanently remove a retired signing key before its normal verification overlap expires. Use only for confirmed or suspected private-key compromise; still-valid tokens signed by this key will stop verifying.", RunE: func(cmd *cobra.Command, _ []string) error {
		svc, closeFn, err := openAdminService(*dbPath)
		if err != nil {
			return err
		}
		defer closeFn()
		if err := svc.PurgeRetiredSigningKey(cmd.Context(), kid); err != nil {
			return err
		}
		return writeJSONLine(cmd.OutOrStdout(), map[string]any{"status": "retired-key-purged", "kid": kid})
	}}
	cmd.Flags().StringVar(&kid, "kid", "", "Retired key ID")
	_ = cmd.MarkFlagRequired("kid")
	return cmd
}

func newAdminKeysGenerateCommand(dbPath *string) *cobra.Command {
	var kid string
	var active bool
	cmd := &cobra.Command{Use: "generate", Short: "Generate an RSA signing key", RunE: func(cmd *cobra.Command, _ []string) error {
		svc, closeFn, err := openAdminService(*dbPath)
		if err != nil {
			return err
		}
		defer closeFn()
		key, err := svc.GenerateSigningKey(cmd.Context(), kid, active)
		if err != nil {
			return err
		}
		return writeJSONLine(cmd.OutOrStdout(), map[string]any{"status": "generated", "key": admin.RedactSigningKey(key)})
	}}
	cmd.Flags().StringVar(&kid, "kid", "", "Key ID (default generated from current time)")
	cmd.Flags().BoolVar(&active, "active", true, "Activate the generated key")
	return cmd
}

func newAdminKeysRotateCommand(dbPath, actionKeyFile *string) *cobra.Command {
	var reason string
	cmd := &cobra.Command{Use: "rotate", Short: "Rotate the active RSA signing key", RunE: func(cmd *cobra.Command, _ []string) error {
		runtime, err := openAdminCommandRuntime(cmd.Context(), *dbPath, *actionKeyFile, "")
		if err != nil {
			return err
		}
		defer runtime.close()
		response, err := runtime.executeCommand(
			cmd.Context(), runtime.keys, idpadminapp.CommandKeysRotate, "",
			map[string]any{
				"algorithm": "RS256", "reason": reason, "confirmation": "ROTATE",
			},
		)
		if err != nil {
			return err
		}
		var result idpadmin.KeyResult
		if err := json.Unmarshal(response, &result); err != nil {
			return err
		}
		return writeJSONLine(cmd.OutOrStdout(), map[string]any{"status": "rotated", "key": result.Key})
	}}
	cmd.Flags().StringVar(&reason, "reason", "", "Operator reason recorded in administration evidence")
	_ = cmd.MarkFlagRequired("reason")
	_ = cmd.MarkFlagRequired("admin-action-key-file")
	return cmd
}

func newAdminKeysListCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List verification keys", RunE: func(cmd *cobra.Command, _ []string) error {
		svc, closeFn, err := openAdminService(*dbPath)
		if err != nil {
			return err
		}
		defer closeFn()
		keys, err := svc.ListSigningKeys(cmd.Context())
		if err != nil {
			return err
		}
		return writeJSONLine(cmd.OutOrStdout(), map[string]any{"keys": admin.RedactSigningKeys(keys)})
	}}
}

func newAdminKeysRetireCommand(dbPath, actionKeyFile *string) *cobra.Command {
	var kid, reason string
	cmd := &cobra.Command{Use: "retire", Short: "Retire a signing key", RunE: func(cmd *cobra.Command, _ []string) error {
		runtime, err := openAdminCommandRuntime(cmd.Context(), *dbPath, *actionKeyFile, "")
		if err != nil {
			return err
		}
		defer runtime.close()
		response, err := runtime.executeCommand(
			cmd.Context(), runtime.keys, idpadminapp.CommandKeysRetire, kid,
			map[string]any{"reason": reason, "confirmation": "RETIRE"},
		)
		if err != nil {
			return err
		}
		var result idpadmin.KeyResult
		if err := json.Unmarshal(response, &result); err != nil {
			return err
		}
		return writeJSONLine(cmd.OutOrStdout(), map[string]any{"status": "retired", "key": result.Key})
	}}
	cmd.Flags().StringVar(&kid, "kid", "", "Key ID")
	cmd.Flags().StringVar(&reason, "reason", "", "Operator reason recorded in administration evidence")
	_ = cmd.MarkFlagRequired("kid")
	_ = cmd.MarkFlagRequired("reason")
	_ = cmd.MarkFlagRequired("admin-action-key-file")
	return cmd
}
