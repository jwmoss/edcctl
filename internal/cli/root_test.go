package cli

import (
	"bytes"
	"context"
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
