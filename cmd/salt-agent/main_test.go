package main

import "testing"

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
