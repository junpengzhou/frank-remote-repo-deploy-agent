package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTruncateDescriptionCountsUnicodeCharacters(t *testing.T) {
	exact := strings.Repeat("界", 100)
	if got := TruncateDescription(exact); got != exact {
		t.Fatalf("100 characters changed: %q", got)
	}
	if got := TruncateDescription(exact + "多"); got != exact+"..." {
		t.Fatalf("expected 100 characters plus ellipsis, got %q", got)
	}
}

func TestWriteCreatesVersionedJSONWithEmptyCommits(t *testing.T) {
	dir := t.TempDir()
	doc := Document{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   "2026-07-17T14:35:12+08:00",
		Environment:   "test",
		Branch:        "test",
		MainModule:    "example-app",
		Build: Build{
			StartedAt:  "2026-07-17T14:34:01+08:00",
			FinishedAt: "2026-07-17T14:35:12+08:00",
			DurationMs: 71000,
		},
		Modules: []Module{{Name: "example-app", Role: RoleMain, Commits: []Commit{}}},
	}

	if err := Write(dir, doc); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, Filename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("metadata must end with a newline: %q", data)
	}
	if !strings.Contains(string(data), `"commits": []`) {
		t.Fatalf("empty commits must be encoded as an array: %s", data)
	}
	var decoded Document
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != "1.0" || decoded.Modules[0].Commits == nil {
		t.Fatalf("unexpected metadata: %#v", decoded)
	}
}
