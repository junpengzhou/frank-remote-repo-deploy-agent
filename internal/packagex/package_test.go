package packagex

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindArtifactReturnsNewestMatchingPackaging(t *testing.T) {
	moduleDir := t.TempDir()
	target := filepath.Join(moduleDir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(target, "old.war")
	newPath := filepath.Join(target, "new.war")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	artifact, err := FindArtifact(moduleDir, "war")
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Path != newPath {
		t.Fatalf("expected newest artifact %s, got %s", newPath, artifact.Path)
	}
}

func TestPrepareStagingExtractsWar(t *testing.T) {
	dir := t.TempDir()
	warPath := filepath.Join(dir, "app.war")
	makeZip(t, warPath, map[string]string{"WEB-INF/classes/App.class": "bytecode"})
	staging, err := PrepareStaging(Artifact{Path: warPath, Packaging: "war"}, filepath.Join(dir, "staging"), "example-frank")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(staging, "WEB-INF", "classes", "App.class"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "bytecode" {
		t.Fatalf("unexpected extracted content: %s", data)
	}
}

func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(out)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}
