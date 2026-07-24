package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/go-go-golems/tiny-idp/internal/admin"
	"github.com/go-go-golems/tiny-idp/pkg/idp"
	"github.com/go-go-golems/tiny-idp/pkg/idpaccounts"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
)

func NewAdminCommand() (*cobra.Command, error) {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Manage tinyidp production users and credentials",
		Long: `Manage tinyidp production users and credentials.

These commands operate directly on the configured SQLite database. Passwords
should be supplied through --password-from-stdin for normal use so they do not
land in shell history. The --password flag is available for tests and local
throwaway databases only.`,
	}
	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to tinyidp SQLite database")
	cmd.AddCommand(newAdminInitCommand(&dbPath))
	cmd.AddCommand(newAdminMigrateCommand(&dbPath))
	cmd.AddCommand(newAdminDoctorCommand(&dbPath))
	cmd.AddCommand(newAdminClientCommand(&dbPath))
	cmd.AddCommand(newAdminKeysCommand(&dbPath))
	cmd.AddCommand(newAdminUserCommand(&dbPath))
	console, err := newAdminConsoleCommand(&dbPath)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(console)
	invitation, err := newAdminInvitationCommand(&dbPath)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(invitation)
	cmd.AddCommand(newAdminBackupCommand(&dbPath))
	cmd.AddCommand(newAdminExportCommand(&dbPath))
	return cmd, nil
}

func newAdminUserCommand(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{Use: "user", Short: "Manage users and password credentials"}
	cmd.AddCommand(newAdminUserCreateCommand(dbPath))
	cmd.AddCommand(newAdminUserSetPasswordCommand(dbPath))
	cmd.AddCommand(newAdminUserGetCommand(dbPath))
	cmd.AddCommand(newAdminUserDisableCommand(dbPath, true))
	cmd.AddCommand(newAdminUserDisableCommand(dbPath, false))
	return cmd
}

func newAdminUserCreateCommand(dbPath *string) *cobra.Command {
	var login, password, email, name, sub, id string
	var emailVerified, passwordFromStdin bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a user and password credential",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pw, err := readAdminPassword(password, passwordFromStdin, cmd.InOrStdin())
			if err != nil {
				return err
			}
			svc, closeFn, err := openAccountService(*dbPath)
			if err != nil {
				return err
			}
			defer closeFn()
			u, err := svc.Create(cmd.Context(), idpaccounts.CreateRequest{Login: login, Password: pw, ID: id, Subject: sub, Email: email, EmailVerified: emailVerified, Name: name})
			if err != nil {
				return err
			}
			return writeJSONLine(cmd.OutOrStdout(), map[string]any{"status": "created", "user": u})
		},
	}
	cmd.Flags().StringVar(&login, "login", "", "Login name")
	cmd.Flags().StringVar(&password, "password", "", "Password value (prefer --password-from-stdin outside tests)")
	cmd.Flags().BoolVar(&passwordFromStdin, "password-from-stdin", false, "Read password from stdin")
	cmd.Flags().StringVar(&id, "id", "", "Optional user ID")
	cmd.Flags().StringVar(&sub, "sub", "", "Optional OIDC subject; defaults to user ID")
	cmd.Flags().StringVar(&email, "email", "", "Email claim")
	cmd.Flags().BoolVar(&emailVerified, "email-verified", false, "Set email_verified claim")
	cmd.Flags().StringVar(&name, "name", "", "Display name")
	_ = cmd.MarkFlagRequired("login")
	return cmd
}

func newAdminUserSetPasswordCommand(dbPath *string) *cobra.Command {
	var login, password string
	var passwordFromStdin bool
	cmd := &cobra.Command{
		Use:   "set-password",
		Short: "Set or replace a user's password credential",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pw, err := readAdminPassword(password, passwordFromStdin, cmd.InOrStdin())
			if err != nil {
				return err
			}
			svc, closeFn, err := openAccountService(*dbPath)
			if err != nil {
				return err
			}
			defer closeFn()
			if err := svc.SetPassword(cmd.Context(), idpaccounts.SetPasswordRequest{Login: login, Password: pw}); err != nil {
				return err
			}
			return writeJSONLine(cmd.OutOrStdout(), map[string]any{"status": "password-updated", "login": login})
		},
	}
	cmd.Flags().StringVar(&login, "login", "", "Login name")
	cmd.Flags().StringVar(&password, "password", "", "Password value (prefer --password-from-stdin outside tests)")
	cmd.Flags().BoolVar(&passwordFromStdin, "password-from-stdin", false, "Read password from stdin")
	_ = cmd.MarkFlagRequired("login")
	return cmd
}

func newAdminUserGetCommand(dbPath *string) *cobra.Command {
	var login string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a user by login",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, closeFn, err := openAdminService(*dbPath)
			if err != nil {
				return err
			}
			defer closeFn()
			u, err := svc.GetUserByLogin(cmd.Context(), login)
			if err != nil {
				return err
			}
			return writeJSONLine(cmd.OutOrStdout(), map[string]any{"user": u})
		},
	}
	cmd.Flags().StringVar(&login, "login", "", "Login name")
	_ = cmd.MarkFlagRequired("login")
	return cmd
}

func newAdminUserDisableCommand(dbPath *string, disabled bool) *cobra.Command {
	name := "enable"
	status := "enabled"
	shortVerb := "Enable"
	if disabled {
		name = "disable"
		status = "disabled"
		shortVerb = "Disable"
	}
	var login string
	cmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("%s a user", shortVerb),
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, closeFn, err := openAdminService(*dbPath)
			if err != nil {
				return err
			}
			defer closeFn()
			u, err := svc.SetUserDisabled(cmd.Context(), login, disabled)
			if err != nil {
				return err
			}
			return writeJSONLine(cmd.OutOrStdout(), map[string]any{"status": status, "user": u})
		},
	}
	cmd.Flags().StringVar(&login, "login", "", "Login name")
	_ = cmd.MarkFlagRequired("login")
	return cmd
}

func openAdminService(dbPath string) (*admin.Service, func(), error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, nil, fmt.Errorf("--db is required")
	}
	st, err := sqlitestore.Open(context.Background(), sqlitestore.DefaultConfig(dbPath))
	if err != nil {
		return nil, nil, err
	}
	audit, err := idp.NewFileAuditSink(dbPath + ".audit.jsonl")
	if err != nil {
		_ = st.Close()
		return nil, nil, err
	}
	svc, err := admin.NewService(st, admin.Options{Audit: audit})
	if err != nil {
		_ = audit.Close()
		_ = st.Close()
		return nil, nil, err
	}
	return svc, func() { _ = audit.Close(); _ = st.Close() }, nil
}

func openAccountService(dbPath string) (*idpaccounts.Service, func(), error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, nil, fmt.Errorf("--db is required")
	}
	store, err := sqlitestore.Open(context.Background(), sqlitestore.DefaultConfig(dbPath))
	if err != nil {
		return nil, nil, err
	}
	audit, err := idp.NewFileAuditSink(dbPath + ".audit.jsonl")
	if err != nil {
		_ = store.Close()
		return nil, nil, err
	}
	service, err := idpaccounts.NewService(store, idpaccounts.Options{Audit: audit})
	if err != nil {
		_ = audit.Close()
		_ = store.Close()
		return nil, nil, err
	}
	return service, func() { _ = audit.Close(); _ = store.Close() }, nil
}

func emitAdminAudit(ctx context.Context, dbPath string, event idp.Event) error {
	sink, err := idp.NewFileAuditSink(dbPath + ".audit.jsonl")
	if err != nil {
		return err
	}
	emitErr := sink.Emit(ctx, event)
	closeErr := sink.Close()
	if emitErr != nil {
		return fmt.Errorf("%w: %v", idp.ErrAuditDelivery, emitErr)
	}
	return closeErr
}

func readAdminPassword(flagValue string, fromStdin bool, r io.Reader) ([]byte, error) {
	if fromStdin {
		b, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		pw := strings.TrimRight(string(b), "\r\n")
		if pw == "" {
			return nil, fmt.Errorf("password from stdin is empty")
		}
		return []byte(pw), nil
	}
	if flagValue == "" {
		return nil, fmt.Errorf("password is required; use --password-from-stdin or --password")
	}
	return []byte(flagValue), nil
}

func writeJSONLine(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
