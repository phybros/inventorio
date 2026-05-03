package main

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strings"
)

const (
	authModeDisabled = "disabled"
	authModeOAuth    = "oauth"
	authModeProxy    = "proxy"

	defaultProxyAuthHeader = "X-Forwarded-User"
	minSessionSecretLength = 32
)

type AuthConfig struct {
	Mode              string
	PublicURL         string
	SessionSecret     string
	SessionCookieName string
	CookieSecure      string
	ProxyAuthHeader   string
	AllowAllUsers     bool
	AllowedEmails     map[string]struct{}
	AllowedDomains    map[string]struct{}
	GitHub            OAuthProviderConfig
	Google            OAuthProviderConfig
}

type OAuthProviderConfig struct {
	ClientID     string
	ClientSecret string
}

func LoadAuthConfigFromEnv() (*AuthConfig, error) {
	cfg := &AuthConfig{
		Mode:              envOrDefault("INVENTORIO_AUTH_MODE", authModeDisabled),
		PublicURL:         strings.TrimRight(strings.TrimSpace(os.Getenv("INVENTORIO_PUBLIC_URL")), "/"),
		SessionSecret:     strings.TrimSpace(os.Getenv("INVENTORIO_SESSION_SECRET")),
		SessionCookieName: envOrDefault("INVENTORIO_SESSION_COOKIE_NAME", "inventorio_session"),
		CookieSecure:      envOrDefault("INVENTORIO_COOKIE_SECURE", "auto"),
		ProxyAuthHeader:   envOrDefault("INVENTORIO_PROXY_AUTH_HEADER", defaultProxyAuthHeader),
		AllowAllUsers:     strings.EqualFold(strings.TrimSpace(os.Getenv("INVENTORIO_AUTH_ALLOW_ALL_USERS")), "true"),
		GitHub: OAuthProviderConfig{
			ClientID:     os.Getenv("INVENTORIO_GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("INVENTORIO_GITHUB_CLIENT_SECRET"),
		},
		Google: OAuthProviderConfig{
			ClientID:     os.Getenv("INVENTORIO_GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("INVENTORIO_GOOGLE_CLIENT_SECRET"),
		},
	}

	if cfg.Mode != authModeDisabled {
		emails, err := parseAllowedEmails(os.Getenv("INVENTORIO_ALLOWED_EMAILS"))
		if err != nil {
			return nil, err
		}
		cfg.AllowedEmails = emails

		domains, err := parseAllowedDomains(os.Getenv("INVENTORIO_ALLOWED_DOMAINS"))
		if err != nil {
			return nil, err
		}
		cfg.AllowedDomains = domains
	} else {
		cfg.AllowedEmails = map[string]struct{}{}
		cfg.AllowedDomains = map[string]struct{}{}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *AuthConfig) Validate() error {
	switch c.Mode {
	case authModeDisabled, authModeOAuth, authModeProxy:
	default:
		return fmt.Errorf("INVENTORIO_AUTH_MODE must be disabled, oauth, or proxy")
	}

	if c.Mode == authModeDisabled {
		return nil
	}

	if c.SessionCookieName == "" {
		return fmt.Errorf("INVENTORIO_SESSION_COOKIE_NAME must not be empty")
	}

	switch c.CookieSecure {
	case "auto", "true", "false":
	default:
		return fmt.Errorf("INVENTORIO_COOKIE_SECURE must be auto, true, or false")
	}

	if !c.AllowAllUsers && len(c.AllowedEmails) == 0 && len(c.AllowedDomains) == 0 {
		return fmt.Errorf("authentication requires INVENTORIO_ALLOWED_EMAILS, INVENTORIO_ALLOWED_DOMAINS, or INVENTORIO_AUTH_ALLOW_ALL_USERS=true")
	}

	if c.Mode == authModeProxy {
		if c.ProxyAuthHeader == "" {
			return fmt.Errorf("INVENTORIO_PROXY_AUTH_HEADER must not be empty")
		}
		if !validHTTPHeaderName(c.ProxyAuthHeader) {
			return fmt.Errorf("INVENTORIO_PROXY_AUTH_HEADER must be a valid HTTP header name")
		}
		return nil
	}

	if c.PublicURL == "" {
		return fmt.Errorf("oauth mode requires INVENTORIO_PUBLIC_URL")
	}
	if _, err := url.ParseRequestURI(c.PublicURL); err != nil {
		return fmt.Errorf("INVENTORIO_PUBLIC_URL is invalid: %w", err)
	}
	if c.SessionSecret == "" {
		return fmt.Errorf("oauth mode requires INVENTORIO_SESSION_SECRET")
	}
	if len(c.SessionSecret) < minSessionSecretLength {
		return fmt.Errorf("INVENTORIO_SESSION_SECRET must be at least %d characters", minSessionSecretLength)
	}

	githubPartial := (c.GitHub.ClientID == "") != (c.GitHub.ClientSecret == "")
	googlePartial := (c.Google.ClientID == "") != (c.Google.ClientSecret == "")
	if githubPartial || googlePartial {
		return fmt.Errorf("oauth provider configuration must include both client ID and client secret")
	}
	if !c.ProviderConfigured("github") && !c.ProviderConfigured("google") {
		return fmt.Errorf("oauth mode requires at least one configured provider")
	}

	return nil
}

func (c *AuthConfig) ProviderConfigured(provider string) bool {
	switch provider {
	case "github":
		return c.GitHub.ClientID != "" && c.GitHub.ClientSecret != ""
	case "google":
		return c.Google.ClientID != "" && c.Google.ClientSecret != ""
	default:
		return false
	}
}

func (c *AuthConfig) Provider(provider string) OAuthProviderConfig {
	switch provider {
	case "github":
		return c.GitHub
	case "google":
		return c.Google
	default:
		return OAuthProviderConfig{}
	}
}

func (c *AuthConfig) ProxyHeaderName() string {
	if c.ProxyAuthHeader == "" {
		return defaultProxyAuthHeader
	}
	return c.ProxyAuthHeader
}

func (c *AuthConfig) EmailAllowed(email string) bool {
	if c.AllowAllUsers {
		return true
	}
	normalized, err := normalizeEmail(email)
	if err != nil {
		return false
	}
	if _, ok := c.AllowedEmails[normalized]; ok {
		return true
	}
	domain := emailDomain(normalized)
	_, ok := c.AllowedDomains[domain]
	return ok
}

func parseAllowedEmails(raw string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		email, err := normalizeEmail(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid allowlist email %q: %w", entry, err)
		}
		out[email] = struct{}{}
	}
	return out, nil
}

func parseAllowedDomains(raw string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(strings.ToLower(entry))
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "@") || strings.Contains(entry, "://") || strings.HasPrefix(entry, ".") || strings.HasSuffix(entry, ".") || !strings.Contains(entry, ".") {
			return nil, fmt.Errorf("invalid allowlist domain %q", entry)
		}
		for _, part := range strings.Split(entry, ".") {
			if part == "" || strings.ContainsAny(part, " \t\r\n") {
				return nil, fmt.Errorf("invalid allowlist domain %q", entry)
			}
		}
		out[entry] = struct{}{}
	}
	return out, nil
}

func validHTTPHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case strings.ContainsRune("!#$%&'*+-.^_`|~", r):
		default:
			return false
		}
	}
	return true
}

func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", errors.New("email is empty")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil {
		return "", err
	}
	if parsed.Address != email {
		return "", errors.New("email must be a plain address")
	}
	if strings.Count(email, "@") != 1 || strings.HasPrefix(email, "@") || strings.HasSuffix(email, "@") {
		return "", errors.New("email must contain a local part and domain")
	}
	if !strings.Contains(emailDomain(email), ".") {
		return "", errors.New("email domain must contain a dot")
	}
	return email, nil
}

func emailDomain(email string) string {
	idx := strings.LastIndex(email, "@")
	if idx < 0 {
		return ""
	}
	return strings.ToLower(email[idx+1:])
}

func sanitizeNext(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return "/"
	}
	if u, err := url.Parse(raw); err != nil || u.Scheme != "" || u.Host != "" {
		return "/"
	}
	return raw
}
