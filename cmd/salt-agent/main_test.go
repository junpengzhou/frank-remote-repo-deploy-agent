package main

import (
	"reflect"
	"testing"

	registerx "frank-remote-repo-deploy-agent/internal/register"
)

func TestParseRegisterOptionsDefaultsPortAndScriptsDir(t *testing.T) {
	opts, err := parseRegisterOptions([]string{
		"--ssh-dir", "/root/.ssh",
		"--host", "10.0.0.1",
		"--user", "root",
		"--password", "secret",
		"--remote-path", "/data/salt-agent",
	})

	if err != nil {
		t.Fatalf("parseRegisterOptions returned error: %v", err)
	}
	if opts.Port != 22 {
		t.Fatalf("expected default port 22, got %d", opts.Port)
	}
	if opts.ScriptsDir != "scripts" {
		t.Fatalf("expected default scripts dir, got %q", opts.ScriptsDir)
	}
	if opts.RegenerateKey {
		t.Fatal("expected preserve existing key by default")
	}
}

func TestParseRegisterOptionsRequiresSSHDir(t *testing.T) {
	_, err := parseRegisterOptions([]string{
		"--host", "10.0.0.1",
		"--user", "root",
		"--password", "secret",
		"--remote-path", "/data/salt-agent",
	})

	if err == nil {
		t.Fatal("expected missing ssh-dir error")
	}
}

func TestParseRegisterOptionsRequiresRemoteConnectionFields(t *testing.T) {
	_, err := parseRegisterOptions([]string{
		"--ssh-dir", "/root/.ssh",
		"--user", "root",
		"--password", "secret",
		"--remote-path", "/data/salt-agent",
	})

	if err == nil {
		t.Fatal("expected missing host error")
	}
}

func TestParseRegisterOptionsSupportsRegenerateKey(t *testing.T) {
	opts, err := parseRegisterOptions([]string{
		"--ssh-dir", "/root/.ssh",
		"--host", "10.0.0.1",
		"--user", "root",
		"--password", "secret",
		"--remote-path", "/data/salt-agent",
		"--regenerate-key",
	})

	if err != nil {
		t.Fatalf("parseRegisterOptions returned error: %v", err)
	}
	if !opts.RegenerateKey {
		t.Fatal("expected regenerate key option")
	}
}

func TestParseRegisterOptionsMarksProvidedScriptsDir(t *testing.T) {
	opts, err := parseRegisterOptions([]string{
		"--ssh-dir", "/root/.ssh",
		"--host", "10.0.0.1",
		"--user", "root",
		"--password", "secret",
		"--remote-path", "/data/salt-agent",
		"--scripts-dir", "/prosh/salt-agent",
	})

	if err != nil {
		t.Fatalf("parseRegisterOptions returned error: %v", err)
	}
	if !opts.ScriptsDirProvided {
		t.Fatal("expected scripts dir to be marked as provided")
	}
}

func TestRegisterSuccessMessageIncludesHostAndRemotePath(t *testing.T) {
	got := registerSuccessMessage("47.120.6.215", "/prosh/salt-agent")
	want := "register completed successfully, host: 47.120.6.215, remote path: /prosh/salt-agent"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestOpenSSHPasswordlessCommandDisablesStrictHostKeyChecking(t *testing.T) {
	name, args := openSSHPasswordlessCommand(registerCLIOptions{
		Options: registerOptionsForTest("root", "47.120.6.215", 22022),
	}, "/data/salt-agent/.ssh/id_rsa")
	wantArgs := []string{
		"-i", "/data/salt-agent/.ssh/id_rsa",
		"-o", "StrictHostKeyChecking=no",
		"-p", "22022",
		"root@47.120.6.215",
		"echo 'Passwordless login successful'",
	}

	if name != "ssh" {
		t.Fatalf("expected ssh command, got %q", name)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch\nwant %#v\n got %#v", wantArgs, args)
	}
}

func registerOptionsForTest(user, host string, port int) registerx.Options {
	return registerx.Options{User: user, Host: host, Port: port}
}
