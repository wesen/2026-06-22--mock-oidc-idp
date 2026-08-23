// Package twoprocessharness proves the deployable Tiny-IDP and Message Desk
// binaries cooperate without sharing an in-process provider or durable state.
package twoprocessharness

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/go-go-golems/tiny-idp/pkg/idpsignup"
)

const (
	idpPublicOrigin     = "https://idp.example.test"
	idpIssuer           = idpPublicOrigin + "/idp"
	messagePublicOrigin = "https://message.example.test"
)

// TestTwoProcessLifecycle starts the actual command binaries. It deliberately
// keeps the browser-visible origins HTTPS while the test is the trusted local
// proxy that terminates TLS and forwards to the private HTTP listeners.
func TestTwoProcessLifecycle(t *testing.T) {
	t.Parallel()
	harness := newHarness(t)
	harness.build()
	harness.initializeMessageDesk()
	harness.startTinyIDP()
	harness.waitReady(harness.idpAddress, idpPublicOrigin, "/idp/readyz")
	harness.startIDPProxy()
	harness.startMessageDesk()
	harness.waitReady(harness.messageAddress, messagePublicOrigin, "/readyz")

	harness.requireAudit("tinyidp", "identity.bootstrap.client_created")
	harness.requireLog("tinyidp", "tinyidp production host listening")
	harness.requireLog("message-desk", "message application listening")
}

func TestTwoProcessConcurrentSignupContinuation(t *testing.T) {
	t.Parallel()
	harness := newHarness(t)
	harness.build()
	harness.initializeMessageDesk()
	harness.startTinyIDP()
	harness.waitReady(harness.idpAddress, idpPublicOrigin, "/idp/readyz")
	harness.startIDPProxy()
	harness.startMessageDesk()
	harness.waitReady(harness.messageAddress, messagePublicOrigin, "/readyz")

	browser := newPublicBrowser(harness)
	start := browser.get(t, messagePublicOrigin+"/auth/register")
	requireStatus(t, start, http.StatusSeeOther)
	page := browser.get(t, requiredLocation(t, start))
	requireStatus(t, page, http.StatusOK)
	body, err := io.ReadAll(page.Body)
	page.Body.Close()
	if err != nil {
		t.Fatalf("read concurrent signup form: %v", err)
	}
	form := url.Values{
		"action":                {"submit"},
		"interaction":           {hiddenFormValue(t, body, "interaction")},
		"workflow_continuation": {hiddenFormValue(t, body, "workflow_continuation")},
		"csrf_token":            {hiddenFormValue(t, body, "csrf_token")},
		"display_name":          {"Concurrent Ada"},
		"email":                 {"concurrent@example.test"},
		"password":              {"correct horse battery staple 2026"},
		"password_confirmation": {"correct horse battery staple 2026"},
	}
	left, right := browser.clone(), browser.clone()
	type result struct {
		response *http.Response
		err      error
	}
	results := make(chan result, 2)
	for _, client := range []*publicBrowser{left, right} {
		go func(client *publicBrowser) {
			response, requestErr := client.do(http.MethodPost, idpIssuer+"/authorize", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
			results <- result{response: response, err: requestErr}
		}(client)
	}
	statuses := map[int]int{}
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("concurrent signup submission: %v", result.err)
		}
		statuses[result.response.StatusCode]++
		result.response.Body.Close()
	}
	if statuses[http.StatusOK] != 1 || statuses[http.StatusBadRequest] != 1 || len(statuses) != 2 {
		t.Fatalf("concurrent signup statuses = %#v, want one 200 and one 400", statuses)
	}
	harness.assertSingleProviderIdentityAndSession()
}

func TestTwoProcessRegistrationRedirectAndSignup(t *testing.T) {
	t.Parallel()
	harness := newHarness(t)
	harness.build()
	harness.initializeMessageDesk()
	harness.startTinyIDP()
	harness.waitReady(harness.idpAddress, idpPublicOrigin, "/idp/readyz")
	harness.startIDPProxy()
	harness.startMessageDesk()
	harness.waitReady(harness.messageAddress, messagePublicOrigin, "/readyz")

	browser := newPublicBrowser(harness)
	weakBrowser := newPublicBrowser(harness)
	weakRegistration := weakBrowser.get(t, messagePublicOrigin+"/auth/register")
	requireStatus(t, weakRegistration, http.StatusSeeOther)
	weakPage := weakBrowser.get(t, requiredLocation(t, weakRegistration))
	requireStatus(t, weakPage, http.StatusOK)
	weakHTML, err := io.ReadAll(weakPage.Body)
	weakPage.Body.Close()
	if err != nil {
		t.Fatalf("read weak-password signup form: %v", err)
	}
	weakPassword := "too-short"
	weakSubmission := weakBrowser.postForm(t, idpIssuer+"/authorize", url.Values{
		"action":                {"submit"},
		"interaction":           {hiddenFormValue(t, weakHTML, "interaction")},
		"workflow_continuation": {hiddenFormValue(t, weakHTML, "workflow_continuation")},
		"csrf_token":            {hiddenFormValue(t, weakHTML, "csrf_token")},
		"display_name":          {"Weak Password"},
		"email":                 {"weak@example.test"},
		"password":              {weakPassword},
		"password_confirmation": {weakPassword},
	})
	requireStatus(t, weakSubmission, http.StatusBadRequest)
	weakBody, err := io.ReadAll(weakSubmission.Body)
	weakSubmission.Body.Close()
	if err != nil {
		t.Fatalf("read weak-password rejection: %v", err)
	}
	if strings.Contains(string(weakBody), weakPassword) {
		t.Fatalf("weak-password rejection reflects password: %s", weakBody)
	}
	harness.assertProviderCounts(0, 0)
	expiredBrowser := newPublicBrowser(harness)
	expiredStart := expiredBrowser.get(t, messagePublicOrigin+"/auth/register")
	requireStatus(t, expiredStart, http.StatusSeeOther)
	expiredPage := expiredBrowser.get(t, requiredLocation(t, expiredStart))
	requireStatus(t, expiredPage, http.StatusOK)
	expiredHTML, err := io.ReadAll(expiredPage.Body)
	expiredPage.Body.Close()
	if err != nil {
		t.Fatalf("read expired-continuation signup form: %v", err)
	}
	harness.expireWorkflowContinuations()
	expiredSubmission := expiredBrowser.postForm(t, idpIssuer+"/authorize", url.Values{
		"action":                {"submit"},
		"interaction":           {hiddenFormValue(t, expiredHTML, "interaction")},
		"workflow_continuation": {hiddenFormValue(t, expiredHTML, "workflow_continuation")},
		"csrf_token":            {hiddenFormValue(t, expiredHTML, "csrf_token")},
		"display_name":          {"Expired Continuation"},
		"email":                 {"expired@example.test"},
		"password":              {"correct horse battery staple 2026"},
		"password_confirmation": {"correct horse battery staple 2026"},
	})
	requireStatus(t, expiredSubmission, http.StatusBadRequest)
	expiredSubmission.Body.Close()
	harness.assertProviderCounts(0, 0)
	malformedBrowser := newPublicBrowser(harness)
	malformed := malformedBrowser.get(t, messagePublicOrigin+"/auth/register?return_to=%2F%2Fattacker.example.test")
	requireStatus(t, malformed, http.StatusBadRequest)
	malformedBody, err := io.ReadAll(malformed.Body)
	malformed.Body.Close()
	if err != nil {
		t.Fatalf("read malformed registration rejection: %v", err)
	}
	if strings.Contains(string(malformedBody), "attacker.example.test") {
		t.Fatalf("malformed registration reflects untrusted target: %s", malformedBody)
	}
	harness.assertProviderCounts(0, 0)

	registration := browser.get(t, messagePublicOrigin+"/auth/register?return_to=/messages")
	requireStatus(t, registration, http.StatusSeeOther)
	authorizeURL := requiredLocation(t, registration)
	assertRegistrationAuthorization(t, authorizeURL)
	harness.stop("message-desk")
	harness.startMessageDesk()
	harness.waitReady(harness.messageAddress, messagePublicOrigin, "/readyz")

	formPage := browser.get(t, authorizeURL)
	requireStatus(t, formPage, http.StatusOK)
	page, err := io.ReadAll(formPage.Body)
	formPage.Body.Close()
	if err != nil {
		t.Fatalf("read signup form: %v", err)
	}
	form := url.Values{
		"action":                {"submit"},
		"interaction":           {hiddenFormValue(t, page, "interaction")},
		"workflow_continuation": {hiddenFormValue(t, page, "workflow_continuation")},
		"csrf_token":            {hiddenFormValue(t, page, "csrf_token")},
		"display_name":          {"Ada Lovelace"},
		"email":                 {"ada@example.test"},
		"password":              {"correct horse battery staple 2026"},
		"password_confirmation": {"correct horse battery staple 2026"},
	}
	harness.stop("tinyidp")
	harness.startTinyIDP()
	harness.waitReady(harness.idpAddress, idpPublicOrigin, "/idp/readyz")
	completed := browser.postForm(t, idpIssuer+"/authorize", form)
	requireStatus(t, completed, http.StatusOK)
	harness.assertSingleProviderIdentityAndSession()
	duplicateBrowser := newPublicBrowser(harness)
	duplicateStart := duplicateBrowser.get(t, messagePublicOrigin+"/auth/register")
	requireStatus(t, duplicateStart, http.StatusSeeOther)
	duplicatePage := duplicateBrowser.get(t, requiredLocation(t, duplicateStart))
	requireStatus(t, duplicatePage, http.StatusOK)
	duplicateHTML, err := io.ReadAll(duplicatePage.Body)
	duplicatePage.Body.Close()
	if err != nil {
		t.Fatalf("read duplicate signup form: %v", err)
	}
	duplicate := duplicateBrowser.postForm(t, idpIssuer+"/authorize", url.Values{
		"action":                {"submit"},
		"interaction":           {hiddenFormValue(t, duplicateHTML, "interaction")},
		"workflow_continuation": {hiddenFormValue(t, duplicateHTML, "workflow_continuation")},
		"csrf_token":            {hiddenFormValue(t, duplicateHTML, "csrf_token")},
		"display_name":          {"Different Name"},
		"email":                 {"ada@example.test"},
		"password":              {"correct horse battery staple 2026"},
		"password_confirmation": {"correct horse battery staple 2026"},
	})
	requireStatus(t, duplicate, http.StatusBadRequest)
	duplicateBody, err := io.ReadAll(duplicate.Body)
	duplicate.Body.Close()
	if err != nil {
		t.Fatalf("read duplicate signup rejection: %v", err)
	}
	if !strings.Contains(string(duplicateBody), "An account already uses this email address.") || !strings.Contains(string(duplicateBody), "Return to application") || strings.Contains(string(duplicateBody), "This value could not be accepted.") {
		t.Fatalf("duplicate rejection did not use the actionable themed identity guidance: %s", duplicateBody)
	}
	harness.assertSingleProviderIdentityAndSession()
	replayed := browser.postForm(t, idpIssuer+"/authorize", form)
	requireStatus(t, replayed, http.StatusBadRequest)
	replayed.Body.Close()
	harness.assertSingleProviderIdentityAndSession()
	consentPage, err := io.ReadAll(completed.Body)
	completed.Body.Close()
	if err != nil {
		t.Fatalf("read authorization consent: %v", err)
	}
	approval := browser.postForm(t, idpIssuer+"/authorize", url.Values{
		"action":      {"approve"},
		"interaction": {hiddenFormValue(t, consentPage, "interaction")},
		"csrf_token":  {hiddenFormValue(t, consentPage, "csrf_token")},
	})
	requireStatus(t, approval, http.StatusSeeOther)
	callbackURL := requiredLocation(t, approval)
	callback, err := url.Parse(callbackURL)
	if err != nil || callback.Scheme != "https" || callback.Host != "message.example.test" || callback.Path != "/auth/callback" || callback.Query().Get("code") == "" || callback.Query().Get("state") == "" {
		t.Fatalf("unexpected signup callback %q", callbackURL)
	}

	finished := browser.get(t, callbackURL)
	requireStatus(t, finished, http.StatusSeeOther)
	if location := requiredLocation(t, finished); location != "/messages" {
		t.Fatalf("signup return location = %q, want /messages", location)
	}
	session := browser.get(t, messagePublicOrigin+"/api/session")
	requireStatus(t, session, http.StatusOK)
	var sessionBody struct {
		Authenticated bool   `json:"authenticated"`
		Subject       string `json:"subject"`
		CSRFToken     string `json:"csrfToken"`
	}
	if err := json.NewDecoder(session.Body).Decode(&sessionBody); err != nil {
		session.Body.Close()
		t.Fatalf("decode application session: %v", err)
	}
	session.Body.Close()
	if !sessionBody.Authenticated || sessionBody.Subject == "" || sessionBody.CSRFToken == "" {
		t.Fatalf("application session after signup = %#v", sessionBody)
	}
	messageText := "Hello from the real two-process harness."
	created := browser.postJSON(t, messagePublicOrigin+"/api/messages", map[string]string{"body": messageText}, http.Header{"X-CSRF-Token": []string{sessionBody.CSRFToken}})
	requireStatus(t, created, http.StatusCreated)
	var createdBody struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(created.Body).Decode(&createdBody); err != nil {
		created.Body.Close()
		t.Fatalf("decode created message: %v", err)
	}
	created.Body.Close()
	if createdBody.Body != messageText {
		t.Fatalf("created message body = %q, want %q", createdBody.Body, messageText)
	}
	assertMessageListContains(t, browser, messageText)

	rejected := browser.postJSON(t, messagePublicOrigin+"/api/messages", map[string]string{"body": "must not persist"}, nil)
	requireStatus(t, rejected, http.StatusForbidden)
	rejected.Body.Close()
	assertMessageListContains(t, browser, messageText)
	assertMessageListExcludes(t, browser, "must not persist")
	harness.stop("message-desk")
	harness.stop("tinyidp")
	harness.startTinyIDP()
	harness.waitReady(harness.idpAddress, idpPublicOrigin, "/idp/readyz")
	harness.startMessageDesk()
	harness.waitReady(harness.messageAddress, messagePublicOrigin, "/readyz")
	harness.assertSingleProviderIdentityAndSession()
	restartedSession := readAuthenticatedSession(t, browser)
	assertMessageListContains(t, browser, messageText)

	providerLogout := browser.postEmpty(t, messagePublicOrigin+"/auth/logout", http.Header{"X-CSRF-Token": []string{restartedSession.CSRFToken}})
	requireStatus(t, providerLogout, http.StatusOK)
	var providerLogoutBody struct {
		EndSessionURL string `json:"endSessionUrl"`
	}
	if err := json.NewDecoder(providerLogout.Body).Decode(&providerLogoutBody); err != nil {
		providerLogout.Body.Close()
		t.Fatalf("decode provider logout response: %v", err)
	}
	providerLogout.Body.Close()
	if providerLogoutBody.EndSessionURL != idpIssuer+"/end-session?client_id=tinyidp-message-app&post_logout_redirect_uri="+url.QueryEscape(messagePublicOrigin+"/") {
		t.Fatalf("provider logout URL = %q", providerLogoutBody.EndSessionURL)
	}
	providerEnd := browser.get(t, providerLogoutBody.EndSessionURL)
	requireStatus(t, providerEnd, http.StatusFound)
	if location := requiredLocation(t, providerEnd); location != messagePublicOrigin+"/" {
		t.Fatalf("provider logout redirect = %q, want %q", location, messagePublicOrigin+"/")
	}
	if len(browser.cookies["idp.example.test"]) != 0 {
		t.Fatalf("provider logout left IdP cookies: %#v", browser.cookies["idp.example.test"])
	}

	loginStart := browser.get(t, messagePublicOrigin+"/auth/login?return_to=/messages")
	requireStatus(t, loginStart, http.StatusSeeOther)
	loginPage := browser.get(t, requiredLocation(t, loginStart))
	requireStatus(t, loginPage, http.StatusOK)
	loginHTML, err := io.ReadAll(loginPage.Body)
	loginPage.Body.Close()
	if err != nil {
		t.Fatalf("read fresh provider login: %v", err)
	}
	loginApproval := browser.postForm(t, idpIssuer+"/authorize", url.Values{
		"action":      {"approve"},
		"interaction": {hiddenFormValue(t, loginHTML, "interaction")},
		"csrf_token":  {hiddenFormValue(t, loginHTML, "csrf_token")},
		"login":       {"ada@example.test"},
		"password":    {"correct horse battery staple 2026"},
	})
	requireStatus(t, loginApproval, http.StatusSeeOther)
	reloggedCallback := browser.get(t, requiredLocation(t, loginApproval))
	requireStatus(t, reloggedCallback, http.StatusSeeOther)
	if location := requiredLocation(t, reloggedCallback); location != "/messages" {
		t.Fatalf("fresh-login return location = %q, want /messages", location)
	}
	reloggedSession := readAuthenticatedSession(t, browser)

	localLogout := browser.postEmpty(t, messagePublicOrigin+"/auth/logout/local", http.Header{"X-CSRF-Token": []string{reloggedSession.CSRFToken}})
	requireStatus(t, localLogout, http.StatusNoContent)
	localLogout.Body.Close()
	loggedOut := browser.get(t, messagePublicOrigin+"/api/session")
	requireStatus(t, loggedOut, http.StatusOK)
	var loggedOutBody struct {
		Authenticated bool `json:"authenticated"`
	}
	if err := json.NewDecoder(loggedOut.Body).Decode(&loggedOutBody); err != nil {
		loggedOut.Body.Close()
		t.Fatalf("decode local logout session: %v", err)
	}
	loggedOut.Body.Close()
	if loggedOutBody.Authenticated || len(browser.cookies["idp.example.test"]) == 0 {
		t.Fatalf("local logout did not leave only the provider browser context: session=%#v idpCookies=%d", loggedOutBody, len(browser.cookies["idp.example.test"]))
	}
	harness.assertNoSensitiveArtifacts(browser, []string{
		"correct horse battery staple 2026", "too-short", strings.Repeat("B", 32),
		callback.Query().Get("code"), sessionBody.CSRFToken, restartedSession.CSRFToken,
	})
}

type applicationSession struct {
	Authenticated bool   `json:"authenticated"`
	Subject       string `json:"subject"`
	CSRFToken     string `json:"csrfToken"`
}

func readAuthenticatedSession(t *testing.T, browser *publicBrowser) applicationSession {
	t.Helper()
	response := browser.get(t, messagePublicOrigin+"/api/session")
	requireStatus(t, response, http.StatusOK)
	var value applicationSession
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		response.Body.Close()
		t.Fatalf("decode application session: %v", err)
	}
	response.Body.Close()
	if !value.Authenticated || value.Subject == "" || value.CSRFToken == "" {
		t.Fatalf("application session = %#v", value)
	}
	return value
}

type harness struct {
	t              *testing.T
	root           string
	repo           string
	binRoot        string
	idpAddress     string
	adminAddress   string
	messageAddress string
	idpLog         string
	messageLog     string
	idpAudit       string
	idpDatabase    string
	messageState   string
	idpProxy       *httptest.Server
	processes      []*startedProcess
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	repo := repositoryRoot(t)
	root := t.TempDir()
	return &harness{
		t: t, root: root, repo: repo, binRoot: filepath.Join(root, "bin"),
		idpAddress:     unusedLoopbackAddress(t),
		adminAddress:   unusedLoopbackAddress(t),
		messageAddress: unusedLoopbackAddress(t),
		idpLog:         filepath.Join(root, "logs", "tinyidp.log"),
		messageLog:     filepath.Join(root, "logs", "message-desk.log"),
		idpAudit:       filepath.Join(root, "tinyidp", "audit", "events.jsonl"),
		idpDatabase:    filepath.Join(root, "tinyidp", "state", "tinyidp.sqlite"),
		messageState:   filepath.Join(root, "message-desk"),
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate harness source")
	}
	directory := filepath.Dir(file)
	for {
		if info, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil && !info.IsDir() {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("could not locate repository go.mod")
		}
		directory = parent
	}
}

func unusedLoopbackAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate loopback port: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release loopback port: %v", err)
	}
	return address
}

func (h *harness) build() {
	h.t.Helper()
	if err := os.MkdirAll(h.binRoot, 0o700); err != nil {
		h.t.Fatalf("create binary directory: %v", err)
	}
	h.runForeground("build tinyidp", "go", "build", "-o", filepath.Join(h.binRoot, "tinyidp"), "./cmd/tinyidp")
	h.runForeground("build message desk", "go", "build", "-o", filepath.Join(h.binRoot, "message-desk"), "./examples/tinyidp-message-app")
}

func (h *harness) initializeMessageDesk() {
	h.t.Helper()
	h.runForeground("initialize message desk", filepath.Join(h.binRoot, "message-desk"), "init",
		"--state-root", h.messageState, "--public-base-url", messagePublicOrigin)
}

func (h *harness) startTinyIDP() {
	h.t.Helper()
	program := filepath.Join(h.root, "tinyidp", "signup.js")
	secret := filepath.Join(h.root, "tinyidp", "secrets", "token.key")
	adminAuthKey := filepath.Join(h.root, "tinyidp", "secrets", "admin-auth.key")
	adminActionKey := filepath.Join(h.root, "tinyidp", "secrets", "admin-action.key")
	adminBackupRoot := filepath.Join(h.root, "tinyidp", "admin-artifacts")
	invitationKey := filepath.Join(h.root, "tinyidp", "secrets", "invitation.key")
	clients := filepath.Join(h.repo, "examples", "production-host", "catalog", "clients.json")
	themeDir := filepath.Join(h.repo, "examples", "production-host", "themes")
	if err := os.MkdirAll(filepath.Dir(program), 0o700); err != nil {
		h.t.Fatalf("create Tiny-IDP state directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(secret), 0o700); err != nil {
		h.t.Fatalf("create Tiny-IDP secret directory: %v", err)
	}
	if err := os.WriteFile(program, []byte(idpsignup.DefaultSource), 0o600); err != nil {
		h.t.Fatalf("write checked signup program: %v", err)
	}
	if err := os.WriteFile(secret, bytes.Repeat([]byte{0x42}, 32), 0o600); err != nil {
		h.t.Fatalf("write Tiny-IDP test secret: %v", err)
	}
	for path, value := range map[string]byte{
		adminAuthKey: 0x43, adminActionKey: 0x44, invitationKey: 0x45,
	} {
		if err := os.WriteFile(path, bytes.Repeat([]byte{value}, 32), 0o600); err != nil {
			h.t.Fatalf("write Tiny-IDP administration test secret: %v", err)
		}
	}
	h.start("tinyidp", h.idpLog, filepath.Join(h.binRoot, "tinyidp"), "serve-production",
		"--addr", h.idpAddress,
		"--admin-addr", h.adminAddress,
		"--listener-mode", "trusted-proxy-http",
		"--issuer", idpIssuer,
		"--clients-file", clients,
		"--theme-dir", themeDir,
		"--theme-catalog-file", filepath.Join(themeDir, "themes.json"),
		"--signup-program-file", program,
		"--db", h.idpDatabase,
		"--audit-path", h.idpAudit,
		"--token-secret-file", secret,
		"--admin-auth-key-file", adminAuthKey,
		"--admin-action-key-file", adminActionKey,
		"--admin-backup-root", adminBackupRoot,
		"--invitation-lookup-key-file", invitationKey,
		"--trusted-proxy-cidrs", "127.0.0.1/32")
}

func (h *harness) assertSingleProviderIdentityAndSession() {
	h.t.Helper()
	h.assertProviderCounts(1, 1)
}

func (h *harness) assertProviderCounts(users, sessions int) {
	h.t.Helper()
	database, err := sql.Open("sqlite3", "file:"+h.idpDatabase+"?mode=ro")
	if err != nil {
		h.t.Fatalf("open Tiny-IDP assertion database: %v", err)
	}
	defer database.Close()
	context, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for table, want := range map[string]int{"users": users, "sessions": sessions} {
		var got int
		if err := database.QueryRowContext(context, "SELECT COUNT(*) FROM "+table).Scan(&got); err != nil {
			h.t.Fatalf("count Tiny-IDP %s: %v", table, err)
		}
		if got != want {
			h.t.Fatalf("Tiny-IDP %s count = %d, want %d", table, got, want)
		}
	}
}

func (h *harness) assertNoSensitiveArtifacts(browser *publicBrowser, values []string) {
	h.t.Helper()
	for _, byName := range browser.cookies {
		for _, cookie := range byName {
			values = append(values, cookie.Value)
		}
	}
	artifacts := map[string]string{
		"Tiny-IDP log":       readFile(h.idpLog),
		"Message Desk log":   readFile(h.messageLog),
		"Tiny-IDP audit":     readFile(h.idpAudit),
		"Message Desk audit": readFile(filepath.Join(h.messageState, "audit", "events.jsonl")),
	}
	for name, artifact := range artifacts {
		for _, value := range values {
			if value != "" && strings.Contains(artifact, value) {
				h.t.Fatalf("%s contains sensitive runtime value %q", name, value)
			}
		}
	}
}

func (h *harness) expireWorkflowContinuations() {
	h.t.Helper()
	database, err := sql.Open("sqlite3", h.idpDatabase)
	if err != nil {
		h.t.Fatalf("open Tiny-IDP continuation database: %v", err)
	}
	defer database.Close()
	rows, err := database.Query(`SELECT handle_hash, data FROM workflow_continuations WHERE status='active'`)
	if err != nil {
		h.t.Fatalf("list active workflow continuations: %v", err)
	}
	defer rows.Close()
	expired := time.Now().UTC().Add(-time.Minute)
	count := 0
	for rows.Next() {
		var handleHash, encoded []byte
		if err := rows.Scan(&handleHash, &encoded); err != nil {
			h.t.Fatalf("read workflow continuation: %v", err)
		}
		var record map[string]any
		if err := json.Unmarshal(encoded, &record); err != nil {
			h.t.Fatalf("decode workflow continuation: %v", err)
		}
		record["expiresAt"] = expired.Format(time.RFC3339Nano)
		updated, err := json.Marshal(record)
		if err != nil {
			h.t.Fatalf("encode expired workflow continuation: %v", err)
		}
		if _, err := database.Exec(`UPDATE workflow_continuations SET expires_at_ns=?, data=? WHERE handle_hash=?`, expired.UnixNano(), updated, handleHash); err != nil {
			h.t.Fatalf("expire workflow continuation: %v", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		h.t.Fatalf("iterate workflow continuations: %v", err)
	}
	if count == 0 {
		h.t.Fatal("no active workflow continuation to expire")
	}
}

func (h *harness) startMessageDesk() {
	h.t.Helper()
	h.start("message-desk", h.messageLog, filepath.Join(h.binRoot, "message-desk"), "serve",
		"--state-root", h.messageState,
		"--addr", h.messageAddress,
		"--listener-mode", "trusted-proxy-http",
		"--trusted-proxy-cidrs", "127.0.0.1/32",
		"--external-issuer", idpIssuer,
		"--external-backchannel-url", h.idpProxy.URL+"/idp")
}

// startIDPProxy models the only component permitted to translate external
// HTTPS into the IdP's private trusted-proxy listener. It preserves the public
// Host and adds the exact forwarding metadata that the production Traefik
// listener requires; it is not an application shim.
func (h *harness) startIDPProxy() {
	h.t.Helper()
	transport := &http.Transport{}
	h.idpProxy = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		forwarded := request.Clone(request.Context())
		forwarded.URL.Scheme = "http"
		forwarded.URL.Host = h.idpAddress
		forwarded.Host = "idp.example.test"
		forwarded.Header = request.Header.Clone()
		forwarded.Header.Set("X-Forwarded-Proto", "https")
		forwarded.Header.Set("X-Forwarded-Host", "idp.example.test")
		response, err := transport.RoundTrip(forwarded)
		if err != nil {
			http.Error(w, "forward to Tiny-IDP", http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		for key, values := range response.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	h.t.Cleanup(func() {
		if h.idpProxy != nil {
			h.idpProxy.Close()
		}
		transport.CloseIdleConnections()
	})
}

func (h *harness) start(name, logPath, binary string, args ...string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		h.t.Fatalf("create %s log directory: %v", name, err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		h.t.Fatalf("open %s log: %v", name, err)
	}
	command := exec.Command(binary, args...)
	command.Dir = h.repo
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		h.t.Fatalf("start %s: %v", name, err)
	}
	process := &startedProcess{name: name, command: command, logFile: logFile, logPath: logPath}
	h.processes = append(h.processes, process)
	h.t.Cleanup(func() { process.stop(h.t) })
}

func (h *harness) stop(name string) {
	h.t.Helper()
	for index := len(h.processes) - 1; index >= 0; index-- {
		process := h.processes[index]
		if process.name == name {
			process.stop(h.t)
			return
		}
	}
	h.t.Fatalf("process %q is not running", name)
}

func (h *harness) waitReady(address, publicOrigin, path string) {
	h.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		response, err := h.proxyRequest(http.MethodGet, address, publicOrigin, path, nil)
		if err == nil && response.StatusCode == http.StatusOK {
			response.Body.Close()
			return
		}
		if err == nil {
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			last = fmt.Errorf("status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
		} else {
			last = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	h.t.Fatalf("wait for %s readiness: %v\nTiny-IDP log:\n%s\nMessage Desk log:\n%s", publicOrigin, last, readFile(h.idpLog), readFile(h.messageLog))
}

func (h *harness) proxyRequest(method, address, publicOrigin, requestPath string, body io.Reader) (*http.Response, error) {
	request, err := http.NewRequest(method, "http://"+address+requestPath, body)
	if err != nil {
		return nil, err
	}
	publicURL, err := neturl(publicOrigin)
	if err != nil {
		return nil, err
	}
	request.Host = publicURL
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", publicURL)
	return (&http.Client{Timeout: 2 * time.Second}).Do(request)
}

type publicBrowser struct {
	harness *harness
	cookies map[string]map[string]*http.Cookie
	client  *http.Client
}

func newPublicBrowser(harness *harness) *publicBrowser {
	return &publicBrowser{harness: harness, cookies: map[string]map[string]*http.Cookie{}, client: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (b *publicBrowser) clone() *publicBrowser {
	clone := newPublicBrowser(b.harness)
	for host, byName := range b.cookies {
		clone.cookies[host] = map[string]*http.Cookie{}
		for name, cookie := range byName {
			cookieCopy := *cookie
			clone.cookies[host][name] = &cookieCopy
		}
	}
	return clone
}

func (b *publicBrowser) get(t *testing.T, rawURL string) *http.Response {
	t.Helper()
	response, err := b.do(http.MethodGet, rawURL, nil, "")
	if err != nil {
		t.Fatalf("GET %s: %v", rawURL, err)
	}
	return response
}

func (b *publicBrowser) postForm(t *testing.T, rawURL string, form url.Values) *http.Response {
	t.Helper()
	response, err := b.do(http.MethodPost, rawURL, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		t.Fatalf("POST form %s: %v", rawURL, err)
	}
	return response
}

func (b *publicBrowser) postJSON(t *testing.T, rawURL string, value any, headers http.Header) *http.Response {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode JSON request: %v", err)
	}
	response, err := b.doWithHeaders(http.MethodPost, rawURL, bytes.NewReader(body), "application/json", headers)
	if err != nil {
		t.Fatalf("POST JSON %s: %v", rawURL, err)
	}
	return response
}

func (b *publicBrowser) postEmpty(t *testing.T, rawURL string, headers http.Header) *http.Response {
	t.Helper()
	response, err := b.doWithHeaders(http.MethodPost, rawURL, nil, "", headers)
	if err != nil {
		t.Fatalf("POST %s: %v", rawURL, err)
	}
	return response
}

func (b *publicBrowser) do(method, rawURL string, body io.Reader, contentType string) (*http.Response, error) {
	return b.doWithHeaders(method, rawURL, body, contentType, nil)
}

func (b *publicBrowser) doWithHeaders(method, rawURL string, body io.Reader, contentType string, headers http.Header) (*http.Response, error) {
	publicURL, err := url.Parse(rawURL)
	if err != nil || publicURL.Scheme != "https" || publicURL.Host == "" {
		return nil, fmt.Errorf("invalid public browser URL %q", rawURL)
	}
	address, ok := mapPublicHost(b.harness, publicURL.Host)
	if !ok {
		return nil, fmt.Errorf("unmapped public host %q", publicURL.Host)
	}
	target := *publicURL
	target.Scheme, target.Host = "http", address
	request, err := http.NewRequest(method, target.String(), body)
	if err != nil {
		return nil, err
	}
	request.Host = publicURL.Host
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", publicURL.Host)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	if method != http.MethodGet && method != http.MethodHead {
		request.Header.Set("Origin", publicURL.Scheme+"://"+publicURL.Host)
	}
	if values := b.cookies[publicURL.Host]; len(values) > 0 {
		cookies := make([]string, 0, len(values))
		for _, cookie := range values {
			cookies = append(cookies, cookie.Name+"="+cookie.Value)
		}
		request.Header.Set("Cookie", strings.Join(cookies, "; "))
	}
	response, err := b.client.Do(request)
	if err != nil {
		return nil, err
	}
	b.storeCookies(publicURL.Host, response.Cookies())
	return response, nil
}

func assertMessageListContains(t *testing.T, browser *publicBrowser, message string) {
	t.Helper()
	response := browser.get(t, messagePublicOrigin+"/api/messages")
	requireStatus(t, response, http.StatusOK)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatalf("read message list: %v", err)
	}
	if !strings.Contains(string(body), message) {
		t.Fatalf("message list does not contain %q: %s", message, body)
	}
}

func assertMessageListExcludes(t *testing.T, browser *publicBrowser, message string) {
	t.Helper()
	response := browser.get(t, messagePublicOrigin+"/api/messages")
	requireStatus(t, response, http.StatusOK)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatalf("read message list: %v", err)
	}
	if strings.Contains(string(body), message) {
		t.Fatalf("message list unexpectedly contains %q: %s", message, body)
	}
}

func mapPublicHost(harness *harness, host string) (string, bool) {
	switch host {
	case "idp.example.test":
		return harness.idpAddress, true
	case "message.example.test":
		return harness.messageAddress, true
	default:
		return "", false
	}
}

func (b *publicBrowser) storeCookies(host string, cookies []*http.Cookie) {
	if b.cookies[host] == nil {
		b.cookies[host] = map[string]*http.Cookie{}
	}
	for _, cookie := range cookies {
		if cookie.MaxAge < 0 {
			delete(b.cookies[host], cookie.Name)
			continue
		}
		cookieCopy := *cookie
		b.cookies[host][cookie.Name] = &cookieCopy
	}
}

func assertRegistrationAuthorization(t *testing.T, rawURL string) {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "idp.example.test" || parsed.Path != "/idp/authorize" {
		t.Fatalf("registration authorize URL = %q", rawURL)
	}
	query := parsed.Query()
	if query.Get("client_id") != "tinyidp-message-app" || query.Get("redirect_uri") != messagePublicOrigin+"/auth/callback" || query.Get("code_challenge_method") != "S256" || query.Get("tinyidp_signup") != "1" {
		t.Fatalf("registration authorization contract = %s", query.Encode())
	}
	for _, required := range []string{"state", "nonce", "code_challenge"} {
		if query.Get(required) == "" {
			t.Fatalf("registration authorization missing %s: %s", required, query.Encode())
		}
	}
	scopes := map[string]bool{}
	for _, scope := range strings.Fields(query.Get("scope")) {
		scopes[scope] = true
	}
	if !scopes["openid"] || !scopes["profile"] {
		t.Fatalf("registration scopes = %q", query.Get("scope"))
	}
}

func requireStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if response.StatusCode == want {
		return
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	t.Fatalf("response status = %d, want %d: %s", response.StatusCode, want, body)
}

func requiredLocation(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	location := response.Header.Get("Location")
	if location == "" {
		t.Fatalf("response %d has no Location", response.StatusCode)
	}
	return location
}

func hiddenFormValue(t *testing.T, page []byte, name string) string {
	t.Helper()
	re := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]+)"`)
	matches := re.FindStringSubmatch(string(page))
	if len(matches) != 2 {
		t.Fatalf("hidden %q not found in signup page: %s", name, page)
	}
	return matches[1]
}

func neturl(origin string) (string, error) {
	if !strings.HasPrefix(origin, "https://") || strings.Contains(strings.TrimPrefix(origin, "https://"), "/") {
		return "", fmt.Errorf("invalid test public origin %q", origin)
	}
	return strings.TrimPrefix(origin, "https://"), nil
}

func (h *harness) requireAudit(processName, event string) {
	h.t.Helper()
	path := h.idpAudit
	if processName != "tinyidp" {
		h.t.Fatalf("unknown audit process %q", processName)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(readFile(path), event) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	h.t.Fatalf("audit %s does not contain %q: %s", processName, event, readFile(path))
}

func (h *harness) requireLog(processName, fragment string) {
	h.t.Helper()
	path := h.idpLog
	if processName == "message-desk" {
		path = h.messageLog
	} else if processName != "tinyidp" {
		h.t.Fatalf("unknown log process %q", processName)
	}
	if !strings.Contains(readFile(path), fragment) {
		h.t.Fatalf("%s log does not contain %q: %s", processName, fragment, readFile(path))
	}
}

func (h *harness) runForeground(label, binary string, args ...string) {
	h.t.Helper()
	context, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(context, binary, args...)
	command.Dir = h.repo
	output, err := command.CombinedOutput()
	if err != nil {
		h.t.Fatalf("%s: %v\n%s", label, err, output)
	}
}

type startedProcess struct {
	name     string
	command  *exec.Cmd
	logFile  *os.File
	logPath  string
	stopOnce sync.Once
}

func (p *startedProcess) stop(t *testing.T) {
	t.Helper()
	p.stopOnce.Do(func() { p.stopOnceOnly(t) })
}

func (p *startedProcess) stopOnceOnly(t *testing.T) {
	t.Helper()
	if p == nil || p.command == nil || p.command.Process == nil {
		return
	}
	_ = p.command.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- p.command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("stop %s: %v\n%s", p.name, err, readFile(p.logPath))
		}
	case <-time.After(10 * time.Second):
		_ = p.command.Process.Kill()
		<-done
		t.Errorf("stop %s: forced kill after graceful shutdown deadline\n%s", p.name, readFile(p.logPath))
	}
	if p.logFile != nil {
		if err := p.logFile.Close(); err != nil {
			t.Errorf("close %s log: %v", p.name, err)
		}
	}
}

func readFile(path string) string {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "<unavailable: " + err.Error() + ">"
	}
	return string(contents)
}
