package deploy

import (
	"bytes"
	"context"
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
	"frank-remote-repo-deploy-agent/internal/runner"
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

	output := captureStdout(t, func() {
		err := d.monitorStartup(context.Background(), config.Module{
			LogFile:       "/data/logs/app.log",
			HealthURL:     server.URL,
			HealthTimeout: 100 * time.Millisecond,
		}, Options{TailLines: 10})
		if err != nil {
			t.Fatalf("monitorStartup returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Waiting for application startup") {
		t.Fatalf("expected startup wait message, got %q", output)
	}
	if len(run.commands) != 1 {
		t.Fatalf("expected one tail command, got %#v", run.commands)
	}
	if got := run.commands[0].Args[len(run.commands[0].Args)-1]; got != "tail -n 10 '/data/logs/app.log'" {
		t.Fatalf("expected non-follow tail command, got %q", got)
	}
}

func TestMonitorStartupPrintsSuccessWhenHealthHasNoLogFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	d := &Deployer{}

	output := captureStdout(t, func() {
		err := d.monitorStartup(context.Background(), config.Module{
			HealthURL:     server.URL,
			HealthTimeout: 100 * time.Millisecond,
		}, Options{})
		if err != nil {
			t.Fatalf("monitorStartup returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Application started successfully, but logFile is not configured") {
		t.Fatalf("expected no-log success message, got %q", output)
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

type recordingRunner struct {
	commands []runner.Command
}

func (r *recordingRunner) Run(_ context.Context, cmd runner.Command) error {
	r.commands = append(r.commands, cmd)
	return nil
}
