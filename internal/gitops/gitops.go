package gitops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"frank-remote-repo-deploy-agent/internal/runner"
)

type Client struct {
	Runner runner.Runner
}

func (c Client) EnsureRepo(ctx context.Context, repo, dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return c.Runner.Run(ctx, runner.Command{Name: "git", Args: []string{"fetch", "--all", "--prune"}, Dir: dir})
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	return c.Runner.Run(ctx, runner.Command{Name: "git", Args: []string{"clone", repo, dir}})
}

func (c Client) Checkout(ctx context.Context, dir, branch string) error {
	if err := c.Runner.Run(ctx, runner.Command{Name: "git", Args: []string{"checkout", branch}, Dir: dir}); err != nil {
		return err
	}
	return c.Runner.Run(ctx, runner.Command{Name: "git", Args: []string{"pull", "--ff-only"}, Dir: dir})
}

type OutputRunner interface {
	Output(ctx context.Context, cmd runner.Command) (string, error)
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
