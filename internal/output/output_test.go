package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestQuietSuppressesTable(t *testing.T) {
	var stdout bytes.Buffer
	formatter := New(&stdout, &bytes.Buffer{}, false, false, true, true)
	formatter.Table([]string{"A"}, [][]string{{"B"}})
	if stdout.String() != "" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestTermDumbDisablesColor(t *testing.T) {
	t.Setenv("TERM", "dumb")
	var stdout bytes.Buffer
	formatter := New(&stdout, &bytes.Buffer{}, false, false, false, false)
	formatter.Success("saved")
	if strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("stdout contains color escape: %q", stdout.String())
	}
}
