package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"frank-remote-repo-deploy-agent/internal/output"
	"frank-remote-repo-deploy-agent/internal/runner"
)

type Client struct {
	Runner runner.Runner
	Output OutputRunner
}

func (c Client) EnsureRepo(ctx context.Context, repo, dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	return c.Runner.Run(ctx, runner.Command{Name: "git", Args: []string{"clone", repo, dir}})
}

func (c Client) Checkout(ctx context.Context, dir, branch string) error {
	remoteBranch := "origin/" + branch
	refSpec := "+refs/heads/" + branch + ":refs/remotes/origin/" + branch
	if err := c.Runner.Run(ctx, runner.Command{Name: "git", Args: []string{"fetch", "origin", refSpec, "--prune"}, Dir: dir}); err != nil {
		return err
	}
	if err := c.Runner.Run(ctx, runner.Command{Name: "git", Args: []string{"checkout", "-B", branch, remoteBranch}, Dir: dir, SuppressStdout: true}); err != nil {
		return err
	}
	if err := c.Runner.Run(ctx, runner.Command{Name: "git", Args: []string{"reset", "--hard", remoteBranch}, Dir: dir}); err != nil {
		return err
	}
	if err := c.Runner.Run(ctx, runner.Command{Name: "git", Args: []string{"clean", "-ffd"}, Dir: dir, SuppressStdout: true}); err != nil {
		return err
	}
	c.logCheckoutEvidence(ctx, dir, branch)
	return nil
}

func (c Client) logCheckoutEvidence(ctx context.Context, dir, branch string) {
	out := c.Output
	if out == nil {
		if outputRunner, ok := c.Runner.(OutputRunner); ok {
			out = outputRunner
		}
	}
	if out == nil {
		return
	}
	head, err := out.Output(ctx, runner.Command{Name: "git", Args: []string{"rev-parse", "HEAD"}, Dir: dir})
	if err != nil {
		output.Debug("checkout evidence unavailable dir=%s branch=%s: %v", dir, branch, err)
		return
	}
	status, err := out.Output(ctx, runner.Command{Name: "git", Args: []string{"status", "--short"}, Dir: dir})
	if err != nil {
		output.Debug("checkout evidence unavailable dir=%s branch=%s head=%s: %v", dir, branch, strings.TrimSpace(head), err)
		return
	}
	cleanState := strings.TrimSpace(status)
	if cleanState == "" {
		cleanState = "clean"
	}
	output.Debug("checkout synced dir=%s branch=%s head=%s status=%s", dir, branch, strings.TrimSpace(head), cleanState)
}

type OutputRunner interface {
	Output(ctx context.Context, cmd runner.Command) (string, error)
}

type Commit struct {
	Hash           string
	CommitterName  string
	CommitterEmail string
	CommittedAt    string
	Description    string
}

func HeadCommit(ctx context.Context, out OutputRunner, dir string) (string, error) {
	if out == nil {
		return "", errors.New("output runner is required")
	}
	value, err := out.Output(ctx, runner.Command{Name: "git", Args: []string{"rev-parse", "HEAD"}, Dir: dir})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func RecentCommits(ctx context.Context, out OutputRunner, dir string, limit int) ([]Commit, error) {
	if out == nil {
		return nil, errors.New("output runner is required")
	}
	if limit <= 0 {
		return []Commit{}, nil
	}
	value, err := out.Output(ctx, runner.Command{
		Name: "git",
		Args: []string{"log", "-n", strconv.Itoa(limit), "--format=%H%x00%cn%x00%ce%x00%cI%x00%s"},
		Dir:  dir,
	})
	if err != nil {
		return nil, err
	}
	return parseCommitLog(value)
}

func parseCommitLog(value string) ([]Commit, error) {
	value = strings.TrimRight(value, "\r\n")
	if value == "" {
		return []Commit{}, nil
	}
	lines := strings.Split(value, "\n")
	commits := make([]Commit, 0, len(lines))
	for index, line := range lines {
		fields := strings.Split(strings.TrimSuffix(line, "\r"), "\x00")
		if len(fields) != 5 {
			return nil, fmt.Errorf("parse git log record %d: expected 5 fields, got %d", index+1, len(fields))
		}
		commits = append(commits, Commit{
			Hash:           fields[0],
			CommitterName:  fields[1],
			CommitterEmail: fields[2],
			CommittedAt:    fields[3],
			Description:    fields[4],
		})
	}
	return commits, nil
}
