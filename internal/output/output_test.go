package output

import (
	"strings"
	"testing"

	"frank-remote-repo-deploy-agent/internal/testutil"
)

func TestWarningPrintsYellowWarningToStdout(t *testing.T) {
	out := testutil.CaptureStdout(t, func() {
		Warning("check %s", "config")
	})

	if !strings.Contains(out, "\x1b[33m[WARNING]check config\x1b[0m\n") {
		t.Fatalf("expected yellow warning, got %q", out)
	}
}

func TestErrorPrintsRedErrorToStderr(t *testing.T) {
	errOut := testutil.CaptureStderr(t, func() {
		Error("failed: %s", "boom")
	})

	if !strings.Contains(errOut, "\x1b[31m[ERROR]failed: boom\x1b[0m\n") {
		t.Fatalf("expected red error, got %q", errOut)
	}
}

func TestCommonLogLevelsPrintToStdout(t *testing.T) {
	SetDebug(true)
	t.Cleanup(func() {
		SetDebug(false)
	})

	out := testutil.CaptureStdout(t, func() {
		Info("starting %s", "deploy")
		Debug("cache %s", "hit")
		Success("started")
	})

	for _, want := range []string{
		"\x1b[34m[INFO]starting deploy\x1b[0m\n",
		"\x1b[36m[DEBUG]cache hit\x1b[0m\n",
		"\x1b[32m[SUCCESS]started\x1b[0m\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
}

func TestDebugDoesNotPrintByDefault(t *testing.T) {
	SetDebug(false)

	out := testutil.CaptureStdout(t, func() {
		Debug("cache %s", "hit")
	})

	if out != "" {
		t.Fatalf("expected no debug output by default, got %q", out)
	}
}
