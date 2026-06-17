package register

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestNativeAddressUsesDefaultPort(t *testing.T) {
	got := nativeAddress(NativeOptions{Host: "10.0.0.1"})

	if got != "10.0.0.1:22" {
		t.Fatalf("expected default SSH port, got %q", got)
	}
}

func TestBuildSSHClientConfigUsesPasswordAndTimeout(t *testing.T) {
	cfg, err := buildSSHClientConfig(NativeOptions{
		SSHDir:   t.TempDir(),
		User:     "root",
		Password: "secret",
		Timeout:  7 * time.Second,
	})

	if err != nil {
		t.Fatalf("buildSSHClientConfig returned error: %v", err)
	}
	if cfg.User != "root" {
		t.Fatalf("expected user root, got %q", cfg.User)
	}
	if cfg.Timeout != 7*time.Second {
		t.Fatalf("expected custom timeout, got %v", cfg.Timeout)
	}
	if len(cfg.Auth) != 1 {
		t.Fatalf("expected password auth method, got %d", len(cfg.Auth))
	}
	if cfg.HostKeyCallback == nil {
		t.Fatal("expected host key callback")
	}
}

func TestNewNativeClientPreparesSSHDirBeforeDial(t *testing.T) {
	sshDir := filepath.Join(t.TempDir(), ".ssh")

	_, err := NewNativeClient(context.Background(), NativeOptions{
		SSHDir:   sshDir,
		Host:     "127.0.0.1",
		Port:     1,
		User:     "root",
		Password: "secret",
		Timeout:  10 * time.Millisecond,
	})

	if err == nil {
		t.Fatal("expected dial error for closed port")
	}
	info, statErr := os.Stat(sshDir)
	if statErr != nil {
		t.Fatalf("expected ssh dir to be created: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatalf("expected ssh dir, got file")
	}
}

func TestHostKeyCallbackStoresKnownHostInSSHDir(t *testing.T) {
	sshDir := t.TempDir()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	callback, err := hostKeyCallback(sshDir)
	if err != nil {
		t.Fatalf("hostKeyCallback returned error: %v", err)
	}
	err = callback("10.0.0.1:22", &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 22}, publicKey)

	if err != nil {
		t.Fatalf("callback returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(sshDir, "known_hosts"))
	if err != nil {
		t.Fatalf("expected known_hosts file: %v", err)
	}
	if !strings.Contains(string(data), "10.0.0.1") {
		t.Fatalf("expected host entry in known_hosts, got %q", string(data))
	}
}
