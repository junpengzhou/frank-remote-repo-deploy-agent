package gitops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"frank-remote-repo-deploy-agent/internal/output"
	"frank-remote-repo-deploy-agent/internal/runner"
	"frank-remote-repo-deploy-agent/internal/testutil"
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

	logs := testutil.CaptureStdout(t, func() {
		if err := client.Checkout(context.Background(), "/workspace/example", "test"); err != nil {
			t.Fatalf("Checkout returned error: %v", err)
		}
	})

	if !strings.Contains(logs, "checkout synced dir=/workspace/example branch=test head=7be52ff status=clean") {
		t.Fatalf("expected checkout evidence log, got %q", logs)
	}
}

func TestRecentCommitsQueriesAndParsesNewestRecords(t *testing.T) {
	run := &recordingRunner{outputs: []string{
		"0123456789abcdef0123456789abcdef01234567\x00Frank Zhou\x00frank@example.com\x002026-07-17 13:20:30 +0800\x00发布元数据\n" +
			"89abcdef0123456789abcdef0123456789abcdef\x00Developer\x00dev@example.com\x002026-07-16 18:10:00 +0800\x00Update dependency\n",
	}}

	commits, err := RecentCommits(context.Background(), run, "/workspace/example", 3)
	if err != nil {
		t.Fatalf("RecentCommits returned error: %v", err)
	}

	want := runner.Command{
		Name: "git",
		Args: []string{
			"log",
			"-n",
			"3",
			"--format=%H%x00%cn%x00%ce%x00%ci%x00%s",
			"--no-merges",
		},
		Dir: "/workspace/example",
	}
	if !commandsEqual(run.commands, []runner.Command{want}) {
		t.Fatalf("unexpected commands: %#v", run.commands)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %#v", commits)
	}
	if commits[0].Hash != "0123456789abcdef0123456789abcdef01234567" ||
		commits[0].CommitterName != "Frank Zhou" ||
		commits[0].CommitterEmail != "frank@example.com" ||
		commits[0].CommittedAt != "2026-07-17T13:20:30+08:00" ||
		commits[0].Description != "发布元数据" {
		t.Fatalf("unexpected first commit: %#v", commits[0])
	}
	if commits[1].Hash != "89abcdef0123456789abcdef0123456789abcdef" {
		t.Fatalf("unexpected second commit: %#v", commits[1])
	}
}

func TestRecentCommitsRejectsInvalidCommitTime(t *testing.T) {
	run := &recordingRunner{outputs: []string{
		"0123456789abcdef0123456789abcdef01234567\x00Frank Zhou\x00frank@example.com\x00%cI\x00发布元数据\n",
	}}

	_, err := RecentCommits(context.Background(), run, "/workspace/example", 3)
	if err == nil || !strings.Contains(err.Error(), "commit time") {
		t.Fatalf("expected commit time parse error, got %v", err)
	}
}

func TestRecentCommitsReturnsNonNilEmptySliceForNoOutput(t *testing.T) {
	run := &recordingRunner{outputs: []string{""}}

	commits, err := RecentCommits(context.Background(), run, "/workspace/example", 3)
	if err != nil {
		t.Fatalf("RecentCommits returned error: %v", err)
	}
	if commits == nil || len(commits) != 0 {
		t.Fatalf("expected non-nil empty commits, got %#v", commits)
	}
}

func TestRecentCommitsRejectsMalformedOutput(t *testing.T) {
	run := &recordingRunner{outputs: []string{"hash-only\n"}}

	_, err := RecentCommits(context.Background(), run, "/workspace/example", 3)
	if err == nil || !strings.Contains(err.Error(), "record 1") {
		t.Fatalf("expected record parse error, got %v", err)
	}
}

func TestRecentCommitsPropagatesGitFailure(t *testing.T) {
	run := &recordingRunner{outputErr: errors.New("git log failed")}

	_, err := RecentCommits(context.Background(), run, "/workspace/example", 3)
	if err == nil || !strings.Contains(err.Error(), "git log failed") {
		t.Fatalf("expected git failure, got %v", err)
	}
}

type recordingRunner struct {
	commands  []runner.Command
	outputs   []string
	outputErr error
}

func (r *recordingRunner) Run(_ context.Context, cmd runner.Command) error {
	r.commands = append(r.commands, cmd)
	return nil
}

func (r *recordingRunner) Output(_ context.Context, cmd runner.Command) (string, error) {
	r.commands = append(r.commands, cmd)
	if r.outputErr != nil {
		return "", r.outputErr
	}
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
