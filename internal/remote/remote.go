package remote

import (
	"fmt"
	"strings"

	"frank-remote-repo-deploy-agent/internal/config"
	"frank-remote-repo-deploy-agent/internal/constants"
	"frank-remote-repo-deploy-agent/internal/runner"
)

func ScriptCommand(ssh config.SSHConfig, script string) runner.Command {
	return sshCommand(ssh, script)
}

func ScriptCommandWithUser(ssh config.SSHConfig, script, operator string) runner.Command {
	if strings.TrimSpace(operator) == "" {
		return ScriptCommand(ssh, script)
	}
	return sshCommand(ssh, script+" --user "+shellQuote(operator))
}

func DockerRestartCommand(ssh config.SSHConfig, container string) runner.Command {
	return sshCommand(ssh, "docker restart "+shellQuote(container))
}

func TailCommand(ssh config.SSHConfig, logFile string, lines int) runner.Command {
	if lines <= 0 {
		lines = constants.DefaultTailLines
	}
	return sshCommand(ssh, fmt.Sprintf("tail -n %d %s", lines, shellQuote(logFile)))
}

func sshCommand(ssh config.SSHConfig, remoteCommand string) runner.Command {
	args := []string{"-p", fmt.Sprintf("%d", sshPort(ssh)), "-o", "StrictHostKeyChecking=no", "-o", "BatchMode=yes"}
	if ssh.KeyFile != "" {
		args = append(args, "-i", ssh.KeyFile)
	}
	args = append(args, ssh.User+"@"+ssh.Host, remoteCommand)
	return runner.Command{Name: "ssh", Args: args}
}

func sshPort(ssh config.SSHConfig) int {
	if ssh.Port == 0 {
		return 22
	}
	return ssh.Port
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
