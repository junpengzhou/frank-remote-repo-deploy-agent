package pomxml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureModuleAppendsMissingModuleBeforeClosingModules(t *testing.T) {
	path := writePom(t, `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <description>broken legacy text /description>
    <modules>
        <module>ifintech-frank</module>
    </modules>
</project>
`)

	changed, err := EnsureModule(path, "ifintech-test2")
	if err != nil {
		t.Fatalf("EnsureModule returned error: %v", err)
	}
	if !changed {
		t.Fatal("expected missing module to be appended")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "        <module>ifintech-test2</module>\n    </modules>") {
		t.Fatalf("new module was not inserted before </modules> with matching indent:\n%s", content)
	}
}

func TestEnsureModuleDoesNotDuplicateExistingModule(t *testing.T) {
	path := writePom(t, `<project>
    <modules>
        <module>ifintech-frank</module>
    </modules>
</project>
`)

	changed, err := EnsureModule(path, "ifintech-frank")
	if err != nil {
		t.Fatalf("EnsureModule returned error: %v", err)
	}
	if changed {
		t.Fatal("expected existing module to stay unchanged")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "<module>ifintech-frank</module>") != 1 {
		t.Fatalf("module duplicated:\n%s", string(data))
	}
}

func TestEnsureModuleReturnsErrorWhenModulesBlockMissing(t *testing.T) {
	path := writePom(t, `<project></project>`)

	_, err := EnsureModule(path, "ifintech-test2")
	if err == nil || !strings.Contains(err.Error(), "missing </modules>") {
		t.Fatalf("expected missing modules error, got %v", err)
	}
}

func writePom(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pom.xml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
