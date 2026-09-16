package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScheduleQueriesJSONOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/class_calendar-ajax.php" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Error("schedule must request JSON")
		}
		if r.URL.Query().Get("account_id") != "30834" {
			t.Error("missing studio scope")
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		for key, want := range map[string]string{"action": "getclasses", "start": "2026-09-28", "end": "2026-10-05", "selected_view": "agendaWeek"} {
			if r.PostForm.Get(key) != want {
				t.Errorf("%s = %q, want %q", key, r.PostForm.Get(key), want)
			}
		}
		// The real endpoint labels valid JSON as text/html. Trust the body, not that header.
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`[{"id":"end","start":"2026-10-05T00:00:00"},{"id":"later","start":"2026-09-29T17:00:00"},{"id":"before","start":"2026-09-27T23:59:59"},{"id":"first","start":"2026-09-28T00:00:00"},{"id":"all-day","start":"2026-09-30"}]`))
	}))
	defer server.Close()
	from := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	events, err := New(server.URL, "30834").Schedule(context.Background(), from, from.AddDate(0, 0, 7))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].ID != "first" || events[1].ID != "later" || events[2].ID != "all-day" {
		t.Fatalf("events = %+v", events)
	}
}

func TestScheduleEmptyJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[]`)) }))
	defer server.Close()
	from := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	events, err := New(server.URL, "30834").Schedule(context.Background(), from, from.AddDate(0, 0, 7))
	if err != nil || events == nil || len(events) != 0 {
		t.Fatalf("events=%v err=%v", events, err)
	}
}

func TestScheduleRejectsInvalidResponses(t *testing.T) {
	for _, body := range []string{
		`<html><body>secret-marker</body></html>`,
		`<meta http-equiv="refresh" content="0;url=/online/index.php">`,
		`null`, `{"error":"secret-marker"}`, ``, `[null]`,
		`[{"start":"secret-marker"}]`,
	} {
		t.Run(body, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.Write([]byte(body)) }))
			defer server.Close()
			from := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
			_, err := New(server.URL, "30834").Schedule(context.Background(), from, from.AddDate(0, 0, 7))
			if err == nil {
				t.Fatal("accepted invalid schedule response")
			}
			if strings.Contains(err.Error(), "secret-marker") {
				t.Fatal("response body leaked")
			}
			if requests != 1 {
				t.Fatal("unexpected retry or HTML fallback")
			}
		})
	}
}

func TestScheduleRejectsInvalidRangeBeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.Write([]byte(`[]`)) }))
	defer server.Close()
	from := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	for _, end := range []time.Time{from, from.AddDate(0, 0, -1)} {
		if _, err := New(server.URL, "30834").Schedule(context.Background(), from, end); err == nil {
			t.Fatal("accepted invalid range")
		}
	}
	if requests != 0 {
		t.Fatal("sent invalid range to server")
	}
}
