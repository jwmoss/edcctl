package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), []string{"--json", "version"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"version": "dev"`) {
		t.Fatalf("stdout=%s", stdout.String())
	}
}

func TestVersionFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), []string{"--version"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "version dev") {
		t.Fatalf("stdout=%s", stdout.String())
	}
}

func TestCompletionUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), []string{"completion"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestScheduleUsageBeforeLogin(t *testing.T) {
	// If validation reaches login, this unreachable endpoint causes an error exit instead of a usage exit.
	t.Setenv("EDCCTL_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("EDCCTL_USERNAME", "test@example.test")
	t.Setenv("EDCCTL_PASSWORD", "test")
	for _, args := range [][]string{
		{"schedule", "--from", "2026-09-28"},
		{"schedule", "--from", "not-a-date", "--to", "2026-10-05"},
		{"schedule", "--from", "2026-09-28", "--to", "2026-09-28"},
		{"schedule", "--from", "2026-09-28", "--to", "2026-10-05", "--week", "2"},
		{"schedule", "unexpected-argument"},
	} {
		var stdout, stderr bytes.Buffer
		code := Execute(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
		if code != exitUsage {
			t.Fatalf("args=%v code=%d stderr=%s", args, code, stderr.String())
		}
	}
}

func TestUsageExitCodes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("base_url: [broken"), 0600); err != nil {
		t.Fatal(err)
	}
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	http.DefaultTransport = appRoundTripper(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("invalid usage reached network: %s", r.URL.Path)
		return nil, nil
	})
	for _, args := range [][]string{
		{"unknown-command"}, {"--unknown"}, {"version", "--unknown"},
		{"version", "--json=bad"}, {"version", "--timeout", "bad"},
		{"version", "extra"}, {"--version", "extra"},
		{"config", "show", "extra"}, {"config", "init", "extra"},
		{"config", "unknown-command"}, {"app", "unknown-command"},
		{"login", "extra"}, {"doctor", "extra"}, {"completion", "bash", "extra"},
		{"login", "--version"}, {"app", "info", "--version"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Execute(context.Background(), append([]string{"--config", path}, args...), strings.NewReader(""), &stdout, &stderr)
			if code != exitUsage || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestLocalOutputConflicts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("base_url: [broken"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--version"}, {"version"}, {"config", "show"}, {"config", "init", "--force"}, {"completion", "bash"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			argv := append([]string{"--config", path, "--json", "--plain"}, args...)
			if code := Execute(context.Background(), argv, strings.NewReader(""), &stdout, &stderr); code != exitUsage || stdout.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), "choose only one") {
				t.Fatalf("wrong error: %s", stderr.String())
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "base_url: [broken" {
				t.Fatal("format conflict changed config")
			}
		})
	}
}

func TestVersionIgnoresConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("base_url: [broken"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--version"}, {"version", "--json"}, {"version", "--plain"}} {
		var stdout, stderr bytes.Buffer
		code := Execute(context.Background(), append([]string{"--config", path}, args...), strings.NewReader(""), &stdout, &stderr)
		if code != exitOK || stdout.Len() == 0 || stderr.Len() != 0 {
			t.Fatalf("args=%v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestConfigSelectedPath(t *testing.T) {
	for _, key := range []string{"EDCCTL_BASE_URL", "EDCCTL_ACCOUNT_ID", "EDCCTL_USERNAME", "EDCCTL_PASSWORD", "EDC_LOGIN", "EDC_PASSWORD"} {
		t.Setenv(key, "")
	}
	path := filepath.Join(t.TempDir(), "selected.yaml")
	if err := os.WriteFile(path, []byte("email: parent@example.test\npassword: dummy-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"--json", "--plain", "--no-color"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Execute(context.Background(), []string{"--config", path, mode, "config", "show"}, strings.NewReader(""), &stdout, &stderr)
			if code != exitOK || stderr.Len() != 0 || strings.Contains(stdout.String(), "dummy-secret") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if mode == "--json" {
				var cfg map[string]string
				if err := json.Unmarshal(stdout.Bytes(), &cfg); err != nil || cfg["path"] != path || cfg["password"] != "redacted" {
					t.Fatalf("config show: %s, %v", stdout.String(), err)
				}
			} else if !strings.Contains(stdout.String(), path) {
				t.Fatalf("selected path absent: %s", stdout.String())
			}
		})
	}
}

func TestConfigInitJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), []string{"--config", path, "--json", "config", "init", "--password-stdin"}, strings.NewReader("dummy-secret\n"), &stdout, &stderr)
	var receipt map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil || code != exitOK || receipt["path"] != path || receipt["status"] != "written" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q JSON=%v", code, stdout.String(), stderr.String(), err)
	}
	if strings.Contains(stdout.String(), "dummy-secret") {
		t.Fatal("config receipt disclosed password")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte("password: dummy-secret")) {
		t.Fatal("config init did not save stdin password")
	}
}

type createOnRead struct {
	path string
	done bool
}

func (r *createOnRead) Read(p []byte) (int, error) {
	if !r.done {
		r.done = true
		if err := os.WriteFile(r.path, []byte("email: concurrent@example.test\n"), 0644); err != nil {
			return 0, err
		}
	}
	return 0, io.EOF
}

func TestConfigInitPreservesConcurrentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), []string{"--config", path, "config", "init", "--password-stdin"}, &createOnRead{path: path}, &stdout, &stderr)
	data, err := os.ReadFile(path)
	if code != exitErr || stdout.Len() != 0 || stderr.Len() == 0 || err != nil || string(data) != "email: concurrent@example.test\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q data=%q read=%v", code, stdout.String(), stderr.String(), data, err)
	}
}

func TestConfigInitDryRun(t *testing.T) {
	for _, existing := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		original := []byte("email: old@example.test\n")
		if existing {
			if err := os.WriteFile(path, original, 0644); err != nil {
				t.Fatal(err)
			}
		}
		stdin := &createOnRead{path: path}
		var stdout, stderr bytes.Buffer
		code := Execute(context.Background(), []string{"--config", path, "--dry-run", "config", "init", "--force", "--password-stdin", "--json"}, stdin, &stdout, &stderr)
		if code != exitErr || stdout.Len() != 0 || !strings.Contains(stderr.String(), "dry-run") || stdin.done {
			t.Fatalf("code=%d stdout=%q stderr=%q stdin read=%v", code, stdout.String(), stderr.String(), stdin.done)
		}
		data, err := os.ReadFile(path)
		if existing {
			info, statErr := os.Stat(path)
			if err != nil || statErr != nil || !bytes.Equal(data, original) || info.Mode().Perm() != 0644 {
				t.Fatal("dry-run changed the existing config")
			}
		} else if !os.IsNotExist(err) {
			t.Fatal("dry-run created a config")
		}
	}
}
