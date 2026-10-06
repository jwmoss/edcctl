package config

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
	if err := Save(path, cfg, false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); runtime.GOOS != "windows" && got != 0600 {
		t.Fatalf("mode = %v", got)
	}

	// Another hard link must retain the old file and its permissions.
	original := []byte("password: old-secret\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(filepath.Dir(path), "old.yaml")
	if err := os.Link(path, oldPath); err != nil {
		t.Fatal(err)
	}
	cfg.Password = "new-secret"
	if err := Save(path, cfg, true); err != nil {
		t.Fatal(err)
	}
	oldData, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	oldInfo, err := os.Stat(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	newInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	newData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(oldData, original) || (runtime.GOOS != "windows" && oldInfo.Mode().Perm() != 0644) {
		t.Fatal("replacement changed the old linked file")
	}
	if (runtime.GOOS != "windows" && newInfo.Mode().Perm() != 0600) || !bytes.Contains(newData, []byte("new-secret")) {
		t.Fatal("replacement did not publish a private config")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary config remains: %v, %v", entries, err)
	}
}

func TestSaveRefusesDestinationSymlink(t *testing.T) {
	for _, dangling := range []bool{false, true} {
		t.Run(strconv.FormatBool(dangling), func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "target.yaml")
			original := []byte("password: old-secret\n")
			if !dangling {
				if err := os.WriteFile(target, original, 0644); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(dir, "config.yaml")
			if err := os.Symlink(target, path); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if err := Save(path, Config{Password: "new-secret"}, true); err == nil {
				t.Error("save accepted a destination symlink")
			}
			data, err := os.ReadFile(target)
			if dangling {
				if !os.IsNotExist(err) {
					t.Fatal("save created the symlink target")
				}
			} else {
				info, statErr := os.Stat(target)
				if err != nil || statErr != nil || !bytes.Equal(data, original) || (runtime.GOOS != "windows" && info.Mode().Perm() != 0644) {
					t.Fatal("save changed the symlink target")
				}
			}
			if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatal("save replaced a refused symlink")
			}
		})
	}
}
