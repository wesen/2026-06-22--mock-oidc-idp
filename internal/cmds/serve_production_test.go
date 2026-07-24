package cmds

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/tiny-idp/internal/pluginapi"
	jitsiplugin "github.com/go-go-golems/tiny-idp/internal/plugins/jitsi"
	productionsection "github.com/go-go-golems/tiny-idp/internal/sections/production"
	"github.com/go-go-golems/tiny-idp/pkg/idp"
	"github.com/go-go-golems/tiny-idp/pkg/idpaccounts"
	"github.com/go-go-golems/tiny-idp/pkg/idpemailchallenge"
	"github.com/go-go-golems/tiny-idp/pkg/idpinvite"
	"github.com/go-go-golems/tiny-idp/pkg/idpprogram"
	"github.com/go-go-golems/tiny-idp/pkg/idpsignup"
)

func TestReadOwnerOnlySecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token-secret")
	if err := os.WriteFile(path, []byte("0123456789abcdef0123456789abcdef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	secret, err := readOwnerOnlySecret(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) != 32 {
		t.Fatalf("secret length = %d", len(secret))
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readOwnerOnlySecret(path); err == nil {
		t.Fatal("expected permissive secret file rejection")
	}
}

func TestProductionHTTPHandlerServesOnlyTheRendererAssetsBelowStaticThemes(t *testing.T) {
	assets := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/static/themes/message-desk.css" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = writer.Write([]byte("/* Message Desk */"))
	})
	provider := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = writer.Write([]byte("provider route: " + request.URL.Path))
	})
	handler, err := productionHTTPHandler(provider, assets, nil, nil, "", nil, 1024)
	if err != nil {
		t.Fatal(err)
	}

	stylesheet := httptest.NewRecorder()
	handler.ServeHTTP(stylesheet, httptest.NewRequest(http.MethodGet, "https://idp.example.test/static/themes/message-desk.css", nil))
	if stylesheet.Code != http.StatusOK || stylesheet.Body.String() != "/* Message Desk */" {
		t.Fatalf("stylesheet response = %d %q", stylesheet.Code, stylesheet.Body.String())
	}
	if got := stylesheet.Header().Get("Content-Type"); got != "text/css; charset=utf-8" {
		t.Fatalf("stylesheet content type = %q", got)
	}

	providerResponse := httptest.NewRecorder()
	handler.ServeHTTP(providerResponse, httptest.NewRequest(http.MethodGet, "https://idp.example.test/authorize", nil))
	if providerResponse.Code != http.StatusOK || providerResponse.Body.String() != "provider route: /authorize" {
		t.Fatalf("provider response = %d %q", providerResponse.Code, providerResponse.Body.String())
	}
}

func TestProductionHTTPHandlerMountsAdminBeforeProviderFallback(t *testing.T) {
	provider := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("provider:" + request.URL.Path))
	})
	admin := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("admin:" + request.URL.Path))
	})
	handler, err := productionHTTPHandler(provider, http.NotFoundHandler(), admin, nil, "", nil, 1024)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/admin", "/admin/users", "/api/admin/session",
		"/api/widget/pages/overview", "/static/admin/assets/admin.js",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://idp.example.test"+path, nil))
		if got := response.Body.String(); got != "admin:"+path {
			t.Fatalf("%s response = %q", path, got)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://idp.example.test/authorize", nil))
	if got := response.Body.String(); got != "provider:/authorize" {
		t.Fatalf("provider fallback response = %q", got)
	}
}

func TestParseProductionDurationsRejectsNonPositive(t *testing.T) {
	settings := &productionsection.Settings{RateWindow: "1m", MaintenanceInterval: "15m", ReadHeaderTimeout: "5s", ReadTimeout: "15s", WriteTimeout: "30s", IdleTimeout: "1m", ShutdownTimeout: "0s"}
	if _, _, _, _, _, _, _, err := parseProductionDurations(settings); err == nil {
		t.Fatal("expected zero shutdown timeout rejection")
	}
}

func TestProductionListenerModesAreExplicitAndMutuallyExclusive(t *testing.T) {
	if _, err := parseProductionListenerMode(""); err == nil {
		t.Fatal("empty listener mode accepted")
	}
	if _, err := parseProductionListenerMode("plaintext"); err == nil {
		t.Fatal("unknown listener mode accepted")
	}
	direct, err := parseProductionListenerMode("direct-tls")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateProductionListenerSettings(direct, &productionsection.Settings{TLSCertFile: "cert.pem", TLSKeyFile: "key.pem"}); err != nil {
		t.Fatalf("valid direct TLS settings rejected: %v", err)
	}
	if err := validateProductionListenerSettings(direct, &productionsection.Settings{TLSCertFile: "cert.pem", TLSKeyFile: "key.pem", TrustedProxyCIDRs: []string{"10.42.0.0/24"}}); err == nil {
		t.Fatal("direct TLS accepted proxy CIDRs")
	}
	proxy, err := parseProductionListenerMode("trusted-proxy-http")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateProductionListenerSettings(proxy, &productionsection.Settings{Issuer: "https://idp.example.test/idp", TrustedProxyCIDRs: []string{"10.42.0.0/24"}}); err != nil {
		t.Fatalf("valid trusted proxy settings rejected: %v", err)
	}
	if err := validateProductionListenerSettings(proxy, &productionsection.Settings{Issuer: "http://idp.example.test", TrustedProxyCIDRs: []string{"10.42.0.0/24"}}); err == nil {
		t.Fatal("trusted proxy accepted HTTP issuer")
	}
}

func TestReadProductionSignupProgramRequiresBoundedRegularSource(t *testing.T) {
	if _, err := readProductionSignupProgram(""); err == nil {
		t.Fatal("empty program path accepted")
	}
	directory := t.TempDir()
	if _, err := readProductionSignupProgram(filepath.Join(directory, "missing.js")); err == nil {
		t.Fatal("missing program source accepted")
	}
	if _, err := readProductionSignupProgram(directory); err == nil {
		t.Fatal("directory accepted as program source")
	}
	path := filepath.Join(directory, "signup.js")
	if err := os.WriteFile(path, []byte(idpsignup.DefaultSource), 0o644); err != nil {
		t.Fatal(err)
	}
	source, err := readProductionSignupProgram(path)
	if err != nil {
		t.Fatal(err)
	}
	if source != idpsignup.DefaultSource {
		t.Fatal("program source changed while reading")
	}
	oversized := filepath.Join(directory, "oversized.js")
	if err := os.WriteFile(oversized, make([]byte, maxProductionSignupProgramBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readProductionSignupProgram(oversized); err == nil {
		t.Fatal("oversized program accepted")
	}
}

func TestNewProductionSignupManagerChecksAndActivatesOnlySupportedPrograms(t *testing.T) {
	manager, err := newProductionSignupManager(context.Background(), idpsignup.DefaultSource, idp.NewMemorySink(), productionSignupServices{})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close(context.Background())
	if err := manager.Ready(); err != nil {
		t.Fatalf("activated program not ready: %v", err)
	}
	if _, err := newProductionSignupManager(context.Background(), "not javascript", idp.NewMemorySink(), productionSignupServices{}); err == nil {
		t.Fatal("invalid program accepted")
	}
	if _, err := newProductionSignupManager(context.Background(), idpsignup.EmailVerifiedSource, idp.NewMemorySink(), productionSignupServices{}); err == nil || !strings.Contains(err.Error(), "unsupported native services: email_challenge") {
		t.Fatalf("unsupported email challenge error = %v", err)
	}
	emailManager, err := newProductionSignupManager(context.Background(), idpsignup.EmailVerifiedSource, idp.NewMemorySink(), productionSignupServices{EmailChallenges: true})
	if err != nil {
		t.Fatalf("email challenge program rejected with service available: %v", err)
	}
	defer emailManager.Close(context.Background())
	combinedManager, err := newProductionSignupManager(context.Background(), idpsignup.VerifiedInviteSource, idp.NewMemorySink(), productionSignupServices{EmailChallenges: true})
	if err != nil {
		t.Fatalf("combined invitation and email challenge program rejected: %v", err)
	}
	defer combinedManager.Close(context.Background())
	inviteManager, err := newProductionSignupManager(context.Background(), idpsignup.InviteRequiredSource, idp.NewMemorySink(), productionSignupServices{})
	if err != nil {
		t.Fatalf("supported durable invitation program rejected: %v", err)
	}
	defer inviteManager.Close(context.Background())
	inviteArtifact, err := idpsignup.Compile(context.Background(), idpsignup.InviteRequiredSource)
	if err != nil || !productionProgramRequiresDurableInvitations(inviteArtifact.Program()) {
		t.Fatalf("durable invitation requirement was not detected: %v", err)
	}
	providerWithoutConsumption := strings.Replace(idpsignup.InviteRequiredSource, `, "consumeInvitation"`, "", 1)
	if _, err := newProductionSignupManager(context.Background(), providerWithoutConsumption, idp.NewMemorySink(), productionSignupServices{}); err == nil || !strings.Contains(err.Error(), "durable_invitation_provider_without_consumeInvitation") {
		t.Fatalf("durable provider without consumption error = %v", err)
	}
	capabilityProgram := `const A = require("tinyidp").v1;
module.exports = A.program("unsupported-capability", p => {
  p.capabilities({"clock.now": {version:1}});
  const start = A.lambda("signup.start", { input:"signupStartInput", output:"signupResult", outcomes:["complete"], effects:[], capabilities:["clock.now"], timeoutMs:250, maxCapabilityCalls:1, maxOutputBytes:1024, run: async ctx => { await ctx.cap.clock.now({}); return A.result.complete(); } });
  p.workflow("signup", { version:1, entry:"start", handlers:{start}, edges:[] });
});`
	_, err = newProductionSignupManager(context.Background(), capabilityProgram, idp.NewMemorySink(), productionSignupServices{})
	if err == nil || !strings.Contains(err.Error(), "unsupported native capabilities: clock.now") {
		t.Fatalf("unsupported capability error = %v", err)
	}
	displayNameCapabilityProgram := `const A = require("tinyidp").v1;
module.exports = A.program("display-name-capability", p => {
  p.capabilities({"identity.displayName.lookup": {version:1}});
  const start = A.lambda("signup.start", { input:"signupStartInput", output:"signupResult", outcomes:["complete"], effects:[], capabilities:["identity.displayName.lookup"], timeoutMs:250, maxCapabilityCalls:1, maxOutputBytes:1024, run: async ctx => { await ctx.cap.identity.displayName.lookup({displayName:"Ada"}); return A.result.complete(); } });
  p.workflow("signup", { version:1, entry:"start", handlers:{start}, edges:[] });
});`
	_, err = newProductionSignupManager(context.Background(), displayNameCapabilityProgram, idp.NewMemorySink(), productionSignupServices{})
	if err == nil || !strings.Contains(err.Error(), "unsupported native capabilities: identity.displayName.lookup") {
		t.Fatalf("unavailable display-name capability error = %v", err)
	}
	_, err = newProductionSignupManager(context.Background(), displayNameCapabilityProgram, idp.NewMemorySink(), productionSignupServices{DisplayNameLookup: true})
	if err == nil || !strings.Contains(err.Error(), "workflow signup handler start: identity.displayName.lookup@v1") {
		t.Fatalf("entry-handler display-name capability error = %v", err)
	}

	verifiedInviteArtifact, err := idpsignup.Compile(context.Background(), idpsignup.VerifiedInviteSource)
	if err != nil {
		t.Fatal(err)
	}
	resumedDisplayNameProgram := verifiedInviteArtifact.Program()
	resumedDisplayNameProgram.Capabilities[idpaccounts.DisplayNameLookupCapabilityID] = idpprogram.CapabilityRequirement{ID: idpaccounts.DisplayNameLookupCapabilityID, Version: idpaccounts.DisplayNameLookupCapabilityVersion}
	workflow := resumedDisplayNameProgram.Workflows[idpsignup.WorkflowID]
	submitted := resumedDisplayNameProgram.Lambdas[workflow.Handlers[idpsignup.SubmittedHandler].LambdaID]
	submitted.RequiredCapabilities = append(submitted.RequiredCapabilities, idpprogram.CapabilityRequirement{ID: idpaccounts.DisplayNameLookupCapabilityID, Version: idpaccounts.DisplayNameLookupCapabilityVersion})
	resumedDisplayNameProgram.Lambdas[submitted.ID] = submitted
	if err := validateProductionSignupProgram(resumedDisplayNameProgram, productionSignupServices{EmailChallenges: true, DisplayNameLookup: true}); err != nil {
		t.Fatalf("resumed display-name capability rejected: %v", err)
	}

	verifiedInviteArtifact, err = idpsignup.Compile(context.Background(), idpsignup.VerifiedInviteSource)
	if err != nil {
		t.Fatal(err)
	}
	workflowCapabilityMisuse := verifiedInviteArtifact.Program()
	workflow = workflowCapabilityMisuse.Workflows[idpsignup.WorkflowID]
	submitted = workflowCapabilityMisuse.Lambdas[workflow.Handlers[idpsignup.SubmittedHandler].LambdaID]
	submitted.RequiredCapabilities = append(submitted.RequiredCapabilities, idpprogram.CapabilityRequirement{ID: idpinvite.LookupCapabilityID, Version: idpinvite.LookupCapabilityVersion})
	workflowCapabilityMisuse.Lambdas[submitted.ID] = submitted
	if err := validateProductionSignupProgram(workflowCapabilityMisuse, productionSignupServices{}); err == nil || !strings.Contains(err.Error(), "workflow signup handler submitted: invitation.lookup@v1") {
		t.Fatalf("workflow invitation capability error = %v", err)
	}

	verifiedInviteArtifact, err = idpsignup.Compile(context.Background(), idpsignup.VerifiedInviteSource)
	if err != nil {
		t.Fatal(err)
	}
	providerCapabilityMisuse := verifiedInviteArtifact.Program()
	providerCapabilityMisuse.Capabilities[idpaccounts.DisplayNameLookupCapabilityID] = idpprogram.CapabilityRequirement{ID: idpaccounts.DisplayNameLookupCapabilityID, Version: idpaccounts.DisplayNameLookupCapabilityVersion}
	provider := providerCapabilityMisuse.Providers["invitation.signup"]
	validate := providerCapabilityMisuse.Lambdas[provider.Handlers[idpprogram.InvitationValidateHandler].LambdaID]
	validate.RequiredCapabilities = append(validate.RequiredCapabilities, idpprogram.CapabilityRequirement{ID: idpaccounts.DisplayNameLookupCapabilityID, Version: idpaccounts.DisplayNameLookupCapabilityVersion})
	providerCapabilityMisuse.Lambdas[validate.ID] = validate
	if err := validateProductionSignupProgram(providerCapabilityMisuse, productionSignupServices{DisplayNameLookup: true}); err == nil || !strings.Contains(err.Error(), "provider invitation.signup handler validate: identity.displayName.lookup@v1") {
		t.Fatalf("provider display-name capability error = %v", err)
	}
}

func TestProductionEmailChallengesRequireCompleteProgramBoundConfiguration(t *testing.T) {
	artifact, err := idpsignup.Compile(context.Background(), idpsignup.EmailVerifiedSource)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "challenge-key")
	if err := os.WriteFile(keyPath, []byte("0123456789abcdef0123456789abcdef"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings := &productionsection.Settings{
		EmailChallengeKeyFile: keyPath, EmailSMTPAddress: "mailcatcher:1025", EmailSMTPTLSMode: "private-plaintext",
		EmailFromAddress: "accounts@example.test", EmailFromName: "TinyIDP", EmailSMTPConnectTimeout: "1s", EmailSMTPSendTimeout: "2s",
	}
	service, err := newProductionEmailChallenges(settings, idpemailchallenge.NewMemoryStore(), artifact.Program())
	if err != nil || service == nil {
		t.Fatalf("complete email challenge configuration = %v, %v", service, err)
	}
	missing := *settings
	missing.EmailFromAddress = ""
	if _, err := newProductionEmailChallenges(&missing, idpemailchallenge.NewMemoryStore(), artifact.Program()); err == nil {
		t.Fatal("missing sender was accepted")
	}
	defaultArtifact, err := idpsignup.Compile(context.Background(), idpsignup.DefaultSource)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newProductionEmailChallenges(settings, idpemailchallenge.NewMemoryStore(), defaultArtifact.Program()); err == nil {
		t.Fatal("mail configuration was accepted for a program without email challenges")
	}
}

func TestProductionCommandRequiresSignupProgramAndDropsLegacyRegistrationFlag(t *testing.T) {
	registry, err := pluginapi.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	command, err := NewServeProductionCommand(registry)
	if err != nil {
		t.Fatal(err)
	}
	section, ok := command.Schema.Get(productionsection.Slug)
	if !ok {
		t.Fatal("default command section is unavailable")
	}
	program, ok := section.GetDefinitions().Get("signup-program-file")
	if !ok || !program.Required {
		t.Fatal("signup-program-file is not a required production flag")
	}
	lookupKey, ok := section.GetDefinitions().Get("invitation-lookup-key-file")
	if !ok || lookupKey.Required {
		t.Fatal("invitation lookup key must be conditionally required by the selected program")
	}
	for _, conditional := range []string{"email-challenge-key-file", "email-smtp-address", "email-smtp-tls-mode", "email-smtp-password-file", "email-from-address"} {
		definition, ok := section.GetDefinitions().Get(conditional)
		if !ok || definition.Required {
			t.Fatalf("%s must exist and be conditionally required by the selected program", conditional)
		}
	}
	if _, legacy := section.GetDefinitions().Get("registration-enabled"); legacy {
		t.Fatal("legacy registration-enabled production flag is still exposed")
	}
	chooser, ok := section.GetDefinitions().Get("account-chooser")
	if !ok || chooser.Required {
		t.Fatal("optional account-chooser flag is unavailable")
	}
	adminAddr, ok := section.GetDefinitions().Get("admin-addr")
	if !ok || adminAddr.Default == nil || *adminAddr.Default != "127.0.0.1:9090" {
		t.Fatalf("internal administration listener field = %#v, %v", adminAddr, ok)
	}
	for _, required := range []string{"clients-file", "theme-dir", "theme-catalog-file"} {
		definition, ok := section.GetDefinitions().Get(required)
		if !ok || !definition.Required {
			t.Fatalf("%s is not a required production flag", required)
		}
	}
	if _, legacy := section.GetDefinitions().Get("message-desk-origin"); legacy {
		t.Fatal("legacy message-desk-origin production flag is still exposed")
	}
}

func TestProductionCommandComposesCompiledPluginSections(t *testing.T) {
	registry, err := pluginapi.NewRegistry(jitsiplugin.Definition{})
	if err != nil {
		t.Fatal(err)
	}
	command, err := NewServeProductionCommand(registry)
	if err != nil {
		t.Fatal(err)
	}
	section, ok := command.Schema.Get(jitsiplugin.SectionSlug)
	if !ok || section.GetPrefix() != "jitsi-" {
		t.Fatalf("Jitsi section = %#v, %v", section, ok)
	}
	for _, name := range []string{"enabled", "public-origin", "shared-secret-file", "policy-program-file"} {
		if _, ok := section.GetDefinitions().Get(name); !ok {
			t.Fatalf("missing Jitsi field %q", name)
		}
	}
}
