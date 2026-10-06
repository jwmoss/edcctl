package tests

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Install from a local module proxy to test the metadata from go install @version.
func TestModuleVersionFallback(t *testing.T) {
	const module = "github.com/jwmoss/edcctl"
	const moduleVersion = "v0.0.0-test"
	dir := t.TempDir()
	proxy := filepath.Join(dir, "proxy")
	root := filepath.Join("..")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := func(env []string, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = env
		return cmd
	}
	env := []string{}
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		switch key {
		case "GOPROXY", "GOSUMDB", "GOMODCACHE", "GOBIN", "GOWORK", "GOTOOLCHAIN", "GOPRIVATE", "GONOPROXY", "GONOSUMDB", "GOFLAGS":
			continue
		}
		if strings.HasPrefix(key, "EDCCTL_") || key == "EDC_LOGIN" || key == "EDC_PASSWORD" {
			continue
		}
		env = append(env, value)
	}
	// Reuse only dependency archives already needed by this test run.
	cache, err := exec.CommandContext(ctx, "go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatal(err)
	}
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if rel != "." && rel != "cmd" && rel != "internal" && !strings.HasPrefix(rel, "cmd"+string(filepath.Separator)) && !strings.HasPrefix(rel, "internal"+string(filepath.Separator)) {
				return filepath.SkipDir
			}
			return nil
		}
		if rel != "go.mod" && rel != "go.sum" && (!strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go")) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := writer.Create(module + "@" + moduleVersion + "/" + filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		_, err = file.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(proxy, module, "@v", moduleVersion)
	write(base+".mod", mod)
	write(base+".info", []byte(`{"Version":"v0.0.0-test","Time":"2026-01-01T00:00:00Z"}`))
	write(base+".zip", archive.Bytes())
	write(filepath.Join(proxy, module, "@v", "list"), []byte(moduleVersion+"\n"))
	fileURL := func(path string) string {
		path = filepath.ToSlash(path)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return (&url.URL{Scheme: "file", Path: path}).String()
	}
	proxies := fileURL(proxy) + "," + fileURL(filepath.Join(strings.TrimSpace(string(cache)), "cache", "download"))
	env = append(env, "GOPROXY="+proxies, "GOSUMDB=off", "GOMODCACHE="+filepath.Join(dir, "cache"), "GOBIN="+dir, "GOWORK=off", "GOTOOLCHAIN=local", "GOFLAGS=-modcacherw", "GOPRIVATE=", "GONOPROXY=", "GONOSUMDB=", "GOVCS=*:off")
	binary := filepath.Join(dir, "edcctl")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	for _, linkedVersion := range []string{"", "v1.2.3-linked"} {
		args := []string{"install"}
		want := moduleVersion
		if linkedVersion != "" {
			want = linkedVersion
			args = append(args, "-ldflags=-X "+module+"/internal/cli.version="+linkedVersion)
		}
		args = append(args, module+"/cmd/edcctl@"+moduleVersion)
		if output, err := command(env, "go", args...).CombinedOutput(); err != nil {
			t.Fatalf("local go install: %v\n%s", err, output)
		}
		for _, args := range [][]string{{"--json", "version"}, {"--plain", "--version"}} {
			cmd := command(env, binary, append([]string{"--config", dir}, args...)...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil || stderr.Len() != 0 {
				t.Fatalf("version: %v stderr=%s", err, stderr.String())
			}
			got := strings.TrimSpace(stdout.String())
			if args[0] == "--json" {
				var receipt map[string]string
				if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
					t.Fatal(err)
				}
				got = receipt["version"]
			}
			if got != want {
				t.Errorf("args=%v version=%q, want %q", args, got, want)
			}
		}
	}
}
