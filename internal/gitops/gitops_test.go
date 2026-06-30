package gitops

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"frank-remote-repo-deploy-agent/internal/output"
	"frank-remote-repo-deploy-agent/internal/runner"
)

func TestEnsureRepoSkipsFetchForExistingRepo(t *testing.T) {
	run := &recordingRunner{}
	client := Client{Runner: run}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := client.EnsureRepo(context.Background(), "git@example.com/repo.git", dir); err != nil {
		t.Fatalf("EnsureRepo returned error: %v", err)
	}

	if len(run.commands) != 0 {
		t.Fatalf("expected no commands for existing repo, got %#v", run.commands)
	}
}

func TestCheckoutForceSyncsRemoteBranch(t *testing.T) {
	run := &recordingRunner{}
	client := Client{Runner: run}

	if err := client.Checkout(context.Background(), "/workspace/example", "test"); err != nil {
		t.Fatalf("Checkout returned error: %v", err)
	}

	want := []runner.Command{
		{Name: "git", Args: []string{"fetch", "origin", "+refs/heads/test:refs/remotes/origin/test", "--prune"}, Dir: "/workspace/example"},
		{Name: "git", Args: []string{"checkout", "-B", "test", "origin/test"}, Dir: "/workspace/example", SuppressStdout: true},
		{Name: "git", Args: []string{"reset", "--hard", "origin/test"}, Dir: "/workspace/example"},
		{Name: "git", Args: []string{"clean", "-ffd"}, Dir: "/workspace/example", SuppressStdout: true},
		{Name: "git", Args: []string{"rev-parse", "HEAD"}, Dir: "/workspace/example"},
		{Name: "git", Args: []string{"status", "--short"}, Dir: "/workspace/example"},
	}
	if !commandsEqual(run.commands, want) {
		t.Fatalf("commands mismatch\nwant %#v\n got %#v", want, run.commands)
	}
}

func TestCheckoutPrintsPostCheckoutEvidenceInDebugMode(t *testing.T) {
	output.SetDebug(true)
	t.Cleanup(func() {
		output.SetDebug(false)
	})
	run := &recordingRunner{outputs: []string{"7be52ff\n", ""}}
	client := Client{Runner: run, Output: run}

	logs := captureStdout(t, func() {
		if err := client.Checkout(context.Background(), "/workspace/example", "test"); err != nil {
			t.Fatalf("Checkout returned error: %v", err)
		}
	})

	if !strings.Contains(logs, "checkout synced dir=/workspace/example branch=test head=7be52ff status=clean") {
		t.Fatalf("expected checkout evidence log, got %q", logs)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = write
	defer func() {
		os.Stdout = original
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

type recordingRunner struct {
	commands []runner.Command
	outputs  []string
}

func (r *recordingRunner) Run(_ context.Context, cmd runner.Command) error {
	r.commands = append(r.commands, cmd)
	return nil
}

func (r *recordingRunner) Output(_ context.Context, cmd runner.Command) (string, error) {
	r.commands = append(r.commands, cmd)
	if len(r.outputs) == 0 {
		return "", nil
	}
	value := r.outputs[0]
	r.outputs = r.outputs[1:]
	return value, nil
}

func commandsEqual(a, b []runner.Command) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Dir != b[i].Dir || a[i].SuppressStdout != b[i].SuppressStdout || !argsEqual(a[i].Args, b[i].Args) {
			return false
		}
	}
	return true
}

func argsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
