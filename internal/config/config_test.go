package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFileAppliesDefaultsAndResolvesBranch(t *testing.T) {
	path := writeConfig(t, `
workspace: /tmp/spug-agent
cacheFile: /tmp/spug-agent/cache.json
ssh:
  user: deploy
  host: 10.0.0.2
environments:
  test:
    branch: test
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
    logFile: /data/productData/logs/sahara-frank/catalina.out
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
	names, err := cfg.ModuleNames(" example-frank ")
	if err != nil || len(names) != 1 || names[0] != "example-frank" {
		t.Fatalf("unexpected module names: %#v err=%v", names, err)
	}
}

func TestLoadFileRejectsUnknownDependency(t *testing.T) {
	path := writeConfig(t, `
workspace: /tmp/spug-agent
cacheFile: /tmp/spug-agent/cache.json
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
