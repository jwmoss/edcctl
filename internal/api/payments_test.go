package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Synthetic pages copy the verified portal markup: a commented-out cell,
// truncated memos with hidden full text, credit in parentheses, and the
// opening-balance row last.
const historyPage = `<h3>Payment History</h3>Balance: Owe $1,234.50<br>
<form method="post"><select name="show_month"><option value="01">January</option><option selected value="12">December</option></select>
<select name="show_day"><option selected value="01">01</option></select>
<select name="show_year"><option value="2026">2026</option><option selected value="2025">2025</option></select></form>
<table class="table table-bordered"><tr><th>--</th><th>Pmts</th><th>Chrgs</th><th>Bal</th></tr>
<tr><td><span id='sp_9'>&quot;Spring Show<a onclick="toggleMemo(9)"><strong>...</strong></a></span><span id='sp_full_9' style='display:none'>&quot;Spring Show&quot; Costume &amp; Deposit</span><br><p class="text-info"><i>Student One</i></p><p><small>Nov 15,2026</small></p></td><td></td><td>1,250.00</td><td>$1,234.50</td></tr>
<tr><td>October Tuition -- <br><p class="text-info"><i>Student One</i></p><p><small>Oct 01,2026</small></p></td><td>20.50</td><td></td><td>($15.50)</td></tr>
<tr><!--<td></td>--><td></td><td></td><td></td><td>$5.00</td></tr>
</table>`

const paymentsPage = `<h3>You owe $1,234.50</h3>
<table class="table table-striped table-bordered"><tr><th>Name</th><th>Balance</th><th>Statement</th><!--<th>Pay Amount</th>--></tr>
<tr><td>Student One</td><td style='text-align:right'>Owes $1,250.00</td><td><a href="app_statements.php?sid=101">View</a></td></tr>
<tr><td>Student Two</td><td>($15.50)</td><td><a href="app_statements.php?sid=102">View</a></td></tr>
</table>`

func pageServer(t *testing.T, page string, check func(*http.Request)) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			check(r)
		}
		w.Write([]byte(page))
	}))
	t.Cleanup(server.Close)
	client := New(server.URL, "30834")
	client.csrfToken = "token"
	return client
}

func TestHistoryParsesVerifiedLedger(t *testing.T) {
	client := pageServer(t, historyPage, func(r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/my_history.php" || r.Header.Get("Accept") != "text/html" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	history, err := client.History(context.Background(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	want := &PaymentHistory{From: "2025-12-01", BalanceCents: 123450, OpeningBalanceCents: 500, Entries: []LedgerEntry{
		{Date: "2026-11-15", Student: "Student One", Description: `"Spring Show" Costume & Deposit`, ChargeCents: 125000, BalanceCents: 123450},
		{Date: "2026-10-01", Student: "Student One", Description: "October Tuition", PaymentCents: 2050, BalanceCents: -1550},
	}}
	if !reflect.DeepEqual(history, want) {
		t.Fatalf("history=%+v", history)
	}
}

func TestHistoryFromSubmitsDateSearch(t *testing.T) {
	client := pageServer(t, historyPage, func(r *http.Request) {
		r.ParseForm()
		want := map[string]string{"csrf_token": "token", "show_month": "12", "show_day": "01", "show_year": "2025", "btn_search": "Search"}
		for key, value := range want {
			if r.PostForm.Get(key) != value {
				t.Errorf("%s=%q, want %q", key, r.PostForm.Get(key), value)
			}
		}
		if r.Method != http.MethodPost || r.Header.Get("X-CSRF-Token") != "token" {
			t.Error("date search must POST with the CSRF token")
		}
	})
	if _, err := client.History(context.Background(), time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, err := pageServer(t, historyPage, nil).History(context.Background(), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err == nil || !strings.Contains(err.Error(), "not the requested") {
		t.Fatalf("accepted a page for another start date: %v", err)
	}
	client.dryRun = true
	if _, err := client.History(context.Background(), time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("dry-run sent the date search")
	}
}

func TestHistoryFailsClosed(t *testing.T) {
	for name, page := range map[string]string{
		"login form":         `<form><input name="email"><input name="password"></form>`,
		"unreconciled":       strings.Replace(historyPage, "<td>20.50</td>", "<td>20.00</td>", 1),
		"account mismatch":   strings.Replace(historyPage, "Balance: Owe $1,234.50", "Balance: Owe $1.00", 1),
		"no opening row":     strings.Replace(historyPage, "<td></td><td></td><td></td><td>$5.00</td>", "", 1),
		"changed header":     strings.Replace(historyPage, "<th>Pmts</th>", "<th>Payments</th>", 1),
		"unknown amount":     strings.Replace(historyPage, "<td>20.50</td>", "<td>20.5 USD</td>", 1),
		"invalid date":       strings.Replace(historyPage, "Oct 01,2026", "2026-10-01", 1),
		"missing student":    strings.Replace(historyPage, `<p class="text-info"><i>Student One</i></p><p><small>Oct`, `<p><small>Oct`, 1),
		"extra cell":         strings.Replace(historyPage, "<td>20.50</td>", "<td>20.50</td><td></td>", 1),
		"missing start date": strings.Replace(historyPage, `selected value="12"`, `value="12"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := pageServer(t, page, nil).History(context.Background(), time.Time{}); err == nil {
				t.Fatal("accepted unexpected markup")
			}
		})
	}
}

func TestBalanceParsesStudents(t *testing.T) {
	client := pageServer(t, paymentsPage, func(r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/my_payments.php" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	balance, err := client.Balance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := &AccountBalance{BalanceCents: 123450, Students: []StudentBalance{
		{StudentID: "101", Name: "Student One", BalanceCents: 125000},
		{StudentID: "102", Name: "Student Two", BalanceCents: -1550},
	}}
	if !reflect.DeepEqual(balance, want) {
		t.Fatalf("balance=%+v", balance)
	}
}

func TestBalanceFailsClosed(t *testing.T) {
	for name, page := range map[string]string{
		"heading mismatch": strings.Replace(paymentsPage, "You owe $1,234.50", "You owe $9.00", 1),
		"missing link":     strings.Replace(paymentsPage, `<a href="app_statements.php?sid=102">View</a>`, "", 1),
		"no table":         `<h3>You owe $1.00</h3>`,
		"unknown amount":   strings.Replace(paymentsPage, "Owes $1,250.00", "Due $1,250.00", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := pageServer(t, page, nil).Balance(context.Background()); err == nil {
				t.Fatal("accepted unexpected markup")
			}
		})
	}
}

func TestParseMoney(t *testing.T) {
	for value, want := range map[string]int64{"0.00": 0, "12.34": 1234, "$1,234.56": 123456, "Owe $1.00": 100, "Owes $1.00": 100, "($0.50)": -50} {
		if got, err := parseMoney(value); err != nil || got != want {
			t.Errorf("parseMoney(%q)=%d, %v; want %d", value, got, err, want)
		}
	}
	for _, value := range []string{"", "12", "12.3", "$1,23.00", "($1.00", "Owe ($1.00)", "-$1.00", "Credit $1.00"} {
		if _, err := parseMoney(value); err == nil {
			t.Errorf("parseMoney(%q) accepted an unverified format", value)
		}
	}
}
