# Salt Agent Build Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish a versioned `salt-agent-metadata.json` with build timing and up to three recent commits for the main module and each dependency as part of every deploy staging/rsync flow.

**Architecture:** Extend `internal/gitops` with a machine-readable recent-commit query, add an isolated `internal/metadata` package for the JSON contract and atomic staging write, and let `internal/deploy` own timing, cache-skip state, module ordering, and best-effort integration. Existing Maven, staging, rsync, and restart semantics remain unchanged when metadata collection or writing fails.

**Tech Stack:** Go 1.25 standard library, local Git CLI through the existing `OutputRunner`, JSON, Maven-aware deploy orchestration, rsync, Markdown, JSON Schema 2020-12.

---

## File Map

- Modify `internal/gitops/gitops.go` and `internal/gitops/gitops_test.go`: query and parse recent commits.
- Create `internal/metadata/metadata.go` and `internal/metadata/metadata_test.go`: own the JSON protocol, Unicode truncation, and atomic file write.
- Modify `internal/deploy/deploy.go` and `internal/deploy/deploy_test.go`: track the Maven phase and publish metadata best-effort.
- Create `internal/deploy/metadata.go`: map deployment and Git data into the protocol model.
- Create `docs/schemas/salt-agent-metadata.schema.json` and `docs/salt-agent-metadata.md`: publish the downstream contract.
- Modify `README.md` and `README.zh-CN.md`: link the protocol.

### Task 1: Read Recent Commits Through `gitops`

**Files:**
- Modify: `internal/gitops/gitops.go`
- Modify: `internal/gitops/gitops_test.go`

- [ ] **Step 1: Write the failing recent-commit tests**

Add `errors` to the test imports and add:

```go
func TestRecentCommitsQueriesAndParsesNewestRecords(t *testing.T) {
	run := &recordingRunner{outputs: []string{
		"0123456789abcdef0123456789abcdef01234567\x00Frank Zhou\x00frank@example.com\x002026-07-17T13:20:30+08:00\x00发布元数据\n" +
			"89abcdef0123456789abcdef0123456789abcdef\x00Developer\x00dev@example.com\x002026-07-16T18:10:00+08:00\x00Update dependency\n",
	}}
	commits, err := RecentCommits(context.Background(), run, "/workspace/example", 3)
	if err != nil {
		t.Fatalf("RecentCommits returned error: %v", err)
	}
	want := runner.Command{
		Name: "git",
		Args: []string{"log", "-n", "3", "--format=%H%x00%cn%x00%ce%x00%cI%x00%s"},
		Dir:  "/workspace/example",
	}
	if !commandsEqual(run.commands, []runner.Command{want}) {
		t.Fatalf("unexpected commands: %#v", run.commands)
	}
	if len(commits) != 2 || commits[0].CommitterName != "Frank Zhou" ||
		commits[0].CommitterEmail != "frank@example.com" ||
		commits[0].CommittedAt != "2026-07-17T13:20:30+08:00" ||
		commits[0].Description != "发布元数据" {
		t.Fatalf("unexpected commits: %#v", commits)
	}
}

func TestRecentCommitsReturnsNonNilEmptySliceForNoOutput(t *testing.T) {
	run := &recordingRunner{outputs: []string{""}}
	commits, err := RecentCommits(context.Background(), run, "/workspace/example", 3)
	if err != nil {
		t.Fatalf("RecentCommits returned error: %v", err)
	}
	if commits == nil || len(commits) != 0 {
		t.Fatalf("expected non-nil empty commits, got %#v", commits)
	}
}

func TestRecentCommitsRejectsMalformedOutput(t *testing.T) {
	run := &recordingRunner{outputs: []string{"hash-only\n"}}
	_, err := RecentCommits(context.Background(), run, "/workspace/example", 3)
	if err == nil || !strings.Contains(err.Error(), "record 1") {
		t.Fatalf("expected record parse error, got %v", err)
	}
}

func TestRecentCommitsPropagatesGitFailure(t *testing.T) {
	run := &recordingRunner{outputErr: errors.New("git log failed")}
	_, err := RecentCommits(context.Background(), run, "/workspace/example", 3)
	if err == nil || !strings.Contains(err.Error(), "git log failed") {
		t.Fatalf("expected git failure, got %v", err)
	}
}
```

Extend the existing test runner:

```go
type recordingRunner struct {
	commands  []runner.Command
	outputs   []string
	outputErr error
}

func (r *recordingRunner) Output(_ context.Context, cmd runner.Command) (string, error) {
	r.commands = append(r.commands, cmd)
	if r.outputErr != nil {
		return "", r.outputErr
	}
	if len(r.outputs) == 0 {
		return "", nil
	}
	value := r.outputs[0]
	r.outputs = r.outputs[1:]
	return value, nil
}
```

- [ ] **Step 2: Run RED**

Run:

```powershell
go test ./internal/gitops -run 'TestRecentCommits' -v
```

Expected: compilation fails because `RecentCommits` and `Commit` are undefined.

- [ ] **Step 3: Add the minimal API and parser**

Add `fmt` and `strconv` imports, then add to `internal/gitops/gitops.go`:

```go
type Commit struct {
	Hash           string
	CommitterName  string
	CommitterEmail string
	CommittedAt    string
	Description    string
}

func RecentCommits(ctx context.Context, out OutputRunner, dir string, limit int) ([]Commit, error) {
	if out == nil {
		return nil, errors.New("output runner is required")
	}
	if limit <= 0 {
		return []Commit{}, nil
	}
	value, err := out.Output(ctx, runner.Command{
		Name: "git",
		Args: []string{"log", "-n", strconv.Itoa(limit), "--format=%H%x00%cn%x00%ce%x00%cI%x00%s"},
		Dir:  dir,
	})
	if err != nil {
		return nil, err
	}
	return parseCommitLog(value)
}

func parseCommitLog(value string) ([]Commit, error) {
	value = strings.TrimRight(value, "\r\n")
	if value == "" {
		return []Commit{}, nil
	}
	lines := strings.Split(value, "\n")
	commits := make([]Commit, 0, len(lines))
	for index, line := range lines {
		fields := strings.Split(strings.TrimSuffix(line, "\r"), "\x00")
		if len(fields) != 5 {
			return nil, fmt.Errorf("parse git log record %d: expected 5 fields, got %d", index+1, len(fields))
		}
		commits = append(commits, Commit{
			Hash: fields[0], CommitterName: fields[1], CommitterEmail: fields[2],
			CommittedAt: fields[3], Description: fields[4],
		})
	}
	return commits, nil
}
```

- [ ] **Step 4: Run GREEN and commit**

```powershell
gofmt -w internal/gitops/gitops.go internal/gitops/gitops_test.go
go test ./internal/gitops -v
git add internal/gitops/gitops.go internal/gitops/gitops_test.go
git commit -m "feat(gitops): read recent commit metadata"
```

Expected: all `internal/gitops` tests pass before the commit.

### Task 2: Create the Versioned Metadata Package

**Files:**
- Create: `internal/metadata/metadata.go`
- Create: `internal/metadata/metadata_test.go`

- [ ] **Step 1: Write failing contract and truncation tests**

Create `internal/metadata/metadata_test.go`:

```go
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
		GeneratedAt: "2026-07-17T14:35:12+08:00", Environment: "test",
		Branch: "test", MainModule: "example-app",
		Build: Build{StartedAt: "2026-07-17T14:34:01+08:00", FinishedAt: "2026-07-17T14:35:12+08:00", DurationMs: 71000},
		Modules: []Module{{Name: "example-app", Role: RoleMain, Commits: []Commit{}}},
	}
	if err := Write(dir, doc); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, Filename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "\n") || !strings.Contains(string(data), `"commits": []`) {
		t.Fatalf("unexpected JSON: %s", data)
	}
	var decoded Document
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != "1.0" || decoded.Modules[0].Commits == nil {
		t.Fatalf("unexpected metadata: %#v", decoded)
	}
}
```

- [ ] **Step 2: Run RED**

```powershell
go test ./internal/metadata -v
```

Expected: compilation fails because the package API is absent.

- [ ] **Step 3: Implement the protocol model and writer**

Create `internal/metadata/metadata.go`:

```go
package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	Filename = "salt-agent-metadata.json"
	SchemaVersion = "1.0"
	RoleMain = "main"
	RoleDependency = "dependency"
	descriptionLimit = 100
)

type Document struct {
	SchemaVersion string `json:"schemaVersion"`
	GeneratedAt string `json:"generatedAt"`
	Environment string `json:"environment"`
	Branch string `json:"branch"`
	MainModule string `json:"mainModule"`
	Build Build `json:"build"`
	Modules []Module `json:"modules"`
}
type Build struct {
	StartedAt string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	DurationMs int64 `json:"durationMs"`
	BuildSkipped bool `json:"buildSkipped"`
}
type Module struct { Name string `json:"name"`; Role string `json:"role"`; Commits []Commit `json:"commits"` }
type Commit struct { Hash string `json:"hash"`; Committer Committer `json:"committer"`; CommittedAt string `json:"committedAt"`; Description string `json:"description"` }
type Committer struct { Name string `json:"name"`; Email string `json:"email"` }

func TruncateDescription(value string) string {
	runes := []rune(value)
	if len(runes) <= descriptionLimit { return value }
	return string(runes[:descriptionLimit]) + "..."
}

func Write(stagingDir string, doc Document) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil { return err }
	data = append(data, '\n')
	temp, err := os.CreateTemp(stagingDir, ".salt-agent-metadata-*.tmp")
	if err != nil { return err }
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()
	if err := temp.Chmod(0o644); err != nil { _ = temp.Close(); return err }
	if _, err := temp.Write(data); err != nil { _ = temp.Close(); return err }
	if err := temp.Close(); err != nil { return err }
	return os.Rename(tempName, filepath.Join(stagingDir, Filename))
}
```

- [ ] **Step 4: Run GREEN and commit**

```powershell
gofmt -w internal/metadata/metadata.go internal/metadata/metadata_test.go
go test ./internal/metadata -v
git add internal/metadata/metadata.go internal/metadata/metadata_test.go
git commit -m "feat(metadata): write versioned build artifact"
```

Expected: both metadata tests pass before the commit.

### Task 3: Assemble Metadata in Stable Module Order

**Files:**
- Create: `internal/deploy/metadata.go`
- Modify: `internal/deploy/deploy.go`
- Modify: `internal/deploy/deploy_test.go`

- [ ] **Step 1: Write a failing document-assembly test**

Extend the deploy test runner with `logs map[string]string` and
`logErrors map[string]error`. Its `Output` method must route `git log` separately
from the existing `git rev-parse` behavior:

```go
func (r *recordingRunner) Output(_ context.Context, cmd runner.Command) (string, error) {
	if len(cmd.Args) > 0 && cmd.Args[0] == "log" {
		if err := r.logErrors[cmd.Dir]; err != nil { return "", err }
		return r.logs[cmd.Dir], nil
	}
	if commit, ok := r.commits[cmd.Dir]; ok { return commit + "\n", nil }
	return "abc123\n", nil
}
```

Add a test that calls the wished-for assembler:

```go
func TestBuildMetadataOrdersMainBeforeDependenciesAndUsesEmptyCommitsOnFailure(t *testing.T) {
	root := t.TempDir()
	mainDir, depDir := filepath.Join(root, "example-app"), filepath.Join(root, "example-common")
	run := &recordingRunner{
		logs: map[string]string{mainDir: "0123456789abcdef0123456789abcdef01234567\x00Frank Zhou\x00frank@example.com\x002026-07-17T13:20:30+08:00\x00" + strings.Repeat("界", 101) + "\n"},
		logErrors: map[string]error{depDir: errors.New("history unavailable")},
	}
	d := New(deployTestConfig(root), run, run, nil)
	d.now = func() time.Time { return time.Date(2026, 7, 17, 14, 35, 12, 0, time.FixedZone("CST", 8*60*60)) }
	doc := d.buildMetadata(context.Background(), Options{Env: "demo"}, "test", "example-app", buildTiming{
		startedAt: time.Date(2026, 7, 17, 14, 34, 1, 0, time.FixedZone("CST", 8*60*60)),
		finishedAt: time.Date(2026, 7, 17, 14, 35, 12, 0, time.FixedZone("CST", 8*60*60)),
	})
	if len(doc.Modules) != 2 || doc.Modules[0].Role != metadata.RoleMain || doc.Modules[1].Role != metadata.RoleDependency {
		t.Fatalf("unexpected module order: %#v", doc.Modules)
	}
	if doc.Modules[1].Commits == nil || len(doc.Modules[1].Commits) != 0 {
		t.Fatalf("failed history must be [], got %#v", doc.Modules[1].Commits)
	}
	if doc.Modules[0].Commits[0].Description != strings.Repeat("界", 100)+"..." || doc.Build.DurationMs != 71000 {
		t.Fatalf("unexpected document: %#v", doc)
	}
}
```

- [ ] **Step 2: Run RED**

```powershell
go test ./internal/deploy -run TestBuildMetadataOrdersMainBeforeDependenciesAndUsesEmptyCommitsOnFailure -v
```

Expected: compilation fails because `buildTiming`, `buildMetadata`, and the
injectable clock are absent.

- [ ] **Step 3: Add the assembler**

Add private fields to `Deployer` and initialize them in `New`:

```go
	now func() time.Time
	writeMetadata func(string, metadata.Document) error
```

```go
		now: time.Now,
		writeMetadata: metadata.Write,
```

Create `internal/deploy/metadata.go`:

```go
package deploy

import (
	"context"
	"time"

	"frank-remote-repo-deploy-agent/internal/gitops"
	"frank-remote-repo-deploy-agent/internal/metadata"
	"frank-remote-repo-deploy-agent/internal/output"
)

const recentCommitLimit = 3

type buildTiming struct {
	startedAt    time.Time
	finishedAt   time.Time
	buildSkipped bool
}

func (d *Deployer) buildMetadata(ctx context.Context, opts Options, branch, mainModule string, timing buildTiming) metadata.Document {
	moduleNames := append([]string{mainModule}, d.Config.Modules[mainModule].Dependencies...)
	modules := make([]metadata.Module, 0, len(moduleNames))
	for index, name := range moduleNames {
		role := metadata.RoleDependency
		if index == 0 {
			role = metadata.RoleMain
		}
		commits := make([]metadata.Commit, 0)
		records, err := gitops.RecentCommits(ctx, d.Output, d.moduleDir(name), recentCommitLimit)
		if err != nil {
			output.Warning("read recent commits module=%s: %v", name, err)
		} else {
			for _, record := range records {
				commits = append(commits, metadata.Commit{
					Hash: record.Hash,
					Committer: metadata.Committer{
						Name:  record.CommitterName,
						Email: record.CommitterEmail,
					},
					CommittedAt: record.CommittedAt,
					Description: metadata.TruncateDescription(record.Description),
				})
			}
		}
		modules = append(modules, metadata.Module{Name: name, Role: role, Commits: commits})
	}
	durationMs := timing.finishedAt.Sub(timing.startedAt).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}
	return metadata.Document{
		SchemaVersion: metadata.SchemaVersion,
		GeneratedAt:   d.currentTime().Format(time.RFC3339Nano),
		Environment:   opts.Env,
		Branch:        branch,
		MainModule:    mainModule,
		Build: metadata.Build{
			StartedAt:    timing.startedAt.Format(time.RFC3339Nano),
			FinishedAt:   timing.finishedAt.Format(time.RFC3339Nano),
			DurationMs:   durationMs,
			BuildSkipped: timing.buildSkipped,
		},
		Modules: modules,
	}
}

func (d *Deployer) currentTime() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now()
}
```

- [ ] **Step 4: Run GREEN and commit**

```powershell
gofmt -w internal/deploy/deploy.go internal/deploy/deploy_test.go internal/deploy/metadata.go
go test ./internal/deploy -run TestBuildMetadataOrdersMainBeforeDependenciesAndUsesEmptyCommitsOnFailure -v
git add internal/deploy/deploy.go internal/deploy/deploy_test.go internal/deploy/metadata.go
git commit -m "feat(deploy): assemble build metadata"
```

### Task 4: Publish Metadata Through Staging and Rsync

**Files:**
- Modify: `internal/deploy/deploy.go`
- Modify: `internal/deploy/deploy_test.go`

- [ ] **Step 1: Write failing pipeline integration tests**

Add `encoding/json`, `time`, and `internal/metadata` to the test imports, then add:

```go
func TestDeployOneWritesMetadataIntoStagingOnFullCacheHit(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(`<project><modules></modules></project>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := makeWar(filepath.Join(root, "example-app", "target", "app.war")); err != nil {
		t.Fatal(err)
	}
	store, err := cache.Load(filepath.Join(root, "cache", "build-cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.Update("example-common", "test", "dep-head")
	store.Update("example-app", "test", "main-head")
	store.UpdateSnapshot("example-app", "test", map[string]string{
		"example-common": "dep-head",
		"example-app":    "main-head",
	})
	run := &recordingRunner{
		commits: map[string]string{
			filepath.Join(root, "example-common"): "dep-head",
			filepath.Join(root, "example-app"):    "main-head",
		},
		logs: map[string]string{
			filepath.Join(root, "example-app"): "0123456789abcdef0123456789abcdef01234567\x00Frank Zhou\x00frank@example.com\x002026-07-17T13:20:30+08:00\x00Deploy metadata\n",
			filepath.Join(root, "example-common"): "89abcdef0123456789abcdef0123456789abcdef\x00Developer\x00dev@example.com\x002026-07-16T18:10:00+08:00\x00Dependency update\n",
		},
	}
	d := New(deployTestConfig(root), run, run, store)
	d.now = clockSequence(
		time.Date(2026, 7, 17, 14, 34, 1, 0, time.FixedZone("CST", 8*60*60)),
		time.Date(2026, 7, 17, 14, 35, 12, 0, time.FixedZone("CST", 8*60*60)),
		time.Date(2026, 7, 17, 14, 35, 13, 0, time.FixedZone("CST", 8*60*60)),
	)

	if err := d.deployOne(context.Background(), Options{Env: "demo"}, "example-app"); err != nil {
		t.Fatalf("deployOne returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "staging", "example-app", metadata.Filename))
	if err != nil {
		t.Fatal(err)
	}
	var doc metadata.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.Build.BuildSkipped || doc.Build.DurationMs != 71000 {
		t.Fatalf("unexpected build metadata: %#v", doc.Build)
	}
	if len(doc.Modules) != 2 || len(doc.Modules[0].Commits) != 1 || len(doc.Modules[1].Commits) != 1 {
		t.Fatalf("unexpected module metadata: %#v", doc.Modules)
	}
}
```

Add a JAR regression test to prove the file is placed next to, rather than
inside, the artifact:

```go
func TestDeployOneWritesMetadataNextToJar(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(`<project><modules></modules></project>`), 0o600); err != nil {
		t.Fatal(err)
	}
	jarPath := filepath.Join(root, "example-app", "target", "app.jar")
	if err := os.MkdirAll(filepath.Dir(jarPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jarPath, []byte("jar-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := deployTestConfig(root)
	main := cfg.Modules["example-app"]
	main.Packaging = "jar"
	main.Dependencies = nil
	cfg.Modules["example-app"] = main
	store, err := cache.Load(filepath.Join(root, "cache", "build-cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.Update("example-app", "test", "main-head")
	store.UpdateSnapshot("example-app", "test", map[string]string{"example-app": "main-head"})
	run := &recordingRunner{
		commits: map[string]string{filepath.Join(root, "example-app"): "main-head"},
		logs: map[string]string{filepath.Join(root, "example-app"): "0123456789abcdef0123456789abcdef01234567\x00Frank Zhou\x00frank@example.com\x002026-07-17T13:20:30+08:00\x00Deploy JAR\n"},
	}
	d := New(cfg, run, run, store)

	if err := d.deployOne(context.Background(), Options{Env: "demo"}, "example-app"); err != nil {
		t.Fatalf("deployOne returned error: %v", err)
	}
	staging := filepath.Join(root, "staging", "example-app")
	if _, err := os.Stat(filepath.Join(staging, "app.jar")); err != nil {
		t.Fatalf("staged JAR missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(staging, metadata.Filename)); err != nil {
		t.Fatalf("metadata missing next to JAR: %v", err)
	}
}
```

Add the required writer-failure test:

```go
func TestDeployOneContinuesRsyncAndRestartWhenMetadataWriteFails(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(`<project><modules></modules></project>`), 0o600); err != nil { t.Fatal(err) }
	run := &recordingRunner{}
	d := New(deployTestConfig(root), run, run, nil)
	d.writeMetadata = func(string, metadata.Document) error { return errors.New("disk full") }
	if err := d.deployOne(context.Background(), Options{Env: "demo"}, "example-app"); err != nil {
		t.Fatalf("deployOne returned error: %v", err)
	}
	if countCommands(run.commands, "rsync") != 1 || countCommands(run.commands, "ssh") != 1 {
		t.Fatalf("metadata failure blocked deployment: %#v", run.commands)
	}
	if _, err := os.Stat(filepath.Join(root, "staging", "example-app", metadata.Filename)); !os.IsNotExist(err) {
		t.Fatalf("failed metadata must remain absent, got %v", err)
	}
}
```

Use a deterministic test clock:

```go
func clockSequence(values ...time.Time) func() time.Time {
	index := 0
	return func() time.Time {
		if index >= len(values) { return values[len(values)-1] }
		value := values[index]
		index++
		return value
	}
}
```

- [ ] **Step 2: Run RED**

```powershell
go test ./internal/deploy -run 'TestDeployOne(WritesMetadata|ContinuesRsync)' -v
```

Expected: the staging file assertion fails because deploy does not publish it.

- [ ] **Step 3: Add timing and best-effort publication**

Immediately before the dependency cache/build loop:

```go
buildStartedAt := d.currentTime()
buildSkipped := true
```

Set `buildSkipped = false` immediately before each existing dependency or main
Maven `withMavenLock` call. After the main build/cache branch succeeds:

```go
buildFinishedAt := d.currentTime()
buildDocument := d.buildMetadata(ctx, opts, branch, moduleName, buildTiming{
	startedAt: buildStartedAt, finishedAt: buildFinishedAt, buildSkipped: buildSkipped,
})
```

After `PrepareStaging` succeeds, but before the lease check and rsync:

```go
writer := d.writeMetadata
if writer == nil { writer = metadata.Write }
if err := writer(staging, buildDocument); err != nil {
	output.Warning("write build metadata module=%s: %v", moduleName, err)
} else {
	output.Debug("build metadata written module=%s path=%s", moduleName, filepath.Join(staging, metadata.Filename))
}
```

- [ ] **Step 4: Run GREEN, adjacent regressions, and commit**

```powershell
gofmt -w internal/deploy/deploy.go internal/deploy/deploy_test.go
go test ./internal/deploy -v
go test ./internal/gitops ./internal/metadata ./internal/packagex ./internal/rsync ./internal/deploy
git add internal/deploy/deploy.go internal/deploy/deploy_test.go
git commit -m "feat(deploy): sync build metadata with artifacts"
```

Expected: all listed tests pass. Writer failure logs a warning while rsync and
restart still execute.

### Task 5: Publish the Downstream Contract

**Files:**
- Create: `docs/schemas/salt-agent-metadata.schema.json`
- Create: `docs/salt-agent-metadata.md`
- Modify: `README.md`
- Modify: `README.zh-CN.md`

- [ ] **Step 1: Create the JSON Schema**

Create `docs/schemas/salt-agent-metadata.schema.json` with this complete JSON
Schema 2020-12 contract:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "salt-agent-metadata.schema.json",
  "title": "Salt Agent Build Metadata",
  "type": "object",
  "required": ["schemaVersion", "generatedAt", "environment", "branch", "mainModule", "build", "modules"],
  "properties": {
    "schemaVersion": { "const": "1.0" },
    "generatedAt": { "type": "string", "format": "date-time" },
    "environment": { "type": "string" },
    "branch": { "type": "string" },
    "mainModule": { "type": "string" },
    "build": { "$ref": "#/$defs/build" },
    "modules": {
      "type": "array",
      "minItems": 1,
      "items": { "$ref": "#/$defs/module" }
    }
  },
  "$defs": {
    "build": {
      "type": "object",
      "required": ["startedAt", "finishedAt", "durationMs", "buildSkipped"],
      "properties": {
        "startedAt": { "type": "string", "format": "date-time" },
        "finishedAt": { "type": "string", "format": "date-time" },
        "durationMs": { "type": "integer", "minimum": 0 },
        "buildSkipped": { "type": "boolean" }
      },
      "additionalProperties": true
    },
    "module": {
      "type": "object",
      "required": ["name", "role", "commits"],
      "properties": {
        "name": { "type": "string" },
        "role": { "enum": ["main", "dependency"] },
        "commits": {
          "type": "array",
          "maxItems": 3,
          "items": { "$ref": "#/$defs/commit" }
        }
      },
      "additionalProperties": true
    },
    "commit": {
      "type": "object",
      "required": ["hash", "committer", "committedAt", "description"],
      "properties": {
        "hash": { "type": "string", "pattern": "^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$" },
        "committer": { "$ref": "#/$defs/committer" },
        "committedAt": { "type": "string", "format": "date-time" },
        "description": { "type": "string", "maxLength": 103 }
      },
      "additionalProperties": true
    },
    "committer": {
      "type": "object",
      "required": ["name", "email"],
      "properties": {
        "name": { "type": "string" },
        "email": { "type": "string" }
      },
      "additionalProperties": true
    }
  },
  "additionalProperties": true
}
```

- [ ] **Step 2: Write the complete consumer document**

Create `docs/salt-agent-metadata.md` with the following complete contract text;
expand the example to include both main and dependency modules exactly as shown
in the approved design, without introducing fields absent from the schema:

````markdown
# salt-agent-metadata.json 下游接入协议

## 用途与位置

`salt-agent` 在每个主模块的 staging 根目录生成 UTF-8 JSON 文件
`salt-agent-metadata.json`，并与应用文件一起通过 `rsync` 同步到该模块的
`remotePath` 根目录。WAR 部署时它与解压后的应用内容同级；JAR 部署时它与
JAR 文件同级。该文件描述已同步产物的构建和 Git 来源，但不证明远程重启或
健康检查成功。

机器可读定义见
[`docs/schemas/salt-agent-metadata.schema.json`](schemas/salt-agent-metadata.schema.json)。

## 完整示例

```json
{
  "schemaVersion": "1.0",
  "generatedAt": "2026-07-17T14:35:12+08:00",
  "environment": "test",
  "branch": "test",
  "mainModule": "example-app",
  "build": {
    "startedAt": "2026-07-17T14:34:01+08:00",
    "finishedAt": "2026-07-17T14:35:12+08:00",
    "durationMs": 71000,
    "buildSkipped": false
  },
  "modules": [
    {
      "name": "example-app",
      "role": "main",
      "commits": [{
        "hash": "0123456789abcdef0123456789abcdef01234567",
        "committer": { "name": "Frank Zhou", "email": "frank@example.com" },
        "committedAt": "2026-07-17T13:20:30+08:00",
        "description": "Add build metadata"
      }]
    },
    {
      "name": "example-common",
      "role": "dependency",
      "commits": []
    }
  ]
}
```

## 字段定义

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `schemaVersion` | string | 是 | 当前为 `1.0`。 |
| `generatedAt` | RFC 3339 string | 是 | 元数据模型生成时间。 |
| `environment` | string | 是 | `deploy --env` 的环境名。 |
| `branch` | string | 是 | 环境解析出的 Git 分支。 |
| `mainModule` | string | 是 | 本文件对应的主模块。 |
| `build.startedAt` | RFC 3339 string | 是 | Maven 判断/构建阶段开始时间。 |
| `build.finishedAt` | RFC 3339 string | 是 | Maven 阶段成功完成时间。 |
| `build.durationMs` | integer | 是 | 阶段耗时，单位毫秒且不小于零。 |
| `build.buildSkipped` | boolean | 是 | 所有模块均命中缓存、没有执行 Maven 时为 `true`。 |
| `modules` | array | 是 | 主模块在前，依赖按配置顺序排列。 |
| `modules[].name` | string | 是 | 配置中的模块名。 |
| `modules[].role` | string | 是 | `main` 或 `dependency`。 |
| `modules[].commits` | array | 是 | 从本地 HEAD 开始最新最多三条提交。 |
| `hash` | string | 是 | 完整 Git 对象 ID，不允许按固定长度截断。 |
| `committer.name` | string | 是 | Git committer 姓名。 |
| `committer.email` | string | 是 | Git committer 邮箱。 |
| `committedAt` | RFC 3339 string | 是 | Git committer 时间并保留原时区。 |
| `description` | string | 是 | 提交标题，不包含正文。 |

## 数量、排序与截断

每个模块按新到旧输出零至三条提交。仓库只有一条或两条记录时按实际数量
输出；读取或解析失败时必须输出 `"commits": []`。描述按 Unicode 字符而非
UTF-8 字节计数；超过 100 个字符时保留前 100 个字符并追加 `...`，三个点
不计入 100 字符上限。

## 缓存命中示例

全部模块命中构建缓存时仍生成新元数据：

```json
"build": {
  "startedAt": "2026-07-17T14:40:00+08:00",
  "finishedAt": "2026-07-17T14:40:00+08:00",
  "durationMs": 0,
  "buildSkipped": true
}
```

`durationMs` 是本次 Maven 判断/构建阶段的实际耗时，不要求缓存命中时严格
等于零。只要任意模块执行 Maven，`buildSkipped` 就是 `false`。

## 降级与发布生命周期

- 单个模块的 Git 日志获取失败：该模块写入 `commits: []`，部署继续。
- JSON 序列化或文件写入失败：记录 warning，应用 rsync 和重启继续。
- 写入失败时 staging 不含元数据；现有 `rsync --delete` 会删除远端旧文件，
  避免下游误读旧版本。
- Maven 失败：保持原有失败流程，不同步本次应用和元数据。
- rsync 成功但重启失败：远端可能已有新元数据，因此文件存在不表示服务已
  成功启动。

## 兼容与解析建议

消费者必须检查 `schemaVersion` 的主版本。支持 `1.x` 时应忽略未知字段，
始终把 hash 当作不透明字符串处理，并允许 `commits` 为零至三项，不能假设
固定为三项。兼容的新增字段提升次版本；删除字段或改变既有字段语义必须提升
主版本。解析时间时使用支持 RFC 3339 时区偏移的类型，不要转换为本地无时区
时间后再比较。
````

- [ ] **Step 3: Link the protocol from both READMEs**

Add to `README.md`:

```markdown
- Build metadata: each staged deployment includes [`salt-agent-metadata.json`](docs/salt-agent-metadata.md) with build timing and up to three recent commits for the main module and every configured dependency.
```

Add to `README.zh-CN.md`:

```markdown
- 构建元数据：每次 staging 都会包含 [`salt-agent-metadata.json`](docs/salt-agent-metadata.md)，记录构建时间以及主模块和各依赖模块最近最多三条提交。
```

- [ ] **Step 4: Validate and commit documentation**

```powershell
Get-Content -Raw docs/schemas/salt-agent-metadata.schema.json | ConvertFrom-Json | Out-Null
rg -n 'schemaVersion|buildSkipped|commits|durationMs|100|rsync --delete|重启' docs/salt-agent-metadata.md
git diff --check
git add docs/schemas/salt-agent-metadata.schema.json docs/salt-agent-metadata.md README.md README.zh-CN.md
git commit -m "docs: publish build metadata contract"
```

Expected: schema parsing and whitespace checks exit 0, and the search finds all
normative concepts.

### Task 6: Full Verification and Requirement Audit

**Files:**
- Verify all modified files

- [ ] **Step 1: Format all changed Go files**

```powershell
gofmt -w internal/gitops/gitops.go internal/gitops/gitops_test.go internal/metadata/metadata.go internal/metadata/metadata_test.go internal/deploy/deploy.go internal/deploy/deploy_test.go internal/deploy/metadata.go
```

- [ ] **Step 2: Run the complete test suite**

```powershell
go test ./...
```

Expected: every package reports `ok` or `[no test files]` with no failures.

- [ ] **Step 3: Build the CLI**

```powershell
go build -o salt-agent ./cmd/salt-agent
```

Expected: exit 0. Do not stage the generated executable.

- [ ] **Step 4: Validate docs and whitespace**

```powershell
Get-Content -Raw docs/schemas/salt-agent-metadata.schema.json | ConvertFrom-Json | Out-Null
git diff --check
git status --short
```

- [ ] **Step 5: Audit the approved contract**

Confirm from fresh output and file inspection: exact file name and staging-root
location; main-first ordering; zero through three newest commits; full hash,
committer identity/time, and subject; 100 Unicode characters plus `...`; start,
finish, duration, and full-cache skip state; `commits: []` degradation; continued
rsync/restart after writer failure; unchanged WAR/JAR semantics; and agreement
between Go types, examples, consumer docs, and JSON Schema.

- [ ] **Step 6: Commit only verification corrections, if any**

```powershell
git add internal docs README.md README.zh-CN.md
git commit -m "fix: align build metadata contract"
```

Do not create an empty commit when verification needs no correction.
