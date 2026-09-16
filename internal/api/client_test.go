package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

func TestLoginCookieAndEncoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "test"})
			w.Write([]byte(`<form><input name="email"><input name="password"><input name="csrf_token" value="token"></form>`))
			return
		}
		r.ParseForm()
		if r.Form.Get("password") != "a&b+c" || r.Form.Get("csrf_token") != "token" {
			t.Error("invalid encoded form")
		}
		if _, err := r.Cookie("session"); err != nil {
			t.Error("missing cookie")
		}
		w.Write([]byte(`<script>window.parent.postMessage({source: "dsp-portal", messageType:"logininfo", args: []}, '*');</script><meta http-equiv="refresh" content="0;url=app_my_class_schedule.php">`))
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

func TestParsers(t *testing.T) {
	pages := map[string]string{
		"/my_students.php":         `<div id="div_student_table"><table><tr><td><div><a onclick="$('#myModalEdit_123').show()">Test Dancer</a><br>10 years old.</div><div class="accordion"><div class="accordion-item"><button class="accordion-button" data-bs-target="#collapse_123_456">Ballet</button><div class="accordion-body">Monday<br>4 PM to 5 PM<br>Sep 28 - May 30<br>Test Teacher<br>Room A</div></div></div></td></tr></table></div>`,
		"/my_payments.php":         `<h3>You owe $25.00</h3><table><tr><td>Test Dancer</td><td>Owes $25.00</td><td><a href="app_statements.php?sid=123">View</a></td></tr></table>`,
		"/my_history.php":          `<div><h3>Payment History</h3>Balance: Owe $25.00<table class="table-bordered"><tr><td><span id="sp_full_1">Tuition</span><p class="text-info">Dancer</p><small>Sep 28,2026</small></td><td></td><td>25.00</td><td>$25.00</td></tr></table></div>`,
		"/bulletin_board.php":      `<div class="tab-pane" id="tab2"><table><tr><td><a onclick="show_com_log('123')">Hello</a><span class="muted">Sent on Sep 28,2026</span></td></tr></table></div>`,
		"/files.php":               `<div class="tab-pane" id="tab1"><ul><li><em>Dancer</em><a href="app_display_file.php?url=https%3A%2F%2Fexample.test%2Ffile.pdf">Handbook</a></li></ul></div>`,
		"/my_account.php":          `<form><input name="first_name" value="Test"><input name="last_name" value="Parent"><input name="email" value="test@example.test"><textarea name="address">Test address</textarea></form>`,
		"/class_calendar-ajax.php": `[{"id":"2","start":"2026-10-05T16:00:00"},{"id":"1","start":"2026-09-28T16:00:00","title":"Ballet"}]`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(pages[r.URL.Path])) }))
	defer server.Close()
	c := New(server.URL, "30834")
	ctx := context.Background()
	students, err := c.Students(ctx)
	if err != nil || len(students) != 1 {
		t.Fatalf("students: %v %v", students, err)
	}
	class := students[0].Classes[0]
	if class.Instructor != "Test Teacher" || class.Room != "Room A" {
		t.Fatalf("class: %+v", class)
	}
	balance, err := c.Balance(ctx)
	if err != nil || balance.Owed != "$25.00" || len(balance.Students) != 1 {
		t.Fatalf("balance: %+v %v", balance, err)
	}
	ledger, err := c.History(ctx, time.Time{})
	if err != nil || len(ledger.Entries) != 1 || ledger.Entries[0].Description != "Tuition" {
		t.Fatalf("history: %+v %v", ledger, err)
	}
	messages, err := c.Announcements(ctx)
	if err != nil || len(messages) != 1 || messages[0].ID != "123" {
		t.Fatalf("messages: %+v %v", messages, err)
	}
	files, err := c.Files(ctx)
	if err != nil || len(files) != 1 || files[0].URL != "https://example.test/file.pdf" {
		t.Fatalf("files: %+v %v", files, err)
	}
	account, err := c.Account(ctx)
	if err != nil || account.FirstName != "Test" {
		t.Fatalf("account: %+v %v", account, err)
	}
	from, _ := time.ParseInLocation("2006-01-02", "2026-09-28", time.Local)
	events, err := c.Schedule(ctx, from, from.AddDate(0, 0, 7))
	if err != nil || len(events) != 1 || events[0].ID != "1" {
		t.Fatalf("events: %+v %v", events, err)
	}
}

func TestErrorDoesNotEchoBody(t *testing.T) {
	err := checkStatus(403, "POST", "/index.php", []byte("secret-password"))
	if strings.Contains(err.Error(), "secret-password") {
		t.Fatal("response body leaked")
	}
}
