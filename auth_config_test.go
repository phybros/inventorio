package main

import "testing"

func TestLoadAuthConfigDefaultsToDisabled(t *testing.T) {
	clearAuthEnv(t)

	cfg, err := LoadAuthConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadAuthConfigFromEnv() error = %v", err)
	}
	if cfg.Mode != authModeDisabled {
		t.Fatalf("Mode = %q, want %q", cfg.Mode, authModeDisabled)
	}
}

func TestDisabledModeIgnoresOtherAuthConfig(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("INVENTORIO_AUTH_MODE", authModeDisabled)
	t.Setenv("INVENTORIO_ALLOWED_DOMAINS", "https://not-a-domain")
	t.Setenv("INVENTORIO_COOKIE_SECURE", "sometimes")

	if _, err := LoadAuthConfigFromEnv(); err != nil {
		t.Fatalf("LoadAuthConfigFromEnv() error = %v", err)
	}
}

func TestLoadAuthConfigRejectsInvalidMode(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("INVENTORIO_AUTH_MODE", "password")

	if _, err := LoadAuthConfigFromEnv(); err == nil {
		t.Fatal("LoadAuthConfigFromEnv() error = nil, want error")
	}
}

func TestOAuthRequiresProviderAndAllowlist(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("INVENTORIO_AUTH_MODE", authModeOAuth)
	t.Setenv("INVENTORIO_PUBLIC_URL", "https://inventory.example.com")
	t.Setenv("INVENTORIO_SESSION_SECRET", strongSessionSecret())

	if _, err := LoadAuthConfigFromEnv(); err == nil {
		t.Fatal("LoadAuthConfigFromEnv() error = nil, want error")
	}

	t.Setenv("INVENTORIO_GITHUB_CLIENT_ID", "id")
	t.Setenv("INVENTORIO_GITHUB_CLIENT_SECRET", "secret")
	if _, err := LoadAuthConfigFromEnv(); err == nil {
		t.Fatal("LoadAuthConfigFromEnv() without allowlist error = nil, want error")
	}
}

func TestOAuthAcceptsCompleteProviderAndAllowlist(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("INVENTORIO_AUTH_MODE", authModeOAuth)
	t.Setenv("INVENTORIO_PUBLIC_URL", "https://inventory.example.com")
	t.Setenv("INVENTORIO_SESSION_SECRET", strongSessionSecret())
	t.Setenv("INVENTORIO_GITHUB_CLIENT_ID", "id")
	t.Setenv("INVENTORIO_GITHUB_CLIENT_SECRET", "secret")
	t.Setenv("INVENTORIO_ALLOWED_EMAILS", " Alice@Example.com ")

	cfg, err := LoadAuthConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadAuthConfigFromEnv() error = %v", err)
	}
	if !cfg.EmailAllowed("alice@example.com") {
		t.Fatal("EmailAllowed returned false for case-insensitive allowed email")
	}
}

func TestPartialProviderConfigFails(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("INVENTORIO_AUTH_MODE", authModeOAuth)
	t.Setenv("INVENTORIO_PUBLIC_URL", "https://inventory.example.com")
	t.Setenv("INVENTORIO_SESSION_SECRET", strongSessionSecret())
	t.Setenv("INVENTORIO_GITHUB_CLIENT_ID", "id")
	t.Setenv("INVENTORIO_ALLOWED_DOMAINS", "example.com")

	if _, err := LoadAuthConfigFromEnv(); err == nil {
		t.Fatal("LoadAuthConfigFromEnv() error = nil, want error")
	}
}

func TestDomainAllowlistIsCaseInsensitive(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("INVENTORIO_AUTH_MODE", authModeProxy)
	t.Setenv("INVENTORIO_ALLOWED_DOMAINS", "Example.COM")

	cfg, err := LoadAuthConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadAuthConfigFromEnv() error = %v", err)
	}
	if !cfg.EmailAllowed("ALICE@example.com") {
		t.Fatal("EmailAllowed returned false for allowed domain")
	}
}

func TestOAuthRejectsWeakSessionSecret(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("INVENTORIO_AUTH_MODE", authModeOAuth)
	t.Setenv("INVENTORIO_PUBLIC_URL", "https://inventory.example.com")
	t.Setenv("INVENTORIO_SESSION_SECRET", "short-secret")
	t.Setenv("INVENTORIO_GITHUB_CLIENT_ID", "id")
	t.Setenv("INVENTORIO_GITHUB_CLIENT_SECRET", "secret")
	t.Setenv("INVENTORIO_ALLOWED_EMAILS", "alice@example.com")

	if _, err := LoadAuthConfigFromEnv(); err == nil {
		t.Fatal("LoadAuthConfigFromEnv() error = nil, want error")
	}
}

func TestInvalidAllowlistEntriesFail(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("INVENTORIO_AUTH_MODE", authModeProxy)
	t.Setenv("INVENTORIO_ALLOWED_DOMAINS", "https://example.com")

	if _, err := LoadAuthConfigFromEnv(); err == nil {
		t.Fatal("LoadAuthConfigFromEnv() error = nil, want error")
	}
}

func TestSanitizeNext(t *testing.T) {
	tests := map[string]string{
		"":                       "/",
		"/components":            "/components",
		"/components?q=abc":      "/components?q=abc",
		"//evil.example.com":     "/",
		"https://evil.example/x": "/",
		"components":             "/",
	}
	for in, want := range tests {
		if got := sanitizeNext(in); got != want {
			t.Fatalf("sanitizeNext(%q) = %q, want %q", in, got, want)
		}
	}
}

func clearAuthEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"INVENTORIO_AUTH_MODE",
		"INVENTORIO_PUBLIC_URL",
		"INVENTORIO_SESSION_SECRET",
		"INVENTORIO_GITHUB_CLIENT_ID",
		"INVENTORIO_GITHUB_CLIENT_SECRET",
		"INVENTORIO_GOOGLE_CLIENT_ID",
		"INVENTORIO_GOOGLE_CLIENT_SECRET",
		"INVENTORIO_ALLOWED_EMAILS",
		"INVENTORIO_ALLOWED_DOMAINS",
		"INVENTORIO_AUTH_ALLOW_ALL_USERS",
		"INVENTORIO_SESSION_COOKIE_NAME",
		"INVENTORIO_COOKIE_SECURE",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
}

func strongSessionSecret() string {
	return "0123456789abcdef0123456789abcdef"
}
