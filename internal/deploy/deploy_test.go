package deploy

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"frank-remote-repo-deploy-agent/internal/config"
	"frank-remote-repo-deploy-agent/internal/lock"
)

func TestEnsurePomModuleOnlyPrintsPomLogInDebugMode(t *testing.T) {
	defaultOutput := ensurePomModuleOutput(t, false, "example-default")
	if strings.Contains(defaultOutput, "[pom]") {
		t.Fatalf("expected no pom log without debug, got %q", defaultOutput)
	}

	debugOutput := ensurePomModuleOutput(t, true, "example-debug")
	if !strings.Contains(debugOutput, "[pom] appended missing module example-debug") {
		t.Fatalf("expected pom log in debug mode, got %q", debugOutput)
	}
}

func ensurePomModuleOutput(t *testing.T, debug bool, module string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(`<project><modules></modules></project>`), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &Deployer{Config: &config.Config{
		BuildRoot: root,
		LockDir:   filepath.Join(root, "locks"),
	}}
	d.Locks = lock.NewManager(d.Config.LockDir)

	return captureStdout(t, func() {
		if err := d.ensurePomModule(context.Background(), module, debug); err != nil {
			t.Fatalf("ensurePomModule returned error: %v", err)
		}
	})
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = write
	defer func() {
		os.Stdout = original
	}()

	fn()

	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, read); err != nil {
		t.Fatal(err)
	}
	if err := read.Close(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
