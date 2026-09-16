package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const loginForm = `<form><input name="email"><input name="password"><input name="csrf_token" value="token"></form>`
const loginSuccess = `<script>window.parent.postMessage({source: "dsp-portal", messageType:"logininfo", args: []}, '*');</script><meta http-equiv="refresh" content="0;url=app_my_class_schedule.php">`

func TestLoginFailsClosed(t *testing.T) {
	for _, response := range []string{"", "<html>Maintenance</html>", `<form><input name="email"><input name="password"></form>`} {
		t.Run(response, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(response)) }))
			defer server.Close()
			if err := New(server.URL, "30834").Login(context.Background(), "user@example.test", "pw"); err == nil {
				t.Fatal("accepted unverified login")
			}
		})
	}
}

func TestLoginRejectsFailedSubmission(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(loginForm)) }))
	defer server.Close()
	if err := New(server.URL, "30834").Login(context.Background(), "user@example.test", "pw"); err == nil {
		t.Fatal("accepted failed login")
	}
}

func TestLoginCookieAndEncoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "text/html" {
			t.Error("login must accept its HTML form")
		}
		if r.Method == http.MethodGet {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "test"})
			w.Write([]byte(loginForm))
			return
		}
		r.ParseForm()
		if r.Form.Get("password") != "a&b+c" || r.Form.Get("csrf_token") != "token" {
			t.Error("invalid encoded form")
		}
		if _, err := r.Cookie("session"); err != nil {
			t.Error("missing cookie")
		}
		w.Write([]byte(loginSuccess))
	}))
	defer server.Close()
	if err := New(server.URL, "30834").Login(context.Background(), "user@example.test", "a&b+c"); err != nil {
		t.Fatal(err)
	}
}

func TestTransportRejectsExternalURL(t *testing.T) {
	c := New("https://app.gostudiopro.com/online", "30834")
	for _, path := range []string{"https://example.test/", "//example.test/", "/../apps/private.php"} {
		if _, err := c.url(path, nil); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
}

func TestScheduleHTTPFailureDoesNotEchoBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "secret-password", http.StatusForbidden) }))
	defer server.Close()
	from := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	_, err := New(server.URL, "30834").Schedule(context.Background(), from, from.AddDate(0, 0, 7))
	if err == nil {
		t.Fatal("expected HTTP failure")
	}
	if strings.Contains(err.Error(), "secret-password") {
		t.Fatal("response body leaked")
	}
}

func TestDryRunRefusesDataPOST(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer server.Close()
	from := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	_, err := New(server.URL, "30834", WithDryRun(true)).Schedule(context.Background(), from, from.AddDate(0, 0, 7))
	if err == nil || requests != 0 {
		t.Fatal("dry-run must refuse POST before sending it")
	}
}
