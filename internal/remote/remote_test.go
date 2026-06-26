package remote

import (
	"fmt"
	"testing"

	"frank-remote-repo-deploy-agent/internal/config"
	"frank-remote-repo-deploy-agent/internal/constants"
)

func TestDockerRestartCommand(t *testing.T) {
	cmd := DockerRestartCommand(config.SSHConfig{User: "root", Host: "10.0.0.1", Port: 22}, "frank")
	want := []string{"-p", "22", "-o", "StrictHostKeyChecking=no", "root@10.0.0.1", "docker restart 'frank'"}
	if !equal(cmd.Args, want) {
		t.Fatalf("args mismatch\nwant %#v\n got %#v", want, cmd.Args)
	}
}

func TestScriptCommandAppendsOperatorUser(t *testing.T) {
	cmd := ScriptCommandWithUser(config.SSHConfig{User: "root", Host: "10.0.0.1", Port: 22}, "/prosh/frankzhou/unzip_re_docker_test.sh -q", "Frank Zhou")
	want := []string{"-p", "22", "-o", "StrictHostKeyChecking=no", "root@10.0.0.1", "/prosh/frankzhou/unzip_re_docker_test.sh -q --user 'Frank Zhou'"}
	if !equal(cmd.Args, want) {
		t.Fatalf("args mismatch\nwant %#v\n got %#v", want, cmd.Args)
	}
}

func TestTailCommandPrintsRequestedLines(t *testing.T) {
	cmd := TailCommand(config.SSHConfig{User: "root", Host: "10.0.0.1"}, "/data/logs/catalina.out", 50)
	want := []string{"-p", "22", "-o", "StrictHostKeyChecking=no", "root@10.0.0.1", "tail -n 50 '/data/logs/catalina.out'"}
	if !equal(cmd.Args, want) {
		t.Fatalf("args mismatch\nwant %#v\n got %#v", want, cmd.Args)
	}
}

func TestTailCommandUsesDefaultTailLines(t *testing.T) {
	cmd := TailCommand(config.SSHConfig{User: "root", Host: "10.0.0.1"}, "/data/logs/catalina.out", 0)
	want := []string{"-p", "22", "-o", "StrictHostKeyChecking=no", "root@10.0.0.1", fmt.Sprintf("tail -n %d '/data/logs/catalina.out'", constants.DefaultTailLines)}
	if !equal(cmd.Args, want) {
		t.Fatalf("args mismatch\nwant %#v\n got %#v", want, cmd.Args)
	}
}

func equal(a, b []string) bool {
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
