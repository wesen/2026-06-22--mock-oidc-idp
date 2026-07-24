package cmds

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"go.opentelemetry.io/otel"
	"golang.org/x/sync/errgroup"

	"github.com/go-go-golems/tiny-idp/internal/adminweb"
	"github.com/go-go-golems/tiny-idp/internal/observability"
	"github.com/go-go-golems/tiny-idp/internal/pluginapi"
	"github.com/go-go-golems/tiny-idp/internal/pluginhost"
	"github.com/go-go-golems/tiny-idp/internal/pluginhost/oidcbroker"
	"github.com/go-go-golems/tiny-idp/internal/productionconfig"
	"github.com/go-go-golems/tiny-idp/internal/productionui"
	productionsection "github.com/go-go-golems/tiny-idp/internal/sections/production"
	"github.com/go-go-golems/tiny-idp/pkg/embeddedidp"
	"github.com/go-go-golems/tiny-idp/pkg/idp"
	"github.com/go-go-golems/tiny-idp/pkg/idpaccounts"
	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminapp"
	"github.com/go-go-golems/tiny-idp/pkg/idpemailchallenge"
	"github.com/go-go-golems/tiny-idp/pkg/idpemailchallenge/smtpmailer"
	"github.com/go-go-golems/tiny-idp/pkg/idpinvite"
	"github.com/go-go-golems/tiny-idp/pkg/idpprogram"
	"github.com/go-go-golems/tiny-idp/pkg/idpsignup"
	idpstore "github.com/go-go-golems/tiny-idp/pkg/idpstore"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
)

type ServeProductionCommand struct {
	*cmds.CommandDescription
	registry *pluginapi.Registry
}

const maxProductionSignupProgramBytes = 256 << 10

func NewServeProductionCommand(registry *pluginapi.Registry) (*ServeProductionCommand, error) {
	if registry == nil {
		return nil, fmt.Errorf("plugin registry is required")
	}
	productionSettings, err := productionsection.NewSection()
	if err != nil {
		return nil, err
	}
	commandSettings, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}
	sections := []schema.Section{productionSettings}
	for _, definition := range registry.Definitions() {
		section, sectionErr := definition.Section()
		if sectionErr != nil {
			return nil, sectionErr
		}
		sections = append(sections, section)
	}
	sections = append(sections, commandSettings)
	description := cmds.NewCommandDescription(
		"serve-production",
		cmds.WithShort("Run the durable production embedding host"),
		cmds.WithLong(`Run tiny-idp with the public embedded API, durable SQLite and audit stores,
bounded requests, an explicit listener mode, maintenance, and graceful shutdown.

This command intentionally reads no token secret from an environment variable
or command-line value. Put at least 32 random bytes in an owner-only file and
pass its path with --token-secret-file. Provision the database with the admin
commands before startup. The required --signup-program-file is reviewed,
non-secret JavaScript; startup checks and warms it before accepting traffic.

Example:
  tinyidp serve-production --addr :8443 --issuer https://idp.example.test \
    --db /var/lib/tinyidp/idp.db --audit-path /var/log/tinyidp/audit.jsonl \
    --token-secret-file /run/secrets/tinyidp-token \
    --admin-auth-key-file /run/secrets/tinyidp-admin-auth \
    --admin-action-key-file /run/secrets/tinyidp-admin-actions \
    --clients-file /etc/tinyidp/catalog/clients.json \
    --theme-dir /etc/tinyidp/themes \
    --theme-catalog-file /etc/tinyidp/themes/themes.json \
    --signup-program-file /etc/tinyidp/signup.js \
    --tls-cert /run/tls/tls.crt --tls-key /run/tls/tls.key
`),
		cmds.WithSections(sections...),
	)
	return &ServeProductionCommand{CommandDescription: description, registry: registry}, nil
}

func (c *ServeProductionCommand) Run(ctx context.Context, vals *values.Values) error {
	settings, err := productionsection.GetSettings(vals)
	if err != nil {
		return err
	}
	prepared, err := pluginhost.Prepare(ctx, c.registry, vals)
	if err != nil {
		return err
	}
	return runProductionHost(ctx, settings, prepared)
}

func runProductionHost(ctx context.Context, settings *productionsection.Settings, prepared []pluginapi.Prepared) error {
	if settings == nil {
		return fmt.Errorf("settings are required")
	}
	rateWindow, maintenanceInterval, readHeaderTimeout, readTimeout, writeTimeout, idleTimeout, shutdownTimeout, err := parseProductionDurations(settings)
	if err != nil {
		return err
	}
	if settings.RateLimit <= 0 || settings.MaxRequestBytes <= 0 {
		return fmt.Errorf("rate-limit and max-request-bytes must be positive")
	}
	if strings.TrimSpace(settings.AdminAddr) == "" {
		return fmt.Errorf("--admin-addr is required")
	}
	if settings.AdminAddr == settings.Addr {
		return fmt.Errorf("--admin-addr must differ from --addr")
	}
	listenerMode, err := parseProductionListenerMode(settings.ListenerMode)
	if err != nil {
		return err
	}
	if err := validateProductionListenerSettings(listenerMode, settings); err != nil {
		return err
	}
	secret, err := readOwnerOnlySecret(settings.TokenSecretFile)
	if err != nil {
		return err
	}
	defer clearProductionSecret(secret)
	adminAuthKey, err := readOwnerOnlyFile(settings.AdminAuthKeyFile, "admin auth key", 32)
	if err != nil {
		return err
	}
	defer clearProductionSecret(adminAuthKey)
	if len(adminAuthKey) != 32 {
		return fmt.Errorf("admin auth key file must contain exactly 32 bytes")
	}
	adminActionKey, err := readOwnerOnlyFile(settings.AdminActionKeyFile, "admin action key", 32)
	if err != nil {
		return err
	}
	defer clearProductionSecret(adminActionKey)
	signupSource, err := readProductionSignupProgram(settings.SignupProgramFile)
	if err != nil {
		return err
	}
	signupArtifact, err := idpsignup.Compile(ctx, signupSource)
	if err != nil {
		return fmt.Errorf("check signup program: %w", err)
	}
	signupProgram := signupArtifact.Program()
	var invitationLookupKey []byte
	if productionProgramRequiresDurableInvitations(signupProgram) {
		invitationLookupKey, err = readOwnerOnlySecret(settings.InvitationKeyFile)
		if err != nil {
			return fmt.Errorf("read invitation lookup key: %w", err)
		}
		defer clearProductionSecret(invitationLookupKey)
	}
	clientCatalog, err := productionconfig.LoadClientCatalog(settings.ClientsFile)
	if err != nil {
		return err
	}
	clientSpecs := clientCatalog.Specs()
	clients := make([]idpstore.Client, len(clientSpecs))
	for index := range clientSpecs {
		clients[index] = clientSpecs[index].Client
	}
	if err := pluginhost.ValidateClientRequirements(prepared, clients); err != nil {
		return err
	}
	themeCatalog, err := productionui.LoadCatalog(settings.ThemeDir, settings.ThemeCatalogFile, clientCatalog)
	if err != nil {
		return err
	}
	interactionUI, err := productionui.NewRenderer(themeCatalog)
	if err != nil {
		return err
	}
	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(settings.DBPath))
	if err != nil {
		return err
	}
	var durableInvitations *idpinvite.DurableService
	if len(invitationLookupKey) != 0 {
		durableInvitations, err = idpinvite.NewDurableService(store, invitationLookupKey)
		if err != nil {
			_ = store.Close()
			return fmt.Errorf("construct durable invitation service: %w", err)
		}
		clearProductionSecret(invitationLookupKey)
	}
	emailChallenges, err := newProductionEmailChallenges(settings, store, signupProgram)
	if err != nil {
		_ = store.Close()
		return err
	}
	audit, err := idp.NewFileAuditSink(settings.AuditPath)
	if err != nil {
		_ = store.Close()
		return err
	}
	signupManager, err := newProductionSignupManager(ctx, signupSource, audit, productionSignupServices{EmailChallenges: emailChallenges != nil, DisplayNameLookup: true})
	if err != nil {
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	if _, err := embeddedidp.Bootstrap(ctx, store, embeddedidp.BootstrapConfig{Mode: idpstore.ProductionMode, Audit: audit, Clients: clientCatalog.Specs()}); err != nil {
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return fmt.Errorf("bootstrap production browser clients: %w", err)
	}
	addressResolver := idp.ClientAddressResolver(idp.DirectClientAddressResolver{})
	var proxyResolver *idp.TrustedProxyResolver
	if listenerMode == productionListenerTrustedProxyHTTP {
		proxyResolver, err = idp.NewTrustedProxyResolver(idp.TrustedProxyConfig{TrustedCIDRs: settings.TrustedProxyCIDRs, MaxHops: settings.MaxProxyHops})
		if err != nil {
			_ = signupManager.Close(context.Background())
			_ = audit.Close()
			_ = store.Close()
			return err
		}
		addressResolver = proxyResolver
	}
	provider, err := embeddedidp.New(ctx, embeddedidp.Options{
		Issuer:        settings.Issuer,
		Mode:          embeddedidp.ProductionMode,
		Store:         store,
		Cookie:        embeddedidp.CookieConfig{Secure: true, SameSite: http.SameSiteLaxMode},
		Token:         embeddedidp.TokenConfig{SecretKey: secret},
		Audit:         audit,
		RateLimiter:   idp.NewFixedWindowRateLimiter(settings.RateLimit, rateWindow),
		ClientAddress: addressResolver,
		ScriptedSignup: embeddedidp.ScriptedSignupConfig{
			GenerationManager:  signupManager,
			DurableInvitations: durableInvitations,
			EmailChallenges:    emailChallenges,
		},
		Maintenance: embeddedidp.MaintenanceConfig{Interval: maintenanceInterval},
		UI:          embeddedidp.UIConfig{Renderer: interactionUI, WorkflowRenderer: interactionUI},
		AccountChooser: embeddedidp.AccountChooserConfig{
			Enabled:                 settings.AccountChooser,
			RememberOnPasswordLogin: settings.AccountChooser,
			DisplayLabel: func(user idpstore.User) (string, error) {
				if label := strings.TrimSpace(user.Name); label != "" {
					return label, nil
				}
				return user.PreferredUsername, nil
			},
		},
	})
	if err != nil {
		clearProductionSecret(secret)
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	transactionManager, err := oidcbroker.NewTransactionManager(store.SQLDB(), secret, rand.Reader, time.Now)
	clearProductionSecret(secret)
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	broker, err := oidcbroker.New(ctx, settings.Issuer, provider.Handler(), transactionManager)
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	publicOrigin, err := issuerOrigin(settings.Issuer)
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	issuerTransport, err := embeddedidp.NewInProcessIssuerTransport(settings.Issuer, provider.Handler(), embeddedidp.InProcessTransportOptions{})
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return fmt.Errorf("construct admin issuer transport: %w", err)
	}
	oauthFlow, identityVerifier, err := adminweb.NewOIDCFlow(
		ctx,
		settings.Issuer,
		publicOrigin+"/admin/auth/callback",
		&http.Client{Transport: issuerTransport},
	)
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	adminAuth, err := adminweb.NewAuthManager(adminweb.AuthConfig{
		Store: store, OAuth: oauthFlow, Verifier: identityVerifier,
		SecretKey: adminAuthKey, PublicOrigin: publicOrigin, Secure: true,
	})
	clearProductionSecret(adminAuthKey)
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	if _, err := store.RebuildAdminUserProjection(ctx, time.Now().UTC()); err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return fmt.Errorf("rebuild admin user projection: %w", err)
	}
	adminAuthorizer, err := idpadmin.NewAuthorizer(store, 5*time.Minute, time.Now)
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	adminHandles, err := idpadmin.NewHandleService(adminActionKey, 5*time.Minute, time.Now)
	clearProductionSecret(adminActionKey)
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	adminExecutor, err := idpadminapp.NewExecutor(store, adminHandles, adminAuthorizer, time.Now)
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	adminActions, err := idpadminapp.NewActionService(store, adminHandles, adminAuthorizer, time.Now)
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	adminUsers, err := idpadminapp.NewUserCommandService(store, adminExecutor, idpaccounts.Options{}, time.Now)
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	adminPages, err := idpadminapp.NewPageDataService(store, adminAuthorizer, time.Now)
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	adminWidgets, err := adminweb.NewWidgetRuntime()
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	publicAdminHandler, err := adminweb.NewHandler(adminweb.HandlerConfig{
		Auth: adminAuth, Pages: adminPages, Widgets: adminWidgets,
		Actions: adminActions, Users: adminUsers,
		SPA: adminweb.SPAHandler(), Assets: adminweb.AssetsHandler(),
	})
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	telemetry, err := observability.NewMetrics()
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return fmt.Errorf("construct production metrics: %w", err)
	}
	defer func() {
		_ = telemetry.Close(context.Background())
	}()
	runtimes, err := pluginhost.Build(ctx, prepared, pluginapi.RuntimeServices{
		OIDC: broker, Secrets: pluginhost.FileSecretResolver{}, Audit: audit,
		Logger: log.Raw(), Meter: telemetry.Provider().Meter("tinyidp/plugins"), Tracer: otel.Tracer("tinyidp/plugins"),
		Clock: pluginhost.SystemClock{}, Random: rand.Reader,
	})
	if err != nil {
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	if _, err := provider.RunMaintenance(ctx); err != nil {
		_ = pluginhost.Close(context.Background(), runtimes)
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return fmt.Errorf("initial maintenance: %w", err)
	}
	issuerURL, err := url.Parse(settings.Issuer)
	if err != nil {
		_ = pluginhost.Close(context.Background(), runtimes)
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	readyPath := strings.TrimSuffix(issuerURL.Path, "/") + "/readyz"
	handler, err := productionHTTPHandler(provider.Handler(), themeCatalog.AssetsHandler(), publicAdminHandler, runtimes, readyPath, func(readinessCtx context.Context) idp.ReadinessReport {
		return pluginhost.CombineReadiness(provider.Readiness(readinessCtx), pluginhost.Readiness(readinessCtx, runtimes))
	}, settings.MaxRequestBytes)
	if err != nil {
		_ = pluginhost.Close(context.Background(), runtimes)
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	if listenerMode == productionListenerTrustedProxyHTTP {
		handler, err = idp.NewTrustedProxyHTTPHandler(idp.TrustedProxyHTTPConfig{PublicOrigin: publicOrigin, Resolver: proxyResolver}, handler)
		if err != nil {
			_ = pluginhost.Close(context.Background(), runtimes)
			_ = provider.Close(context.Background())
			_ = signupManager.Close(context.Background())
			_ = audit.Close()
			_ = store.Close()
			return err
		}
	}
	httpServer := &http.Server{
		Addr:              settings.Addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    1 << 20,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
	}
	adminHandler, err := observability.NewAdminHandler(telemetry.Handler(), func(readinessCtx context.Context) idp.ReadinessReport {
		return pluginhost.CombineReadiness(provider.Readiness(readinessCtx), pluginhost.Readiness(readinessCtx, runtimes))
	})
	if err != nil {
		_ = pluginhost.Close(context.Background(), runtimes)
		_ = provider.Close(context.Background())
		_ = signupManager.Close(context.Background())
		_ = audit.Close()
		_ = store.Close()
		return err
	}
	adminServer := &http.Server{
		Addr:              settings.AdminAddr,
		Handler:           adminHandler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    1 << 20,
	}
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		log.Info().Str("addr", settings.Addr).Str("issuer", settings.Issuer).Str("listener_mode", string(listenerMode)).Msg("tinyidp production host listening")
		var serveErr error
		if listenerMode == productionListenerDirectTLS {
			serveErr = httpServer.ListenAndServeTLS(settings.TLSCertFile, settings.TLSKeyFile)
		} else {
			serveErr = httpServer.ListenAndServe()
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serve production listener: %w", serveErr)
		}
		return nil
	})
	group.Go(func() error {
		log.Info().Str("addr", settings.AdminAddr).Msg("tinyidp internal administration listener active")
		serveErr := adminServer.ListenAndServe()
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serve production administration listener: %w", serveErr)
		}
		return nil
	})
	group.Go(func() error {
		ticker := time.NewTicker(maintenanceInterval)
		defer ticker.Stop()
		for {
			select {
			case <-groupCtx.Done():
				return nil
			case <-ticker.C:
				if _, err := provider.RunMaintenance(groupCtx); err != nil && groupCtx.Err() == nil {
					log.Error().Err(err).Msg("retention maintenance failed; readiness is degraded")
				}
			}
		}
	})
	group.Go(func() error {
		<-groupCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return errors.Join(httpServer.Shutdown(shutdownCtx), adminServer.Shutdown(shutdownCtx))
	})
	runErr := group.Wait()
	closeErr := errors.Join(pluginhost.Close(context.Background(), runtimes), provider.Close(context.Background()), signupManager.Close(context.Background()), audit.Close(), store.Close())
	return errors.Join(runErr, closeErr)
}

// productionHTTPHandler keeps reviewed interaction assets on the provider's
// own origin. Every OAuth/OIDC and form route remains owned by the embedded
// provider handler.
func productionHTTPHandler(providerHandler, assetsHandler, adminHandler http.Handler, runtimes []pluginapi.Runtime, readyPath string, readiness func(context.Context) idp.ReadinessReport, maxRequestBytes int) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.Handle("/static/themes/", assetsHandler)
	if adminHandler != nil {
		mux.Handle("/admin", adminHandler)
		mux.Handle("/admin/", adminHandler)
		mux.Handle("/api/admin/", adminHandler)
		mux.Handle("/api/widget/", adminHandler)
		mux.Handle("/static/admin/", adminHandler)
	}
	if err := pluginhost.Mount(mux, runtimes); err != nil {
		return nil, err
	}
	if readyPath != "" && readiness != nil {
		mux.HandleFunc(readyPath, func(writer http.ResponseWriter, request *http.Request) {
			report := readiness(request.Context())
			writer.Header().Set("Content-Type", "application/json")
			status := http.StatusOK
			if !report.Ready {
				status = http.StatusServiceUnavailable
			}
			writer.WriteHeader(status)
			_ = json.NewEncoder(writer).Encode(report)
		})
	}
	mux.Handle("/", providerHandler)
	return http.MaxBytesHandler(mux, int64(maxRequestBytes)), nil
}

func clearProductionSecret(secret []byte) {
	for i := range secret {
		secret[i] = 0
	}
}

func readProductionSignupProgram(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("--signup-program-file is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open signup program file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat signup program file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("signup program file must be a regular file")
	}
	if info.Size() > maxProductionSignupProgramBytes {
		return "", fmt.Errorf("signup program file exceeds %d bytes", maxProductionSignupProgramBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxProductionSignupProgramBytes+1))
	if err != nil {
		return "", fmt.Errorf("read signup program file: %w", err)
	}
	if len(data) > maxProductionSignupProgramBytes {
		return "", fmt.Errorf("signup program file exceeds %d bytes", maxProductionSignupProgramBytes)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return "", fmt.Errorf("signup program file must not be empty")
	}
	return string(data), nil
}

type productionSignupServices struct {
	EmailChallenges   bool
	DisplayNameLookup bool
}

func newProductionSignupManager(ctx context.Context, source string, audit idp.Sink, services productionSignupServices) (*idpsignup.GenerationManager, error) {
	_, err := checkProductionSignupProgram(ctx, source, services)
	if err != nil {
		return nil, err
	}
	manager, err := idpsignup.NewGenerationManagerWithOptions(ctx, source, 1, 1, idpsignup.GenerationManagerOptions{Audit: audit})
	if err != nil {
		return nil, fmt.Errorf("activate signup program: %w", err)
	}
	if err := manager.Ready(); err != nil {
		_ = manager.Close(context.Background())
		return nil, fmt.Errorf("active signup program is unavailable: %w", err)
	}
	return manager, nil
}

func checkProductionSignupProgram(ctx context.Context, source string, services productionSignupServices) (idpprogram.Program, error) {
	artifact, err := idpsignup.Compile(ctx, source)
	if err != nil {
		return idpprogram.Program{}, fmt.Errorf("check signup program: %w", err)
	}
	program := artifact.Program()
	if err := validateProductionSignupProgram(program, services); err != nil {
		return idpprogram.Program{}, err
	}
	return program, nil
}

func validateProductionSignupProgram(program idpprogram.Program, services productionSignupServices) error {
	unsupportedCapabilities := make([]string, 0)
	for id, requirement := range program.Capabilities {
		supported := id == idpinvite.LookupCapabilityID && requirement.Version == idpinvite.LookupCapabilityVersion ||
			id == idpaccounts.DisplayNameLookupCapabilityID && requirement.Version == idpaccounts.DisplayNameLookupCapabilityVersion && services.DisplayNameLookup
		if !supported {
			unsupportedCapabilities = append(unsupportedCapabilities, id)
		}
	}
	if len(unsupportedCapabilities) != 0 {
		sort.Strings(unsupportedCapabilities)
		return fmt.Errorf("signup program declares unsupported native capabilities: %s", strings.Join(unsupportedCapabilities, ", "))
	}
	if err := validateProductionSignupCapabilityBindings(program, services); err != nil {
		return err
	}
	durableProvider := false
	for _, provider := range program.Providers {
		if provider.Kind != idpprogram.ProviderKindInvitation || provider.State != idpprogram.ProviderStateDurable {
			continue
		}
		durableProvider = true
		handler, ok := provider.Handlers[idpprogram.InvitationValidateHandler]
		lambda, lambdaOK := program.Lambdas[handler.LambdaID]
		if !ok || !lambdaOK || !lambdaRequiresCapability(lambda, idpinvite.LookupCapabilityID, idpinvite.LookupCapabilityVersion) {
			return fmt.Errorf("durable invitation provider %q must bind validate to invitation.lookup@v1", provider.ID)
		}
	}
	if _, declared := program.Capabilities[idpinvite.LookupCapabilityID]; declared && !durableProvider {
		return fmt.Errorf("signup program declares invitation.lookup without a durable invitation provider")
	}
	unsupported := map[string]struct{}{}
	usesInvitationEffect := false
	for _, lambda := range program.Lambdas {
		for _, outcome := range lambda.AllowedOutcomes {
			if outcome == idpprogram.OutcomeChallenge && !services.EmailChallenges {
				unsupported["email_challenge"] = struct{}{}
			}
		}
		for _, effect := range lambda.AllowedEffects {
			if effect == idpprogram.EffectConsumeInvitation {
				usesInvitationEffect = true
				continue
			}
			if effect != idpprogram.EffectCreateLocalIdentity && effect != idpprogram.EffectAttachPasswordCredential {
				unsupported["effect:"+string(effect)] = struct{}{}
			}
		}
	}
	if usesInvitationEffect && !durableProvider {
		unsupported["effect:consumeInvitation_without_durable_provider"] = struct{}{}
	}
	if durableProvider && !usesInvitationEffect {
		unsupported["durable_invitation_provider_without_consumeInvitation"] = struct{}{}
	}
	if len(unsupported) == 0 {
		return nil
	}
	ids := make([]string, 0, len(unsupported))
	for id := range unsupported {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return fmt.Errorf("signup program declares unsupported native services: %s", strings.Join(ids, ", "))
}

func validateProductionSignupCapabilityBindings(program idpprogram.Program, services productionSignupServices) error {
	unsupported := make([]string, 0)
	for workflowID, workflow := range program.Workflows {
		for handlerID, handler := range workflow.Handlers {
			lambda := program.Lambdas[handler.LambdaID]
			for _, requirement := range lambda.RequiredCapabilities {
				if handlerID == workflow.EntryHandler || requirement.ID != idpaccounts.DisplayNameLookupCapabilityID || requirement.Version != idpaccounts.DisplayNameLookupCapabilityVersion || !services.DisplayNameLookup {
					unsupported = append(unsupported, fmt.Sprintf("workflow %s handler %s: %s@v%d", workflowID, handlerID, requirement.ID, requirement.Version))
				}
			}
		}
	}
	for providerID, provider := range program.Providers {
		for handlerID, handler := range provider.Handlers {
			lambda := program.Lambdas[handler.LambdaID]
			for _, requirement := range lambda.RequiredCapabilities {
				if provider.Kind != idpprogram.ProviderKindInvitation || provider.State != idpprogram.ProviderStateDurable || requirement.ID != idpinvite.LookupCapabilityID || requirement.Version != idpinvite.LookupCapabilityVersion {
					unsupported = append(unsupported, fmt.Sprintf("provider %s handler %s: %s@v%d", providerID, handlerID, requirement.ID, requirement.Version))
				}
			}
		}
	}
	if len(unsupported) == 0 {
		return nil
	}
	sort.Strings(unsupported)
	return fmt.Errorf("signup program requires capabilities unavailable on their invocation paths: %s", strings.Join(unsupported, ", "))
}

func productionProgramRequiresDurableInvitations(program idpprogram.Program) bool {
	for _, provider := range program.Providers {
		if provider.Kind == idpprogram.ProviderKindInvitation && provider.State == idpprogram.ProviderStateDurable {
			return true
		}
	}
	return false
}

func productionProgramRequiresEmailChallenges(program idpprogram.Program) bool {
	for _, lambda := range program.Lambdas {
		for _, outcome := range lambda.AllowedOutcomes {
			if outcome == idpprogram.OutcomeChallenge {
				return true
			}
		}
	}
	return false
}

func newProductionEmailChallenges(settings *productionsection.Settings, store idpemailchallenge.Store, program idpprogram.Program) (*idpemailchallenge.Service, error) {
	required := productionProgramRequiresEmailChallenges(program)
	configured := strings.TrimSpace(settings.EmailChallengeKeyFile) != "" || strings.TrimSpace(settings.EmailSMTPAddress) != "" || strings.TrimSpace(settings.EmailSMTPTLSMode) != "" || strings.TrimSpace(settings.EmailSMTPServerName) != "" || strings.TrimSpace(settings.EmailSMTPUsername) != "" || strings.TrimSpace(settings.EmailSMTPPasswordFile) != "" || strings.TrimSpace(settings.EmailFromAddress) != ""
	if !required {
		if configured {
			return nil, errors.New("email delivery flags require a signup program that declares an email challenge")
		}
		return nil, nil
	}
	if store == nil {
		return nil, errors.New("email challenge signup requires a durable challenge store")
	}
	if strings.TrimSpace(settings.EmailChallengeKeyFile) == "" || strings.TrimSpace(settings.EmailSMTPAddress) == "" || strings.TrimSpace(settings.EmailSMTPTLSMode) == "" || strings.TrimSpace(settings.EmailFromAddress) == "" {
		return nil, errors.New("email challenge signup requires --email-challenge-key-file, --email-smtp-address, --email-smtp-tls-mode, and --email-from-address")
	}
	connectTimeout, err := positiveDurationFlag("email-smtp-connect-timeout", settings.EmailSMTPConnectTimeout)
	if err != nil {
		return nil, err
	}
	sendTimeout, err := positiveDurationFlag("email-smtp-send-timeout", settings.EmailSMTPSendTimeout)
	if err != nil {
		return nil, err
	}
	key, err := readOwnerOnlyFile(settings.EmailChallengeKeyFile, "email challenge key", 32)
	if err != nil {
		return nil, err
	}
	defer clearProductionSecret(key)
	var password []byte
	if strings.TrimSpace(settings.EmailSMTPUsername) != "" || strings.TrimSpace(settings.EmailSMTPPasswordFile) != "" {
		if strings.TrimSpace(settings.EmailSMTPUsername) == "" || strings.TrimSpace(settings.EmailSMTPPasswordFile) == "" {
			return nil, errors.New("--email-smtp-username and --email-smtp-password-file must be configured together")
		}
		password, err = readOwnerOnlyFile(settings.EmailSMTPPasswordFile, "SMTP password", 1)
		if err != nil {
			return nil, err
		}
		defer clearProductionSecret(password)
	}
	mailer, err := smtpmailer.New(smtpmailer.Config{
		Address: settings.EmailSMTPAddress, TLSMode: smtpmailer.TLSMode(settings.EmailSMTPTLSMode), ServerName: settings.EmailSMTPServerName,
		Username: settings.EmailSMTPUsername, Password: password, FromAddress: settings.EmailFromAddress, FromName: settings.EmailFromName,
		ConnectTimeout: connectTimeout, SendTimeout: sendTimeout, Templates: smtpmailer.SignupTemplates(),
	})
	if err != nil {
		return nil, fmt.Errorf("construct SMTP email challenge mailer: %w", err)
	}
	service, err := idpemailchallenge.NewService(store, mailer, key)
	if err != nil {
		return nil, fmt.Errorf("construct durable email challenge service: %w", err)
	}
	return service, nil
}

func positiveDurationFlag(name, raw string) (time.Duration, error) {
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid --%s duration %q", name, raw)
	}
	return duration, nil
}

func lambdaRequiresCapability(lambda idpprogram.LambdaSpec, id string, version uint32) bool {
	for _, requirement := range lambda.RequiredCapabilities {
		if requirement.ID == id && requirement.Version == version {
			return true
		}
	}
	return false
}

type productionListenerMode string

const (
	productionListenerDirectTLS        productionListenerMode = "direct-tls"
	productionListenerTrustedProxyHTTP productionListenerMode = "trusted-proxy-http"
)

func parseProductionListenerMode(raw string) (productionListenerMode, error) {
	mode := productionListenerMode(strings.TrimSpace(raw))
	if mode != productionListenerDirectTLS && mode != productionListenerTrustedProxyHTTP {
		return "", fmt.Errorf("--listener-mode must be direct-tls or trusted-proxy-http")
	}
	return mode, nil
}

func validateProductionListenerSettings(mode productionListenerMode, settings *productionsection.Settings) error {
	if mode == productionListenerDirectTLS {
		if settings.TLSCertFile == "" || settings.TLSKeyFile == "" || len(settings.TrustedProxyCIDRs) != 0 {
			return fmt.Errorf("direct-tls requires --tls-cert and --tls-key and forbids --trusted-proxy-cidrs")
		}
		return nil
	}
	issuer, err := url.Parse(settings.Issuer)
	if err != nil || issuer.Scheme != "https" || issuer.Host == "" || len(settings.TrustedProxyCIDRs) == 0 || settings.TLSCertFile != "" || settings.TLSKeyFile != "" {
		return fmt.Errorf("trusted-proxy-http requires an HTTPS issuer and --trusted-proxy-cidrs and forbids TLS certificate flags")
	}
	return nil
}

func issuerOrigin(raw string) (string, error) {
	issuer, err := url.Parse(raw)
	if err != nil || issuer.Scheme != "https" || issuer.Host == "" {
		return "", fmt.Errorf("issuer must have an HTTPS origin")
	}
	return issuer.Scheme + "://" + issuer.Host, nil
}

func parseProductionDurations(settings *productionsection.Settings) (time.Duration, time.Duration, time.Duration, time.Duration, time.Duration, time.Duration, time.Duration, error) {
	values := []struct {
		name string
		raw  string
	}{
		{"rate-window", settings.RateWindow}, {"maintenance-interval", settings.MaintenanceInterval},
		{"read-header-timeout", settings.ReadHeaderTimeout}, {"read-timeout", settings.ReadTimeout},
		{"write-timeout", settings.WriteTimeout}, {"idle-timeout", settings.IdleTimeout},
		{"shutdown-timeout", settings.ShutdownTimeout},
	}
	parsed := make([]time.Duration, len(values))
	for index, value := range values {
		duration, err := time.ParseDuration(value.raw)
		if err != nil || duration <= 0 {
			return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid --%s duration %q", value.name, value.raw)
		}
		parsed[index] = duration
	}
	return parsed[0], parsed[1], parsed[2], parsed[3], parsed[4], parsed[5], parsed[6], nil
}

func readOwnerOnlySecret(path string) ([]byte, error) {
	return readOwnerOnlyFile(path, "token secret", 32)
}

func readOwnerOnlyFile(path, label string, minimumBytes int) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("%s file is required", label)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s file: %w", label, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s file must be regular and owner-only (0600 or 0400)", label)
	}
	secret, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s file: %w", label, err)
	}
	secret = bytes.TrimSuffix(secret, []byte("\n"))
	if len(secret) < minimumBytes {
		return nil, fmt.Errorf("%s file must contain at least %d bytes", label, minimumBytes)
	}
	return secret, nil
}
