package register

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	SSHDir     string
	Host       string
	Port       int
	User       string
	Password   string
	RemotePath string
	ScriptsDir string
}

type Client interface {
	Run(ctx context.Context, command string) error
	MkdirAll(ctx context.Context, path string) error
	UploadFile(ctx context.Context, localPath, remotePath string, mode os.FileMode) error
	Close() error
}

type Registrar struct {
	Client Client
}

func (r Registrar) Run(ctx context.Context, opts Options) error {
	if r.Client == nil {
		return errors.New("client is required")
	}
	if err := validate(opts); err != nil {
		return err
	}
	defer func() {
		_ = r.Client.Close()
	}()

	if err := r.Client.Run(ctx, installScript(opts.RemotePath)); err != nil {
		return fmt.Errorf("run remote install script: %w", err)
	}
	remoteScripts := remoteJoin(opts.RemotePath, "scripts")
	if err := r.syncScripts(ctx, opts.ScriptsDir, remoteScripts); err != nil {
		return err
	}
	if err := r.Client.Run(ctx, "chmod -R a+rx "+shellQuote(remoteScripts)); err != nil {
		return fmt.Errorf("chmod remote scripts: %w", err)
	}
	return nil
}

func validate(opts Options) error {
	if strings.TrimSpace(opts.SSHDir) == "" {
		return errors.New("ssh dir is required")
	}
	if strings.TrimSpace(opts.Host) == "" {
		return errors.New("host is required")
	}
	if strings.TrimSpace(opts.User) == "" {
		return errors.New("user is required")
	}
	if opts.Password == "" {
		return errors.New("password is required")
	}
	if strings.TrimSpace(opts.RemotePath) == "" {
		return errors.New("remote path is required")
	}
	if strings.TrimSpace(opts.ScriptsDir) == "" {
		return errors.New("scripts dir is required")
	}
	return nil
}

func (r Registrar) syncScripts(ctx context.Context, scriptsDir, remoteScripts string) error {
	return filepath.WalkDir(scriptsDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(scriptsDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return r.Client.MkdirAll(ctx, remoteScripts)
		}
		remotePath := remoteJoin(remoteScripts, filepath.ToSlash(rel))
		if entry.IsDir() {
			return r.Client.MkdirAll(ctx, remotePath)
		}
		mode := os.FileMode(0o644)
		if strings.EqualFold(filepath.Ext(entry.Name()), ".sh") {
			mode = 0o755
		}
		if err := r.Client.UploadFile(ctx, path, remotePath, mode); err != nil {
			return fmt.Errorf("upload script %s: %w", rel, err)
		}
		return nil
	})
}

func installScript(remotePath string) string {
	quotedPath := shellQuote(remotePath)
	return strings.Join([]string{
		"set -e",
		"SALT_AGENT_HOME=" + quotedPath,
		"mkdir -p \"$SALT_AGENT_HOME\" \"$SALT_AGENT_HOME/scripts\"",
		"profile=/etc/profile.d/salt-agent.sh",
		"profile_content=\"export SALT_AGENT_HOME=$SALT_AGENT_HOME\nexport PATH=\\$PATH:\\$SALT_AGENT_HOME:\\$SALT_AGENT_HOME/scripts\"",
		"if command -v sudo >/dev/null 2>&1; then",
		"  printf '%s\n' \"$profile_content\" | sudo tee \"$profile\" >/dev/null",
		"else",
		"  printf '%s\n' \"$profile_content\" > \"$profile\"",
		"fi",
		"echo \"Salt-Agent registered at $SALT_AGENT_HOME\"",
	}, "\n")
}

func remoteJoin(base string, parts ...string) string {
	result := strings.TrimRight(base, "/")
	for _, part := range parts {
		clean := strings.Trim(part, "/")
		if clean == "" {
			continue
		}
		result += "/" + clean
	}
	if result == "" {
		return "/"
	}
	return result
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
