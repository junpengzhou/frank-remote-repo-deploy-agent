package gitops

import (
	"context"
	"testing"

	"frank-remote-repo-deploy-agent/internal/runner"
)

func TestCheckoutForceSyncsRemoteBranch(t *testing.T) {
	run := &recordingRunner{}
	client := Client{Runner: run}

	if err := client.Checkout(context.Background(), "/workspace/example", "test"); err != nil {
		t.Fatalf("Checkout returned error: %v", err)
	}

	want := []runner.Command{
		{Name: "git", Args: []string{"fetch", "origin", "test", "--prune"}, Dir: "/workspace/example"},
		{Name: "git", Args: []string{"checkout", "-B", "test", "origin/test"}, Dir: "/workspace/example"},
		{Name: "git", Args: []string{"reset", "--hard", "origin/test"}, Dir: "/workspace/example"},
		{Name: "git", Args: []string{"clean", "-ffd"}, Dir: "/workspace/example"},
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
		if a[i].Name != b[i].Name || a[i].Dir != b[i].Dir || !argsEqual(a[i].Args, b[i].Args) {
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
