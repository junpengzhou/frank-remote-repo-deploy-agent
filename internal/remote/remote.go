package remote

import (
	"fmt"
	"strings"

	"frank-remote-repo-deploy-agent/internal/config"
	"frank-remote-repo-deploy-agent/internal/runner"
)

func ScriptCommand(ssh config.SSHConfig, script string) runner.Command {
	return sshCommand(ssh, script)
}

func DockerRestartCommand(ssh config.SSHConfig, container string) runner.Command {
	return sshCommand(ssh, "docker restart "+shellQuote(container))
}

func TailCommand(ssh config.SSHConfig, logFile string, follow bool, lines int) runner.Command {
	if lines <= 0 {
		lines = 3000
	}
	flag := "-n"
	if follow {
		flag = "-fn"
	}
	return sshCommand(ssh, fmt.Sprintf("tail %s %d %s", flag, lines, shellQuote(logFile)))
}

func sshCommand(ssh config.SSHConfig, remoteCommand string) runner.Command {
	args := []string{"-p", fmt.Sprintf("%d", sshPort(ssh))}
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
