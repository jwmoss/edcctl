package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHTMLResourceCommandsAreAbsent(t *testing.T) {
	root := newRootCommand(&runtime{g: &globals{}})
	for _, cmd := range root.Commands() {
		switch cmd.Name() {
		case "students", "balance", "history", "announcements", "files", "account":
			t.Errorf("HTML resource command still exposed: %s", cmd.Name())
		}
	}
}

func TestAuthenticatedJSONQueries(t *testing.T) {
	for _, command := range []string{"schedule", "doctor"} {
		t.Run(command, func(t *testing.T) {
			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.Path)
				switch {
				case r.Method == "GET" && r.URL.Path == "/index.php":
					http.SetCookie(w, &http.Cookie{Name: "session", Value: "synthetic"})
					w.Write([]byte(`<form><input name="email"><input name="password"><input name="csrf_token" value="synthetic-token"></form>`))
				case r.Method == "POST" && r.URL.Path == "/index.php":
					r.ParseForm()
					if r.PostForm.Get("email") != "user@example.test" || r.PostForm.Get("password") != "test-password" || r.PostForm.Get("csrf_token") != "synthetic-token" {
						t.Error("login form mismatch")
					}
					w.Write([]byte(`messageType:"logininfo" app_my_class_schedule.php`))
				case r.Method == "POST" && r.URL.Path == "/class_calendar-ajax.php":
					if r.Header.Get("Accept") != "application/json" {
						t.Error("did not request JSON")
					}
					if r.Header.Get("X-CSRF-Token") != "synthetic-token" {
						t.Error("missing CSRF header")
					}
					if cookie, err := r.Cookie("session"); err != nil || cookie.Value != "synthetic" {
						t.Error("missing session")
					}
					w.Header().Set("Content-Type", "text/html")
					w.Write([]byte(`[]`))
				default:
					t.Errorf("unexpected data request: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected request", 500)
				}
			}))
			defer server.Close()
			t.Setenv("EDCCTL_BASE_URL", server.URL)
			t.Setenv("EDCCTL_ACCOUNT_ID", "30834")
			t.Setenv("EDCCTL_USERNAME", "")
			t.Setenv("EDCCTL_PASSWORD", "")
			t.Setenv("EDC_LOGIN", "user@example.test")
			t.Setenv("EDC_PASSWORD", "test-password")
			args := []string{"--config", filepath.Join(t.TempDir(), "config.yaml"), "--json", command}
			if command == "schedule" {
				args = append(args, "--from", "2026-09-28", "--to", "2026-10-05")
			}
			var stdout, stderr bytes.Buffer
			code := Execute(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			if !json.Valid(stdout.Bytes()) {
				t.Fatal("invalid JSON output")
			}
			if command == "schedule" && strings.TrimSpace(stdout.String()) != "[]" {
				t.Fatalf("empty schedule=%q", stdout.String())
			}
			want := []string{"GET /index.php", "POST /index.php", "POST /class_calendar-ajax.php"}
			if !reflect.DeepEqual(requests, want) {
				t.Fatalf("requests=%v, want %v", requests, want)
			}
			if strings.Contains(stdout.String()+stderr.String(), "test-password") {
				t.Fatal("credential leaked")
			}
		})
	}
}
