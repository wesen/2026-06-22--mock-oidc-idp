// Package production defines the reusable Glazed configuration section for
// the durable TinyIDP production host.
package production

import (
	"fmt"

	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
)

const Slug = "production"

// Settings is the complete non-secret production configuration. Secret
// settings are file references; secret contents never enter Glazed values.
type Settings struct {
	Addr                    string   `glazed:"addr"`
	AdminAddr               string   `glazed:"admin-addr"`
	ListenerMode            string   `glazed:"listener-mode"`
	Issuer                  string   `glazed:"issuer"`
	ClientsFile             string   `glazed:"clients-file"`
	ThemeDir                string   `glazed:"theme-dir"`
	ThemeCatalogFile        string   `glazed:"theme-catalog-file"`
	SignupProgramFile       string   `glazed:"signup-program-file"`
	DBPath                  string   `glazed:"db"`
	AuditPath               string   `glazed:"audit-path"`
	TokenSecretFile         string   `glazed:"token-secret-file"`
	AdminAuthKeyFile        string   `glazed:"admin-auth-key-file"`
	AdminActionKeyFile      string   `glazed:"admin-action-key-file"`
	InvitationKeyFile       string   `glazed:"invitation-lookup-key-file"`
	EmailChallengeKeyFile   string   `glazed:"email-challenge-key-file"`
	EmailSMTPAddress        string   `glazed:"email-smtp-address"`
	EmailSMTPTLSMode        string   `glazed:"email-smtp-tls-mode"`
	EmailSMTPServerName     string   `glazed:"email-smtp-server-name"`
	EmailSMTPUsername       string   `glazed:"email-smtp-username"`
	EmailSMTPPasswordFile   string   `glazed:"email-smtp-password-file"`
	EmailFromAddress        string   `glazed:"email-from-address"`
	EmailFromName           string   `glazed:"email-from-name"`
	EmailSMTPConnectTimeout string   `glazed:"email-smtp-connect-timeout"`
	EmailSMTPSendTimeout    string   `glazed:"email-smtp-send-timeout"`
	TLSCertFile             string   `glazed:"tls-cert"`
	TLSKeyFile              string   `glazed:"tls-key"`
	TrustedProxyCIDRs       []string `glazed:"trusted-proxy-cidrs"`
	MaxProxyHops            int      `glazed:"max-proxy-hops"`
	AccountChooser          bool     `glazed:"account-chooser"`
	RateLimit               int      `glazed:"rate-limit"`
	RateWindow              string   `glazed:"rate-window"`
	MaintenanceInterval     string   `glazed:"maintenance-interval"`
	ReadHeaderTimeout       string   `glazed:"read-header-timeout"`
	ReadTimeout             string   `glazed:"read-timeout"`
	WriteTimeout            string   `glazed:"write-timeout"`
	IdleTimeout             string   `glazed:"idle-timeout"`
	ShutdownTimeout         string   `glazed:"shutdown-timeout"`
	MaxRequestBytes         int      `glazed:"max-request-bytes"`
}

// NewSection declares production configuration once for the server and for
// configuration inspection commands.
func NewSection() (schema.Section, error) {
	return schema.NewSection(
		Slug,
		"Production host configuration",
		schema.WithFields(
			fields.New("addr", fields.TypeString, fields.WithDefault(":8443"), fields.WithHelp("Listener address")),
			fields.New("admin-addr", fields.TypeString, fields.WithDefault("127.0.0.1:9090"), fields.WithHelp("Internal health, readiness, and Prometheus listener address")),
			fields.New("listener-mode", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Required listener mode: direct-tls or trusted-proxy-http")),
			fields.New("issuer", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Canonical HTTPS issuer URL")),
			fields.New("clients-file", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Reviewed non-secret JSON catalog of exact production browser clients")),
			fields.New("theme-dir", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Read-only root containing reviewed production theme CSS")),
			fields.New("theme-catalog-file", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Reviewed non-secret JSON theme catalog inside --theme-dir")),
			fields.New("signup-program-file", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Reviewed non-secret JavaScript signup program; checked and activated before listening")),
			fields.New("db", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Provisioned SQLite database path")),
			fields.New("audit-path", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Synchronous JSONL audit path")),
			fields.New("token-secret-file", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Owner-only file containing at least 32 random bytes")),
			fields.New("admin-auth-key-file", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Owner-only file containing exactly 32 random bytes for administration sessions")),
			fields.New("admin-action-key-file", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Owner-only file containing at least 32 random bytes for administration action handles")),
			fields.New("invitation-lookup-key-file", fields.TypeString, fields.WithHelp("Owner-only 32-byte HMAC key; required when the signup program declares a durable invitation provider")),
			fields.New("email-challenge-key-file", fields.TypeString, fields.WithHelp("Owner-only 32-byte HMAC key; required when the signup program declares an email challenge")),
			fields.New("email-smtp-address", fields.TypeString, fields.WithHelp("SMTP submission host:port; required for email-challenge signup")),
			fields.New("email-smtp-tls-mode", fields.TypeString, fields.WithHelp("SMTP transport: starttls, implicit, or private-plaintext")),
			fields.New("email-smtp-server-name", fields.TypeString, fields.WithHelp("Optional TLS server name; defaults to the SMTP address host")),
			fields.New("email-smtp-username", fields.TypeString, fields.WithHelp("Optional SMTP username; requires --email-smtp-password-file and TLS")),
			fields.New("email-smtp-password-file", fields.TypeString, fields.WithHelp("Owner-only SMTP password file; required with --email-smtp-username")),
			fields.New("email-from-address", fields.TypeString, fields.WithHelp("Fixed sender mailbox for email challenges")),
			fields.New("email-from-name", fields.TypeString, fields.WithDefault("TinyIDP"), fields.WithHelp("Fixed sender display name")),
			fields.New("email-smtp-connect-timeout", fields.TypeString, fields.WithDefault("5s"), fields.WithHelp("SMTP connection timeout")),
			fields.New("email-smtp-send-timeout", fields.TypeString, fields.WithDefault("15s"), fields.WithHelp("Complete SMTP exchange timeout")),
			fields.New("tls-cert", fields.TypeString, fields.WithHelp("TLS certificate PEM path; required only for direct-tls")),
			fields.New("tls-key", fields.TypeString, fields.WithHelp("TLS private-key PEM path; required only for direct-tls")),
			fields.New("trusted-proxy-cidrs", fields.TypeStringList, fields.WithHelp("Required only for trusted-proxy-http; narrow CIDRs allowed to supply forwarded metadata")),
			fields.New("max-proxy-hops", fields.TypeInteger, fields.WithDefault(8), fields.WithHelp("Maximum accepted forwarded-address hops")),
			fields.New("account-chooser", fields.TypeBool, fields.WithHelp("Offer remembered signed-in accounts when an OIDC client requests prompt=select_account")),
			fields.New("rate-limit", fields.TypeInteger, fields.WithDefault(30), fields.WithHelp("Login attempts per account/client/address bucket and window")),
			fields.New("rate-window", fields.TypeString, fields.WithDefault("1m"), fields.WithHelp("Login rate-limit window")),
			fields.New("maintenance-interval", fields.TypeString, fields.WithDefault("15m"), fields.WithHelp("Retention maintenance interval")),
			fields.New("read-header-timeout", fields.TypeString, fields.WithDefault("5s"), fields.WithHelp("HTTP header read timeout")),
			fields.New("read-timeout", fields.TypeString, fields.WithDefault("15s"), fields.WithHelp("HTTP request read timeout")),
			fields.New("write-timeout", fields.TypeString, fields.WithDefault("30s"), fields.WithHelp("HTTP response write timeout")),
			fields.New("idle-timeout", fields.TypeString, fields.WithDefault("1m"), fields.WithHelp("HTTP keep-alive idle timeout")),
			fields.New("shutdown-timeout", fields.TypeString, fields.WithDefault("20s"), fields.WithHelp("Graceful shutdown deadline")),
			fields.New("max-request-bytes", fields.TypeInteger, fields.WithDefault(1<<20), fields.WithHelp("Maximum request body size")),
		),
	)
}

func GetSettings(vals *values.Values) (*Settings, error) {
	settings := &Settings{}
	if err := vals.DecodeSectionInto(Slug, settings); err != nil {
		return nil, fmt.Errorf("decode %s settings: %w", Slug, err)
	}
	return settings, nil
}
