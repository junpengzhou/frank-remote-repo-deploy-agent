package deploy

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"frank-remote-repo-deploy-agent/internal/config"
	"frank-remote-repo-deploy-agent/internal/lock"
	"frank-remote-repo-deploy-agent/internal/output"
	"frank-remote-repo-deploy-agent/internal/runner"
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

func TestWaitForHealthSucceedsOnHTTP200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := waitForHealth(context.Background(), server.URL, 100*time.Millisecond)

	if err != nil {
		t.Fatalf("expected health check success, got %v", err)
	}
}

func TestWaitForHealthTimesOutUntilHTTP200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	err := waitForHealth(context.Background(), server.URL, 20*time.Millisecond)

	if err == nil || !strings.Contains(err.Error(), "health check timed out") {
		t.Fatalf("expected health timeout, got %v", err)
	}
}

func TestMonitorStartupWaitsForHealthThenPrintsTail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	run := &recordingRunner{}
	d := &Deployer{
		Config: &config.Config{SSH: config.SSHConfig{User: "root", Host: "127.0.0.1"}},
		Runner: run,
	}

	captureOutput := captureStdout(t, func() {
		err := d.monitorStartup(context.Background(), config.Module{
			LogFile:       "/data/logs/app.log",
			HealthURL:     server.URL,
			HealthTimeout: 100 * time.Millisecond,
		}, Options{TailLines: 10})
		if err != nil {
			t.Fatalf("monitorStartup returned error: %v", err)
		}
	})

	if !strings.Contains(captureOutput, "Waiting for application startup") {
		t.Fatalf("expected startup wait message, got %q", captureOutput)
	}
	if len(run.commands) != 1 {
		t.Fatalf("expected one tail command, got %#v", run.commands)
	}
	if got := run.commands[0].Args[len(run.commands[0].Args)-1]; got != "tail -n 10 '/data/logs/app.log'" {
		t.Fatalf("expected non-follow tail command, got %q", got)
	}
}

func TestMonitorStartupSuggestsManualCheckWhenOnlyLogFileConfigured(t *testing.T) {
	run := &recordingRunner{}
	d := &Deployer{
		Config: &config.Config{SSH: config.SSHConfig{User: "root", Host: "127.0.0.1"}},
		Runner: run,
	}

	captureOutput := captureStdout(t, func() {
		err := d.monitorStartup(context.Background(), config.Module{
			LogFile: "/data/logs/app.log",
		}, Options{TailLines: 10})
		if err != nil {
			t.Fatalf("monitorStartup returned error: %v", err)
		}
	})

	if len(run.commands) != 0 {
		t.Fatalf("expected no tail command, got %#v", run.commands)
	}
	if !strings.Contains(captureOutput, "Startup health check is not configured") {
		t.Fatalf("expected manual startup check message, got %q", captureOutput)
	}
	if !strings.Contains(captureOutput, "\x1b[33m[WARNING]") {
		t.Fatalf("expected yellow warning prefix, got %q", captureOutput)
	}
	if !strings.Contains(captureOutput, "tail -fn 10 /data/logs/app.log") {
		t.Fatalf("expected suggested tail command, got %q", captureOutput)
	}
}

func TestMonitorStartupPrintsSuccessWhenHealthHasNoLogFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	d := &Deployer{}

	captureOutput := captureStdout(t, func() {
		err := d.monitorStartup(context.Background(), config.Module{
			HealthURL:     server.URL,
			HealthTimeout: 100 * time.Millisecond,
		}, Options{})
		if err != nil {
			t.Fatalf("monitorStartup returned error: %v", err)
		}
	})

	if !strings.Contains(captureOutput, "Application started successfully, but logFile is not configured") {
		t.Fatalf("expected no-log success message, got %q", captureOutput)
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

	logs := captureStdout(t, func() {
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

	return captureStdout(t, func() {
		if err := d.ensurePomModule(context.Background(), module); err != nil {
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

type recordingRunner struct {
	commands []runner.Command
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

func (r *recordingRunner) Output(_ context.Context, _ runner.Command) (string, error) {
	return "abc123\n", nil
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
