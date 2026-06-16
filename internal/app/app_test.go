package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHelpIncludesCompile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main(context.Background(), []string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "compile") {
		t.Fatalf("help did not include compile command: %s", stdout.String())
	}
}

func TestCompileHelpIncludesRequiredFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main(context.Background(), []string{"compile", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	help := stderr.String()
	for _, want := range []string{"-output", "-work-dir", "-skip-download", "-arin-xml", "-lacnic-db", "-ripe", "-apnic", "-afrinic"} {
		if !strings.Contains(help, want) {
			t.Fatalf("compile help missing %s in:\n%s", want, help)
		}
	}
}

func TestCompileAcceptsDoubleDashFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	dir := t.TempDir()
	code := Main(context.Background(), []string{"compile", "--skip-download", "--output", dir + "/out", "--work-dir", dir + "/work"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d stderr=%s", code, stderr.String())
	}
}
