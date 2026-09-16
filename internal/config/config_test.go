package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("base_url: https://file.example\naccount_id: \"1234\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvPrefix+"_BASE_URL", "https://env.example/")
	t.Setenv(EnvPrefix+"_ACCOUNT_ID", "5678")
	t.Setenv(EnvPrefix+"_USERNAME", "user@example.com")
	t.Setenv(EnvPrefix+"_PASSWORD", "secret")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://env.example" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.AccountID != "5678" {
		t.Fatalf("AccountID = %q", cfg.AccountID)
	}
	if cfg.Email != "user@example.com" {
		t.Fatalf("Email = %q", cfg.Email)
	}
	if cfg.Password != "secret" {
		t.Fatal("Password not set from env")
	}
}

func TestLoadEDCLoginFallback(t *testing.T) {
	t.Setenv("EDC_LOGIN", "dance@example.com")
	t.Setenv("EDC_PASSWORD", "pw")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Email != "dance@example.com" {
		t.Fatalf("Email = %q", cfg.Email)
	}
	if cfg.Password != "pw" {
		t.Fatal("Password not set from EDC_PASSWORD")
	}
}

func TestValidateRequiresCredentials(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty email")
	} else if !strings.Contains(err.Error(), "email") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSaveWritesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	cfg := Config{BaseURL: "https://api.example.com", AccountID: "30834"}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("mode = %v", got)
	}
}
