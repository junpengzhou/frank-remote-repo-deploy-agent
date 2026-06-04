package remote

import (
	"testing"

	"frank-remote-repo-deploy-agent/internal/config"
)

func TestDockerRestartCommand(t *testing.T) {
	cmd := DockerRestartCommand(config.SSHConfig{User: "root", Host: "10.0.0.1", Port: 22}, "frank")
	want := []string{"-p", "22", "root@10.0.0.1", "docker restart 'frank'"}
	if !equal(cmd.Args, want) {
		t.Fatalf("args mismatch\nwant %#v\n got %#v", want, cmd.Args)
	}
}

func TestScriptCommandAppendsOperatorUser(t *testing.T) {
	cmd := ScriptCommandWithUser(config.SSHConfig{User: "root", Host: "10.0.0.1", Port: 22}, "/prosh/frankzhou/unzip_re_docker_test.sh -q", "Frank Zhou")
	want := []string{"-p", "22", "root@10.0.0.1", "/prosh/frankzhou/unzip_re_docker_test.sh -q --user 'Frank Zhou'"}
	if !equal(cmd.Args, want) {
		t.Fatalf("args mismatch\nwant %#v\n got %#v", want, cmd.Args)
	}
}

func TestTailCommandPrintsLast3000Lines(t *testing.T) {
	cmd := TailCommand(config.SSHConfig{User: "root", Host: "10.0.0.1"}, "/data/logs/catalina.out", 3000)
	want := []string{"-p", "22", "root@10.0.0.1", "tail -n 3000 '/data/logs/catalina.out'"}
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
