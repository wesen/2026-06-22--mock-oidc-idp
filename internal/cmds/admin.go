package cmds

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/go-go-golems/tiny-idp/internal/admin"
	"github.com/go-go-golems/tiny-idp/pkg/idp"
	"github.com/go-go-golems/tiny-idp/pkg/idpaccounts"
	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
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
	var actionKeyFile string
	cmd.PersistentFlags().StringVar(&actionKeyFile, "admin-action-key-file", "", "Owner-only action-handle key file")
	cmd.AddCommand(newAdminUserCreateCommand(dbPath, &actionKeyFile))
	cmd.AddCommand(newAdminUserSetPasswordCommand(dbPath, &actionKeyFile))
	cmd.AddCommand(newAdminUserGetCommand(dbPath))
	cmd.AddCommand(newAdminUserDisableCommand(dbPath, &actionKeyFile, true))
	cmd.AddCommand(newAdminUserDisableCommand(dbPath, &actionKeyFile, false))
	return cmd
}

func newAdminUserCreateCommand(dbPath, actionKeyFile *string) *cobra.Command {
	var login, password, email, name string
	var emailVerified, passwordFromStdin bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a user and password credential",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pw, err := readAdminPassword(password, passwordFromStdin, cmd.InOrStdin())
			if err != nil {
				return err
			}
			defer clearProductionSecret(pw)
			runtime, err := openAdminUserRuntime(cmd.Context(), *dbPath, *actionKeyFile)
			if err != nil {
				return err
			}
			defer runtime.close()
			result, err := runtime.execute(cmd.Context(), idpadminapp.CommandUsersCreate, "", map[string]any{
				"login": login, "password": string(pw), "email": email,
				"email_verified": emailVerified, "display_name": name,
			})
			if err != nil {
				return err
			}
			return writeJSONLine(cmd.OutOrStdout(), map[string]any{"status": "created", "user": result.User})
		},
	}
	cmd.Flags().StringVar(&login, "login", "", "Login name")
	cmd.Flags().StringVar(&password, "password", "", "Password value (prefer --password-from-stdin outside tests)")
	cmd.Flags().BoolVar(&passwordFromStdin, "password-from-stdin", false, "Read password from stdin")
	cmd.Flags().StringVar(&email, "email", "", "Email claim")
	cmd.Flags().BoolVar(&emailVerified, "email-verified", false, "Set email_verified claim")
	cmd.Flags().StringVar(&name, "name", "", "Display name")
	_ = cmd.MarkFlagRequired("login")
	return cmd
}

func newAdminUserSetPasswordCommand(dbPath, actionKeyFile *string) *cobra.Command {
	var login, password, reason string
	var passwordFromStdin bool
	cmd := &cobra.Command{
		Use:   "set-password",
		Short: "Set or replace a user's password credential",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pw, err := readAdminPassword(password, passwordFromStdin, cmd.InOrStdin())
			if err != nil {
				return err
			}
			defer clearProductionSecret(pw)
			runtime, err := openAdminUserRuntime(cmd.Context(), *dbPath, *actionKeyFile)
			if err != nil {
				return err
			}
			defer runtime.close()
			user, err := runtime.store.GetUserByLogin(cmd.Context(), login)
			if err != nil {
				return err
			}
			if _, err := runtime.execute(cmd.Context(), idpadminapp.CommandUsersSetPassword, user.ID, map[string]any{
				"password": string(pw), "reason": reason,
			}); err != nil {
				return err
			}
			return writeJSONLine(cmd.OutOrStdout(), map[string]any{"status": "password-updated", "login": login})
		},
	}
	cmd.Flags().StringVar(&login, "login", "", "Login name")
	cmd.Flags().StringVar(&password, "password", "", "Password value (prefer --password-from-stdin outside tests)")
	cmd.Flags().BoolVar(&passwordFromStdin, "password-from-stdin", false, "Read password from stdin")
	cmd.Flags().StringVar(&reason, "reason", "", "Required operator reason")
	_ = cmd.MarkFlagRequired("login")
	_ = cmd.MarkFlagRequired("reason")
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

func newAdminUserDisableCommand(dbPath, actionKeyFile *string, disabled bool) *cobra.Command {
	name := "enable"
	status := "enabled"
	shortVerb := "Enable"
	if disabled {
		name = "disable"
		status = "disabled"
		shortVerb = "Disable"
	}
	var login, reason, confirmation string
	cmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("%s a user", shortVerb),
		RunE: func(cmd *cobra.Command, _ []string) error {
			runtime, err := openAdminUserRuntime(cmd.Context(), *dbPath, *actionKeyFile)
			if err != nil {
				return err
			}
			defer runtime.close()
			user, err := runtime.store.GetUserByLogin(cmd.Context(), login)
			if err != nil {
				return err
			}
			command := idpadminapp.CommandUsersEnable
			if disabled {
				command = idpadminapp.CommandUsersDisable
			}
			result, err := runtime.execute(cmd.Context(), command, user.ID, map[string]any{
				"reason": reason, "confirmation": confirmation,
			})
			if err != nil {
				return err
			}
			return writeJSONLine(cmd.OutOrStdout(), map[string]any{"status": status, "user": result.User})
		},
	}
	cmd.Flags().StringVar(&login, "login", "", "Login name")
	cmd.Flags().StringVar(&reason, "reason", "", "Required operator reason")
	cmd.Flags().StringVar(&confirmation, "confirm", "", "Typed confirmation (DISABLE when disabling)")
	_ = cmd.MarkFlagRequired("login")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}

type adminUserRuntime struct {
	store     *sqlitestore.Store
	actions   *idpadminapp.ActionService
	users     *idpadminapp.UserCommandService
	principal idpadmin.AdminPrincipal
}

func openAdminUserRuntime(ctx context.Context, dbPath, actionKeyFile string) (*adminUserRuntime, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, fmt.Errorf("--db is required")
	}
	key, err := readOwnerOnlyFile(actionKeyFile, "admin action key", 32)
	if err != nil {
		return nil, err
	}
	defer clearProductionSecret(key)
	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(dbPath))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if _, err := store.RebuildAdminUserProjection(ctx, now); err != nil {
		_ = store.Close()
		return nil, err
	}
	grant, err := store.GetActiveSystemOwner(ctx, now)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("load active console owner: %w", err)
	}
	sessionBinding, err := adminCLIRandomID()
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	clock := time.Now
	handles, err := idpadmin.NewHandleService(key, 5*time.Minute, clock)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	authorizer, err := idpadmin.NewAuthorizer(store, 5*time.Minute, clock)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	executor, err := idpadminapp.NewExecutor(store, handles, authorizer, clock)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	actions, err := idpadminapp.NewActionService(store, handles, authorizer, clock)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	users, err := idpadminapp.NewUserCommandService(store, executor, idpaccounts.Options{}, clock)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	return &adminUserRuntime{
		store: store, actions: actions, users: users,
		principal: idpadmin.AdminPrincipal{
			Subject: grant.ActorSubject, SessionID: sessionBinding,
			Authenticated: now, Assurance: idpadmin.AssuranceFresh,
			GrantID: grant.ID, GrantVersion: grant.Version,
		},
	}, nil
}

func (r *adminUserRuntime) close() { _ = r.store.Close() }

func (r *adminUserRuntime) execute(
	ctx context.Context,
	command, targetID string,
	input map[string]any,
) (idpadmin.UserResult, error) {
	prepared, err := r.actions.Prepare(ctx, r.principal, idpadminapp.PrepareActionRequest{
		Command: command, TargetID: targetID,
	})
	if err != nil {
		return idpadmin.UserResult{}, err
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return idpadmin.UserResult{}, err
	}
	hash := sha256.Sum256(raw)
	requestID, err := adminCLIRandomID()
	if err != nil {
		return idpadmin.UserResult{}, err
	}
	response, err := r.users.Execute(ctx, idpadminapp.ExecutionRequest{
		Handle: prepared.Handle, Principal: r.principal, RequestID: requestID,
		IdempotencyKey: requestID, RequestHash: hash[:],
	}, raw)
	if err != nil {
		return idpadmin.UserResult{}, err
	}
	var result idpadmin.UserResult
	err = json.Unmarshal(response, &result)
	return result, err
}

func adminCLIRandomID() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
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
