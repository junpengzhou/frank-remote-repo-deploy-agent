package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestWarningPrintsYellowWarningToStdout(t *testing.T) {
	out := captureStdout(t, func() {
		Warning("check %s", "config")
	})

	if !strings.Contains(out, "\x1b[33m[WARNING]check config\x1b[0m\n") {
		t.Fatalf("expected yellow warning, got %q", out)
	}
}

func TestErrorPrintsRedErrorToStderr(t *testing.T) {
	errOut := captureStderr(t, func() {
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

	out := captureStdout(t, func() {
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

	out := captureStdout(t, func() {
		Debug("cache %s", "hit")
	})

	if out != "" {
		t.Fatalf("expected no debug output by default, got %q", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return captureOutput(t, &os.Stdout, fn)
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return captureOutput(t, &os.Stderr, fn)
}

func captureOutput(t *testing.T, target **os.File, fn func()) string {
	t.Helper()
	original := *target
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	*target = write
	defer func() {
		*target = original
	}()

	fn()

	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, read); err != nil {
		t.Fatal(err)
	}
	if err := read.Close(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
