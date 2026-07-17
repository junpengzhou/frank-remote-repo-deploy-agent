package deploy

import (
	"archive/zip"
	"context"
	"errors"
	"frank-remote-repo-deploy-agent/internal/cache"
	"frank-remote-repo-deploy-agent/internal/config"
	"frank-remote-repo-deploy-agent/internal/lock"
	"frank-remote-repo-deploy-agent/internal/metadata"
	"frank-remote-repo-deploy-agent/internal/output"
	"frank-remote-repo-deploy-agent/internal/runner"
	"frank-remote-repo-deploy-agent/internal/testutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnsurePomModuleOnlyPrintsPomLogInDebugMode(t *testing.T) {
	defaultOutput := ensurePomModuleOutput(t, false, "example-default")
	if strings.Contains(defaultOutput, "pom appended missing module") {
		t.Fatalf("expected no pom log without debug, got %q", defaultOutput)
	}

	debugOutput := ensurePomModuleOutput(t, true, "example-debug")
	if !strings.Contains(debugOutput, "\x1b[36m[DEBUG]pom appended missing module example-debug") {
		t.Fatalf("expected pom log in debug mode, got %q", debugOutput)
	}
}

func TestRunRsyncWithRetryRetriesTransientFailures(t *testing.T) {
	run := &failingRunner{failuresBeforeSuccess: 2}
	cmd := runner.Command{Name: "rsync", Args: []string{"-az"}}

	err := runRsyncWithRetry(context.Background(), run, cmd, 3, 0)

	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if run.calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", run.calls)
	}
}

func TestRunRsyncWithRetryReturnsLastErrorAfterRetries(t *testing.T) {
	run := &failingRunner{failuresBeforeSuccess: 99}
	cmd := runner.Command{Name: "rsync", Args: []string{"-az"}}

	err := runRsyncWithRetry(context.Background(), run, cmd, 2, 0)

	if err == nil || !strings.Contains(err.Error(), "temporary rsync failure") {
		t.Fatalf("expected final rsync error, got %v", err)
	}
	if run.calls != 3 {
		t.Fatalf("expected initial attempt plus 2 retries, got %d", run.calls)
	}
}

func TestDeployOnePrintsDebugContextForCheckoutArtifactAndRsync(t *testing.T) {
	output.SetDebug(true)
	t.Cleanup(func() {
		output.SetDebug(false)
	})
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(`<project><modules></modules></project>`), 0o600); err != nil {
		t.Fatal(err)
	}
	run := &recordingRunner{}
	d := New(&config.Config{
		BuildRoot:  root,
		StagingDir: filepath.Join(root, "staging"),
		LockDir:    filepath.Join(root, "locks"),
		Maven:      config.MavenConfig{Executable: "mvn"},
		SSH:        config.SSHConfig{User: "root", Host: "127.0.0.1"},
		Environments: map[string]config.EnvConfig{
			"demo": {Branch: "test"},
		},
		Modules: map[string]config.Module{
			"ifintech-im-export": {
				Repo:       "git@example.com/export.git",
				Packaging:  "war",
				RemotePath: "/data/app/",
				Container:  "im-export",
			},
		},
	}, run, run, nil)

	logs := testutil.CaptureStdout(t, func() {
		err := d.deployOne(context.Background(), Options{Env: "demo"}, "ifintech-im-export")
		if err != nil {
			t.Fatalf("deployOne returned error: %v", err)
		}
	})

	moduleDir := filepath.Join(root, "ifintech-im-export")
	if !strings.Contains(logs, "checkout module ifintech-im-export repo=git@example.com/export.git branch=test dir="+moduleDir) {
		t.Fatalf("expected checkout debug context, got %q", logs)
	}
	if !strings.Contains(logs, "artifact selected module=ifintech-im-export path="+filepath.Join(moduleDir, "target", "app.war")) {
		t.Fatalf("expected artifact debug context, got %q", logs)
	}
	if !strings.Contains(logs, "rsync module=ifintech-im-export source="+filepath.Join(root, "staging", "ifintech-im-export")+" remotePath=/data/app/") {
		t.Fatalf("expected rsync debug context, got %q", logs)
	}
}

func TestDeployOneSkipsMavenWhenDependenciesAndMainModuleAreCacheHits(t *testing.T) {
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
	store.Update("example-common", "test", "abc123")
	store.Update("example-app", "test", "abc123")
	store.UpdateSnapshot("example-app", "test", map[string]string{
		"example-common": "abc123",
		"example-app":    "abc123",
	})
	run := &recordingRunner{}
	d := New(deployTestConfig(root), run, run, store)

	if err := d.deployOne(context.Background(), Options{Env: "demo"}, "example-app"); err != nil {
		t.Fatalf("deployOne returned error: %v", err)
	}

	if got := countCommands(run.commands, "mvn"); got != 0 {
		t.Fatalf("expected no maven commands on full cache hit, got %d commands: %#v", got, run.commands)
	}
	if got := countCommands(run.commands, "rsync"); got != 1 {
		t.Fatalf("expected rsync to continue after skipped build, got %d commands: %#v", got, run.commands)
	}
	if got := countCommands(run.commands, "ssh"); got != 1 {
		t.Fatalf("expected restart to continue after skipped build, got %d commands: %#v", got, run.commands)
	}
}

func TestDeployOneBuildsMainModuleWhenDependencyCacheMisses(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(`<project><modules></modules></project>`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := cache.Load(filepath.Join(root, "cache", "build-cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.Update("example-app", "test", "abc123")
	run := &recordingRunner{}
	d := New(deployTestConfig(root), run, run, store)

	if err := d.deployOne(context.Background(), Options{Env: "demo"}, "example-app"); err != nil {
		t.Fatalf("deployOne returned error: %v", err)
	}

	if got := countCommands(run.commands, "mvn"); got != 2 {
		t.Fatalf("expected dependency and main maven commands, got %d commands: %#v", got, run.commands)
	}
}

func TestDeployOneBuildsMainModuleWhenOwnDependencySnapshotIsStale(t *testing.T) {
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
	store.Update("example-common", "test", "def456")
	store.Update("example-app", "test", "abc123")
	run := &recordingRunner{
		commits: map[string]string{
			filepath.Join(root, "example-common"): "def456",
			filepath.Join(root, "example-app"):    "abc123",
		},
	}
	d := New(deployTestConfig(root), run, run, store)

	if err := d.deployOne(context.Background(), Options{Env: "demo"}, "example-app"); err != nil {
		t.Fatalf("deployOne returned error: %v", err)
	}

	if got := countCommands(run.commands, "mvn"); got != 1 {
		t.Fatalf("expected main module rebuild only, got %d commands: %#v", got, run.commands)
	}
	if got := findMavenModules(run.commands); len(got) != 1 || got[0] != "example-app" {
		t.Fatalf("expected only example-app to rebuild, got %#v", got)
	}
}

func TestBuildMetadataOrdersMainBeforeDependenciesAndUsesEmptyCommitsOnFailure(t *testing.T) {
	root := t.TempDir()
	mainDir := filepath.Join(root, "example-app")
	depDir := filepath.Join(root, "example-common")
	run := &recordingRunner{
		logs: map[string]string{
			mainDir: "0123456789abcdef0123456789abcdef01234567\x00Frank Zhou\x00frank@example.com\x002026-07-17T13:20:30+08:00\x00" + strings.Repeat("界", 101) + "\n",
		},
		logErrors: map[string]error{
			depDir: errors.New("history unavailable"),
		},
	}
	d := New(deployTestConfig(root), run, run, nil)
	d.now = func() time.Time {
		return time.Date(2026, 7, 17, 14, 35, 12, 0, time.FixedZone("CST", 8*60*60))
	}

	doc := d.buildMetadata(context.Background(), Options{Env: "demo"}, "test", "example-app", buildTiming{
		startedAt:  time.Date(2026, 7, 17, 14, 34, 1, 0, time.FixedZone("CST", 8*60*60)),
		finishedAt: time.Date(2026, 7, 17, 14, 35, 12, 0, time.FixedZone("CST", 8*60*60)),
	})

	if len(doc.Modules) != 2 ||
		doc.Modules[0].Name != "example-app" ||
		doc.Modules[0].Role != metadata.RoleMain ||
		doc.Modules[1].Name != "example-common" ||
		doc.Modules[1].Role != metadata.RoleDependency {
		t.Fatalf("unexpected module order: %#v", doc.Modules)
	}
	if doc.Modules[1].Commits == nil || len(doc.Modules[1].Commits) != 0 {
		t.Fatalf("failed history must be [], got %#v", doc.Modules[1].Commits)
	}
	if got := doc.Modules[0].Commits[0].Description; got != strings.Repeat("界", 100)+"..." {
		t.Fatalf("unexpected truncated description: %q", got)
	}
	if doc.Build.DurationMs != 71000 ||
		doc.GeneratedAt != "2026-07-17T14:35:12+08:00" ||
		doc.Environment != "demo" ||
		doc.Branch != "test" {
		t.Fatalf("unexpected document: %#v", doc)
	}
}

func ensurePomModuleOutput(t *testing.T, debug bool, module string) string {
	t.Helper()
	output.SetDebug(debug)
	t.Cleanup(func() {
		output.SetDebug(false)
	})

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(`<project><modules></modules></project>`), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &Deployer{Config: &config.Config{
		BuildRoot: root,
		LockDir:   filepath.Join(root, "locks"),
	}}
	d.Locks = lock.NewManager(d.Config.LockDir)

	return testutil.CaptureStdout(t, func() {
		if err := d.ensurePomModule(context.Background(), module); err != nil {
			t.Fatalf("ensurePomModule returned error: %v", err)
		}
	})
}

func deployTestConfig(root string) *config.Config {
	return &config.Config{
		BuildRoot:  root,
		StagingDir: filepath.Join(root, "staging"),
		LockDir:    filepath.Join(root, "locks"),
		Maven:      config.MavenConfig{Executable: "mvn"},
		SSH:        config.SSHConfig{User: "root", Host: "127.0.0.1"},
		Environments: map[string]config.EnvConfig{
			"demo": {Branch: "test"},
		},
		Modules: map[string]config.Module{
			"example-common": {
				Repo:      "git@example.com/common.git",
				Packaging: "jar",
			},
			"example-app": {
				Repo:         "git@example.com/app.git",
				Packaging:    "war",
				Dependencies: []string{"example-common"},
				RemotePath:   "/data/app/",
				Container:    "example-app",
			},
		},
	}
}

func countCommands(commands []runner.Command, name string) int {
	count := 0
	for _, cmd := range commands {
		if cmd.Name == name {
			count++
		}
	}
	return count
}

type recordingRunner struct {
	commands  []runner.Command
	commits   map[string]string
	logs      map[string]string
	logErrors map[string]error
}

func (r *recordingRunner) Run(_ context.Context, cmd runner.Command) error {
	r.commands = append(r.commands, cmd)
	if cmd.Name == "mvn" {
		artifact := filepath.Join(cmd.Dir, cmd.Args[3], "target", "app.war")
		if err := makeWar(artifact); err != nil {
			return err
		}
	}
	return nil
}

func (r *recordingRunner) Output(_ context.Context, cmd runner.Command) (string, error) {
	if len(cmd.Args) > 0 && cmd.Args[0] == "log" {
		if err := r.logErrors[cmd.Dir]; err != nil {
			return "", err
		}
		return r.logs[cmd.Dir], nil
	}
	if commit, ok := r.commits[cmd.Dir]; ok {
		return commit + "\n", nil
	}
	return "abc123\n", nil
}

func findMavenModules(commands []runner.Command) []string {
	var modules []string
	for _, cmd := range commands {
		if cmd.Name != "mvn" || len(cmd.Args) < 4 {
			continue
		}
		modules = append(modules, cmd.Args[3])
	}
	return modules
}

type failingRunner struct {
	calls                 int
	failuresBeforeSuccess int
}

func (r *failingRunner) Run(_ context.Context, _ runner.Command) error {
	r.calls++
	if r.calls <= r.failuresBeforeSuccess {
		return errors.New("temporary rsync failure")
	}
	return nil
}

func makeWar(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(out)
	file, err := writer.Create("WEB-INF/classes/App.class")
	if err != nil {
		_ = out.Close()
		return err
	}
	if _, err := file.Write([]byte("bytecode")); err != nil {
		_ = out.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
