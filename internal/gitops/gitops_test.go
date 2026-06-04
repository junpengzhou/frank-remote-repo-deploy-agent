package gitops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
	}
	if !commandsEqual(run.commands, want) {
		t.Fatalf("commands mismatch\nwant %#v\n got %#v", want, run.commands)
	}
}

type recordingRunner struct {
	commands []runner.Command
}

func (r *recordingRunner) Run(_ context.Context, cmd runner.Command) error {
	r.commands = append(r.commands, cmd)
	return nil
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
