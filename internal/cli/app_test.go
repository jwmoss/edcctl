package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type appRoundTripper func(*http.Request) (*http.Response, error)

func (f appRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAppCommands(t *testing.T) {
	for _, mode := range []string{"--json", "--plain", "--no-color"} {
		for _, resource := range []string{"info", "groups", "notifications", "profile"} {
			t.Run(mode+resource, func(t *testing.T) {
				t.Setenv("EDCCTL_BASE_URL", "https://portal.example.test/online")
				t.Setenv("EDCCTL_ACCOUNT_ID", "30834")
				t.Setenv("EDCCTL_USERNAME", "parent@example.test")
				t.Setenv("EDCCTL_PASSWORD", "test-password")
				old := http.DefaultTransport
				t.Cleanup(func() { http.DefaultTransport = old })
				requests := 0
				http.DefaultTransport = appRoundTripper(func(r *http.Request) (*http.Response, error) {
					requests++
					body := ""
					headers := make(http.Header)
					switch r.URL.Path {
					case "/online/index.php":
						if r.Method == "GET" {
							body = `<form><input name="email"><input name="password"><input name="csrf_token" value="synthetic-token"></form>`
							headers.Set("Set-Cookie", "session=synthetic; Path=/")
						} else {
							body = `messageType:"logininfo" app_my_class_schedule.php`
						}
					case "/applib/getAppInfo.php":
						body = `{"appinfo":{"appName":"Dance"},"locations":[]}`
					case "/applib/groupsUtil.php":
						body = `[{"id":12,"appID":2555,"name":"Families","anyoneCanJoin":1,"invitationOnly":0,"askToJoin":0,"loginRequired":0,"hidden":0,"pwd":""}]`
					case "/applib/notificationsUtil.php":
						body = `{"messages":[{"PushMsgId":3,"sentDt":123,"title":"Title","Msg":"Two\nlines\tand\u001bcontrols"}]}`
					case "/applib/AppProfileManager.php":
						r.ParseForm()
						if r.Form.Get("email") != "parent@example.test" || r.Form.Get("password") != "" {
							t.Error("wrong profile request")
						}
						body = `{"success":true,"data":{"id":"7","firstName":"Parent","email":"parent@example.test","chatToken":"hidden-secret"}}`
					default:
						t.Errorf("unexpected request %s", r.URL.Path)
					}
					if strings.HasPrefix(r.URL.Path, "/applib/") {
						if r.URL.Host != "mobileinventor.com" || r.Header.Get("Cookie") != "" || r.Header.Get("X-CSRF-Token") != "" {
							t.Error("incorrect app transport or credential leakage")
						}
						if r.Header.Get("Accept") != "application/json" {
							t.Error("non-JSON data query")
						}
					}
					return &http.Response{StatusCode: 200, Header: headers, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
				})
				args := []string{"--config", filepath.Join(t.TempDir(), "config.yaml"), "--trace-http", mode, "app", resource}
				if resource == "notifications" {
					args = append(args, "--group", "12")
				}
				var out, errOut bytes.Buffer
				if code := Execute(context.Background(), args, strings.NewReader(""), &out, &errOut); code != 0 {
					t.Fatalf("exit %d: %s", code, errOut.String())
				}
				want := 3
				if resource == "notifications" {
					want = 4
				}
				if requests != want {
					t.Fatalf("requests=%d want %d", requests, want)
				}
				if mode == "--json" && !json.Valid(out.Bytes()) {
					t.Fatal("invalid JSON")
				}
				if mode == "--plain" && resource == "notifications" && strings.Count(out.String(), "\n") != 1 {
					t.Fatal("plain output is not line-oriented")
				}
				for _, secret := range []string{"test-password", "synthetic-token", "hidden-secret", "\x1b"} {
					if strings.Contains(out.String()+errOut.String(), secret) {
						t.Fatalf("output contains %q", secret)
					}
				}
			})
		}
	}
}

func TestAppValidationBeforeDataQueries(t *testing.T) {
	t.Setenv("EDCCTL_BASE_URL", "https://portal.example.test/online")
	t.Setenv("EDCCTL_USERNAME", "parent@example.test")
	t.Setenv("EDCCTL_PASSWORD", "test-password")
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	calls := 0
	http.DefaultTransport = appRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `<form><input name="email"><input name="password"><input name="csrf_token" value="token"></form>`
		if r.Method == "POST" {
			body = `messageType:"logininfo" app_my_class_schedule.php`
		}
		if r.URL.Host == "mobileinventor.com" {
			t.Fatal("reached app despite invalid arguments or account scope")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	for _, args := range [][]string{{"app", "notifications"}, {"app", "notifications", "--group", "-1"}, {"app", "profile", "other@example.test"}} {
		var out, errOut bytes.Buffer
		if code := Execute(context.Background(), args, strings.NewReader(""), &out, &errOut); code != exitUsage || calls != 0 {
			t.Fatalf("validation ran too late: %d %s", code, errOut.String())
		}
	}
	t.Setenv("EDCCTL_ACCOUNT_ID", "999")
	var out, errOut bytes.Buffer
	if code := Execute(context.Background(), []string{"app", "profile"}, strings.NewReader(""), &out, &errOut); code != exitErr {
		t.Fatal("accepted non-EDC account")
	}
}
