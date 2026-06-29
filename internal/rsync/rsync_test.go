package rsync

import (
	"testing"

	"frank-remote-repo-deploy-agent/internal/config"
)

func TestBuildCommandUsesDeletePartialSSHAndTrailingSlash(t *testing.T) {
	cmd := BuildCommand("/tmp/staging/frank", "/data/productData/sahara-frank/", Options{
		SSH: config.SSHConfig{User: "root", Host: "10.0.0.1", Port: 2222, KeyFile: "/keys/id_rsa"},
	})
	want := []string{"-az", "--delete", "--partial", "-e", "ssh -p 2222 -o StrictHostKeyChecking=no -o BatchMode=yes -i /keys/id_rsa", "/tmp/staging/frank/", "root@10.0.0.1:/data/productData/sahara-frank/"}
	if !equal(cmd.Args, want) {
		t.Fatalf("args mismatch\nwant %#v\n got %#v", want, cmd.Args)
	}
}

func TestBuildCommandAddsRsyncAndSSHTimeouts(t *testing.T) {
	cmd := BuildCommand("/tmp/staging/frank", "/data/frank", Options{
		ConnectTimeoutSeconds: 10,
		TimeoutSeconds:        60,
		SSH:                   config.SSHConfig{User: "root", Host: "10.0.0.1", Port: 2222},
	})
	want := []string{"-az", "--delete", "--partial", "--contimeout=10", "--timeout=60", "-e", "ssh -p 2222 -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=10", "/tmp/staging/frank/", "root@10.0.0.1:/data/frank"}
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
