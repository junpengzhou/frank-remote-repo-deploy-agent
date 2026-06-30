package rsync

import (
	"fmt"
	"strings"

	"frank-remote-repo-deploy-agent/internal/config"
	"frank-remote-repo-deploy-agent/internal/runner"
)

type Options struct {
	Executable            string
	Options               []string
	ConnectTimeoutSeconds int
	SSH                   config.SSHConfig
}

func BuildCommand(sourceDir, remotePath string, opts Options) runner.Command {
	name := opts.Executable
	if name == "" {
		name = "rsync"
	}
	args := append([]string{}, opts.Options...)
	if len(args) == 0 {
		args = []string{"-az", "--delete", "--partial"}
	}
	sshArgs := []string{"ssh", "-p", fmt.Sprintf("%d", sshPort(opts.SSH)), "-o", "StrictHostKeyChecking=no", "-o", "BatchMode=yes"}
	if opts.ConnectTimeoutSeconds > 0 {
		sshArgs = append(sshArgs, "-o", fmt.Sprintf("ConnectTimeout=%d", opts.ConnectTimeoutSeconds))
	}
	if opts.SSH.KeyFile != "" {
		sshArgs = append(sshArgs, "-i", opts.SSH.KeyFile)
	}
	args = append(args, "-e", strings.Join(sshArgs, " "))
	args = append(args, ensureTrailingSlash(sourceDir), remote(opts.SSH)+":"+remotePath)
	return runner.Command{Name: name, Args: args}
}

func remote(ssh config.SSHConfig) string {
	return ssh.User + "@" + ssh.Host
}

func sshPort(ssh config.SSHConfig) int {
	if ssh.Port == 0 {
		return 22
	}
	return ssh.Port
}

func ensureTrailingSlash(path string) string {
	if strings.HasSuffix(path, "/") || strings.HasSuffix(path, "\\") {
		return path
	}
	return path + "/"
}
