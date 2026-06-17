package register

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistrarRunsInstallUploadsScriptsAndChmods(t *testing.T) {
	scriptsDir := t.TempDir()
	writeFile(t, filepath.Join(scriptsDir, "install_salt_agent.sh"), "#!/bin/bash\necho install\n")
	writeFile(t, filepath.Join(scriptsDir, "nested", "restart.sh"), "#!/bin/bash\necho restart\n")
	writeFile(t, filepath.Join(scriptsDir, "README.txt"), "notes\n")

	client := &recordingClient{}
	registrar := Registrar{Client: client}

	err := registrar.Run(context.Background(), Options{
		SSHDir:     filepath.Join(t.TempDir(), ".ssh"),
		Host:       "10.0.0.1",
		Port:       2222,
		User:       "root",
		Password:   "secret",
		RemotePath: "/data/salt-agent",
		ScriptsDir: scriptsDir,
	})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(client.commands) != 2 {
		t.Fatalf("expected two remote commands, got %#v", client.commands)
	}
	if !strings.Contains(client.commands[0], "SALT_AGENT_HOME='/data/salt-agent'") {
		t.Fatalf("expected install script to include remote path, got %q", client.commands[0])
	}
	if !strings.Contains(client.commands[0], "/etc/profile.d/salt-agent.sh") {
		t.Fatalf("expected install script to configure profile, got %q", client.commands[0])
	}
	if client.commands[1] != "chmod -R a+rx '/data/salt-agent/scripts'" {
		t.Fatalf("unexpected chmod command %q", client.commands[1])
	}

	wantUploads := []uploadRecord{
		{local: filepath.Join(scriptsDir, "README.txt"), remote: "/data/salt-agent/scripts/README.txt", mode: 0o644},
		{local: filepath.Join(scriptsDir, "install_salt_agent.sh"), remote: "/data/salt-agent/scripts/install_salt_agent.sh", mode: 0o755},
		{local: filepath.Join(scriptsDir, "nested", "restart.sh"), remote: "/data/salt-agent/scripts/nested/restart.sh", mode: 0o755},
	}
	if !sameUploads(client.uploads, wantUploads) {
		t.Fatalf("uploads mismatch\nwant %#v\n got %#v", wantUploads, client.uploads)
	}
	if !contains(client.dirs, "/data/salt-agent/scripts") || !contains(client.dirs, "/data/salt-agent/scripts/nested") {
		t.Fatalf("expected remote script dirs to be created, got %#v", client.dirs)
	}
}

func TestRegistrarRequiresConnectionAndPathFields(t *testing.T) {
	err := (Registrar{}).Run(context.Background(), Options{
		Host:       "10.0.0.1",
		User:       "root",
		Password:   "secret",
		RemotePath: "/data/salt-agent",
	})

	if err == nil || !strings.Contains(err.Error(), "client is required") {
		t.Fatalf("expected missing client error, got %v", err)
	}

	err = (Registrar{Client: &recordingClient{}}).Run(context.Background(), Options{
		SSHDir:     filepath.Join(t.TempDir(), ".ssh"),
		User:       "root",
		Password:   "secret",
		RemotePath: "/data/salt-agent",
	})

	if err == nil || !strings.Contains(err.Error(), "host is required") {
		t.Fatalf("expected missing host error, got %v", err)
	}
}

func TestInstallScriptUsesQuotedRemotePath(t *testing.T) {
	script := installScript("/opt/salt agent")

	if !strings.Contains(script, "SALT_AGENT_HOME='/opt/salt agent'") {
		t.Fatalf("expected quoted SALT_AGENT_HOME, got %q", script)
	}
	if !strings.Contains(script, "mkdir -p \"$SALT_AGENT_HOME\" \"$SALT_AGENT_HOME/scripts\"") {
		t.Fatalf("expected script to create home and scripts directory, got %q", script)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type recordingClient struct {
	commands []string
	dirs     []string
	uploads  []uploadRecord
}

func (c *recordingClient) Run(_ context.Context, command string) error {
	c.commands = append(c.commands, command)
	return nil
}

func (c *recordingClient) MkdirAll(_ context.Context, path string) error {
	c.dirs = append(c.dirs, path)
	return nil
}

func (c *recordingClient) UploadFile(_ context.Context, localPath, remotePath string, mode os.FileMode) error {
	c.uploads = append(c.uploads, uploadRecord{local: localPath, remote: remotePath, mode: mode})
	return nil
}

func (c *recordingClient) Close() error {
	return nil
}

type uploadRecord struct {
	local  string
	remote string
	mode   os.FileMode
}

func sameUploads(got, want []uploadRecord) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
