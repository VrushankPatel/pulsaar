package config

import (
	"os"
	"testing"
)

func mustSetEnv(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set %s: %v", key, err)
	}
}

func mustUnsetEnv(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset %s: %v", key, err)
	}
}

func TestConfigIntegrationDefaults(t *testing.T) {
	// Setup test environment variables
	mustSetEnv(t, "PULSAAR_TLS_CERT_FILE", "/path/to/cert")
	mustSetEnv(t, "PULSAAR_TLS_KEY_FILE", "/path/to/key")
	mustSetEnv(t, "PULSAAR_TLS_CA_FILE", "/path/to/ca")
	mustSetEnv(t, "PULSAAR_RATE_LIMIT_OPS_PER_SEC", "25")
	mustSetEnv(t, "PULSAAR_DENIED_PATHS", "/etc,/root")
	mustSetEnv(t, "PULSAAR_ALLOWED_ROOTS", "/var/log,/var/tmp")

	cfg, err := LoadAgentConfig()
	if err != nil {
		t.Fatalf("failed to load agent config: %v", err)
	}

	if cfg.TLSCertFile != "/path/to/cert" {
		t.Errorf("expected TLSCertFile /path/to/cert, got %s", cfg.TLSCertFile)
	}
	if cfg.RateLimitOpsPerSec != 25 {
		t.Errorf("expected RateLimitOpsPerSec 25, got %d", cfg.RateLimitOpsPerSec)
	}
	if len(cfg.DeniedPaths) != 2 || cfg.DeniedPaths[0] != "/etc" || cfg.DeniedPaths[1] != "/root" {
		t.Errorf("unexpected DeniedPaths: %v", cfg.DeniedPaths)
	}
	if len(cfg.AllowedRoots) != 2 || cfg.AllowedRoots[0] != "/var/log" || cfg.AllowedRoots[1] != "/var/tmp" {
		t.Errorf("unexpected AllowedRoots: %v", cfg.AllowedRoots)
	}

	// Clean up environment variables
	mustUnsetEnv(t, "PULSAAR_TLS_CERT_FILE")
	mustUnsetEnv(t, "PULSAAR_TLS_KEY_FILE")
	mustUnsetEnv(t, "PULSAAR_TLS_CA_FILE")
	mustUnsetEnv(t, "PULSAAR_RATE_LIMIT_OPS_PER_SEC")
	mustUnsetEnv(t, "PULSAAR_DENIED_PATHS")
	mustUnsetEnv(t, "PULSAAR_ALLOWED_ROOTS")
}

func TestConfigLoadFallback(t *testing.T) {
	// Test default fallback logic
	mustUnsetEnv(t, "PULSAAR_ALLOWED_ROOTS")
	mustUnsetEnv(t, "PULSAAR_DENIED_PATHS")
	mustUnsetEnv(t, "PULSAAR_RATE_LIMIT_OPS_PER_SEC")

	cfg, err := LoadAgentConfig()
	if err != nil {
		t.Fatalf("failed to load agent config: %v", err)
	}

	if cfg.RateLimitOpsPerSec != 10 {
		t.Errorf("expected default rate limit 10, got %d", cfg.RateLimitOpsPerSec)
	}

	if len(cfg.AllowedRoots) != 1 || cfg.AllowedRoots[0] != "/" {
		t.Errorf("expected default allowed roots [/], got %v", cfg.AllowedRoots)
	}

	if len(cfg.DeniedPaths) != 0 {
		t.Errorf("expected empty default denied paths, got %v", cfg.DeniedPaths)
	}
}
