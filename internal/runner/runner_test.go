package runner

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestExecRunnerDoesNotPrintCommandByDefault(t *testing.T) {
	var out bytes.Buffer
	run := ExecRunner{Stdout: &out}
	cmd := helperCommand()

	err := run.Run(context.Background(), cmd)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if strings.Contains(out.String(), "[cmd]") {
		t.Fatalf("expected no command log, got %q", out.String())
	}
}

func TestExecRunnerPrintsCommandInDebugMode(t *testing.T) {
	var out bytes.Buffer
	run := ExecRunner{Stdout: &out, Debug: true}
	cmd := helperCommand()

	err := run.Run(context.Background(), cmd)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(out.String(), "[cmd] "+cmd.String()) {
		t.Fatalf("expected command log, got %q", out.String())
	}
}

func TestExecRunnerPrintsCommandInDryRunMode(t *testing.T) {
	var out bytes.Buffer
	run := ExecRunner{Stdout: &out, DryRun: true}

	err := run.Run(context.Background(), Command{Name: "git", Args: []string{"status"}})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(out.String(), "[cmd] git status") {
		t.Fatalf("expected dry-run command log, got %q", out.String())
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

func helperCommand() Command {
	return Command{
		Name: os.Args[0],
		Args: []string{"-test.run=TestHelperProcess", "--"},
		Env:  map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	}
}
