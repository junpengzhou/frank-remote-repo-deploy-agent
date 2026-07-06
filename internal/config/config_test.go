package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadFileAppliesDefaultsAndResolvesBranch(t *testing.T) {
	path := writeConfig(t, `
workspace: /tmp/salt-agent
cacheFile: /tmp/salt-agent/cache.json
ssh:
  user: deploy
  host: 10.0.0.2
environments:
  test:
    branch: test
    mavenProfile: test
modules:
  example-common:
    repo: git@example.com/common.git
    remotePath: /data/common
    container: common
  example-frank:
    repo: git@example.com/frank.git
    packaging: war
    dependencies: [example-common]
    remotePath: /data/productData/sahara-frank/
    container: frank
`)

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}
	if cfg.Maven.Executable != "mvn" {
		t.Fatalf("expected default maven executable, got %q", cfg.Maven.Executable)
	}
	if cfg.SSH.Port != 22 {
		t.Fatalf("expected default ssh port 22, got %d", cfg.SSH.Port)
	}
	branch, err := cfg.BranchForEnv("test")
	if err != nil || branch != "test" {
		t.Fatalf("expected test branch, got %q err=%v", branch, err)
	}
	if cfg.Environments["test"].MavenProfile != "test" {
		t.Fatalf("expected test maven profile, got %q", cfg.Environments["test"].MavenProfile)
	}
	names, err := cfg.ModuleNames(" example-frank ")
	if err != nil || len(names) != 1 || names[0] != "example-frank" {
		t.Fatalf("unexpected module names: %#v err=%v", names, err)
	}
}

func TestLoadFileReadsSSHConnectTimeoutAndRsyncRetrySettings(t *testing.T) {
	path := writeConfig(t, `
workspace: /tmp/salt-agent
cacheFile: /tmp/salt-agent/cache.json
ssh:
  user: deploy
  host: 10.0.0.2
  connectTimeout: 10s
rsync:
  retries: 3
  retryDelay: 5s
environments:
  test:
    branch: test
modules:
  example-frank:
    repo: git@example.com/frank.git
    remotePath: /data/frank
    container: frank
`)

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}
	if cfg.SSH.ConnectTimeout != 10*time.Second {
		t.Fatalf("expected ssh connect timeout 10s, got %v", cfg.SSH.ConnectTimeout)
	}
	if cfg.Rsync.Retries != 3 {
		t.Fatalf("expected 3 retries, got %d", cfg.Rsync.Retries)
	}
	if cfg.Rsync.RetryDelay != 5*time.Second {
		t.Fatalf("expected retry delay 5s, got %v", cfg.Rsync.RetryDelay)
	}
}

func TestLoadFileRejectsUnknownDependency(t *testing.T) {
	path := writeConfig(t, `
workspace: /tmp/salt-agent
cacheFile: /tmp/salt-agent/cache.json
ssh:
  user: deploy
  host: 10.0.0.2
environments:
  test:
    branch: test
modules:
  example-frank:
    repo: git@example.com/frank.git
    dependencies: [example-common]
    remotePath: /data/productData/sahara-frank/
    container: frank
`)

	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "unknown module example-common") {
		t.Fatalf("expected unknown dependency error, got %v", err)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
