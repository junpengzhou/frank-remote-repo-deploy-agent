package runner

import (
	"bytes"
	"context"
	"fmt"
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

func TestExecRunnerPrintsWorkingDirectoryInDebugMode(t *testing.T) {
	var out bytes.Buffer
	run := ExecRunner{Stdout: &out, Debug: true}
	cmd := helperCommand()
	cmd.Dir = t.TempDir()

	err := run.Run(context.Background(), cmd)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(out.String(), "[cmd] (dir="+cmd.Dir+") "+cmd.String()) {
		t.Fatalf("expected command log with working directory, got %q", out.String())
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

func TestExecRunnerSuppressesCommandStdoutButKeepsDebugCommandLog(t *testing.T) {
	var out bytes.Buffer
	run := ExecRunner{Stdout: &out, Debug: true}
	cmd := helperCommand()
	cmd.SuppressStdout = true

	err := run.Run(context.Background(), cmd)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(out.String(), "[cmd] "+cmd.String()) {
		t.Fatalf("expected command log, got %q", out.String())
	}
	if strings.Contains(out.String(), "helper stdout") {
		t.Fatalf("expected helper stdout to be suppressed, got %q", out.String())
	}
}

func TestExecRunnerRunErrorEncryptsCommandDetails(t *testing.T) {
	run := ExecRunner{}
	cmd := Command{Name: "definitely-not-a-real-command", Args: []string{"--flag"}}

	err := run.Run(context.Background(), cmd)

	if err == nil {
		t.Fatal("expected Run to return error")
	}
	msg := err.Error()
	if strings.Contains(msg, cmd.String()) {
		t.Fatalf("expected error to hide plaintext command, got %q", msg)
	}
	if !strings.Contains(msg, "command failed: [encrypted-command:") {
		t.Fatalf("expected encrypted command marker, got %q", msg)
	}
}

func TestExecRunnerOutputErrorEncryptsCommandDetails(t *testing.T) {
	run := ExecRunner{}
	cmd := Command{Name: "definitely-not-a-real-command", Args: []string{"--flag"}}

	_, err := run.Output(context.Background(), cmd)

	if err == nil {
		t.Fatal("expected Output to return error")
	}
	msg := err.Error()
	if strings.Contains(msg, cmd.String()) {
		t.Fatalf("expected error to hide plaintext command, got %q", msg)
	}
	if !strings.Contains(msg, "command failed: [encrypted-command:") {
		t.Fatalf("expected encrypted command marker, got %q", msg)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	_, _ = fmt.Fprint(os.Stdout, "helper stdout")
	os.Exit(0)
}

func helperCommand() Command {
	return Command{
		Name: os.Args[0],
		Args: []string{"-test.run=TestHelperProcess", "--"},
		Env:  map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	}
}
