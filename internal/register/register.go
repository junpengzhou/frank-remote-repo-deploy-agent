package register

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

type Options struct {
	SSHDir        string
	Host          string
	Port          int
	User          string
	Password      string
	RemotePath    string
	ScriptsDir    string
	PublicKeyPath string
}

type Client interface {
	Run(ctx context.Context, command string) error
	RunWithInput(ctx context.Context, command string, input string) error
	MkdirAll(ctx context.Context, path string) error
	UploadFile(ctx context.Context, localPath, remotePath string, mode os.FileMode) error
	Close() error
}

type KeyPair struct {
	PrivateKeyPath string
	PublicKeyPath  string
}

type Registrar struct {
	Client Client
}

func (r Registrar) Run(ctx context.Context, opts Options) error {
	if r.Client == nil {
		return errors.New("client is required")
	}
	// 参数校验
	if err := validate(opts); err != nil {
		return err
	}
	defer func() {
		_ = r.Client.Close()
	}()

	if opts.PublicKeyPath != "" {
		if err := r.uploadPublicKey(ctx, opts.PublicKeyPath); err != nil {
			return err
		}
		if err := r.Client.Run(ctx, "echo 'SSH connection successful'"); err != nil {
			return fmt.Errorf("verify password ssh connection: %w", err)
		}
	}
	if err := r.Client.Run(ctx, installScript(opts.RemotePath)); err != nil {
		return fmt.Errorf("run remote install script: %w", err)
	}
	remoteScripts := remoteJoin(opts.RemotePath, "scripts")
	if err := r.syncScripts(ctx, opts.ScriptsDir, remoteScripts); err != nil {
		return err
	}
	if err := r.Client.Run(ctx, "chmod -R a+rx "+shellQuote(remoteScripts)); err != nil {
		return fmt.Errorf("chmod remote scripts: %w", err)
	}
	return nil
}

func EnsureKeyPair(sshDir string, regenerate bool) (KeyPair, error) {
	if strings.TrimSpace(sshDir) == "" {
		return KeyPair{}, errors.New("ssh dir is required")
	}
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return KeyPair{}, fmt.Errorf("create ssh dir: %w", err)
	}
	pair := KeyPair{
		PrivateKeyPath: filepath.Join(sshDir, "id_rsa"),
		PublicKeyPath:  filepath.Join(sshDir, "id_rsa.pub"),
	}
	if !regenerate && fileExists(pair.PrivateKeyPath) && fileExists(pair.PublicKeyPath) {
		return pair, nil
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return KeyPair{}, fmt.Errorf("generate rsa key: %w", err)
	}
	privateDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateDER})
	if err := os.WriteFile(pair.PrivateKeyPath, privatePEM, 0o600); err != nil {
		return KeyPair{}, fmt.Errorf("write private key: %w", err)
	}
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return KeyPair{}, fmt.Errorf("encode public key: %w", err)
	}
	if err := os.WriteFile(pair.PublicKeyPath, ssh.MarshalAuthorizedKey(publicKey), 0o644); err != nil {
		return KeyPair{}, fmt.Errorf("write public key: %w", err)
	}
	return pair, nil
}

func (r Registrar) uploadPublicKey(ctx context.Context, publicKeyPath string) error {
	publicKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return fmt.Errorf("read public key: %w", err)
	}
	command := strings.Join([]string{
		"key=$(cat)",
		"mkdir -p ~/.ssh",
		"chmod 700 ~/.ssh",
		"touch ~/.ssh/authorized_keys",
		"grep -qxF \"$key\" ~/.ssh/authorized_keys || printf '%s\n' \"$key\" >> ~/.ssh/authorized_keys",
		"chmod 600 ~/.ssh/authorized_keys",
	}, " && ")
	if err := r.Client.RunWithInput(ctx, command, string(publicKey)); err != nil {
		return fmt.Errorf("upload public key: %w", err)
	}
	return nil
}

func validate(opts Options) error {
	if strings.TrimSpace(opts.SSHDir) == "" {
		return errors.New("ssh dir is required")
	}
	if strings.TrimSpace(opts.Host) == "" {
		return errors.New("host is required")
	}
	if strings.TrimSpace(opts.User) == "" {
		return errors.New("user is required")
	}
	if opts.Password == "" {
		return errors.New("password is required")
	}
	if strings.TrimSpace(opts.RemotePath) == "" {
		return errors.New("remote path is required")
	}
	if strings.TrimSpace(opts.ScriptsDir) == "" {
		return errors.New("scripts dir is required")
	}
	return nil
}

func (r Registrar) syncScripts(ctx context.Context, scriptsDir, remoteScripts string) error {
	return filepath.WalkDir(scriptsDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(scriptsDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return r.Client.MkdirAll(ctx, remoteScripts)
		}
		remotePath := remoteJoin(remoteScripts, filepath.ToSlash(rel))
		if entry.IsDir() {
			return r.Client.MkdirAll(ctx, remotePath)
		}
		mode := os.FileMode(0o644)
		if strings.EqualFold(filepath.Ext(entry.Name()), ".sh") {
			mode = 0o755
		}
		if err := r.Client.UploadFile(ctx, path, remotePath, mode); err != nil {
			return fmt.Errorf("upload script %s: %w", rel, err)
		}
		return nil
	})
}

func installScript(remotePath string) string {
	// 安装远程扩展脚本的环境变量，便于全局所有节点下都能执行
	quotedPath := shellQuote(remotePath)
	return strings.Join([]string{
		"set -e",
		"SALT_AGENT_HOME=" + quotedPath,
		"mkdir -p \"$SALT_AGENT_HOME\" \"$SALT_AGENT_HOME/scripts\"",
		"profile=/etc/profile.d/salt-agent.sh",
		"profile_content=\"export SALT_AGENT_HOME=$SALT_AGENT_HOME\nexport PATH=\\$PATH:\\$SALT_AGENT_HOME:\\$SALT_AGENT_HOME/scripts\"",
		"if command -v sudo >/dev/null 2>&1; then",
		"  printf '%s\n' \"$profile_content\" | sudo tee \"$profile\" >/dev/null",
		"else",
		"  printf '%s\n' \"$profile_content\" > \"$profile\"",
		"fi",
		"echo \"Salt-Agent registered at $SALT_AGENT_HOME\"",
	}, "\n")
}

func remoteJoin(base string, parts ...string) string {
	result := strings.TrimRight(base, "/")
	for _, part := range parts {
		clean := strings.Trim(part, "/")
		if clean == "" {
			continue
		}
		result += "/" + clean
	}
	if result == "" {
		return "/"
	}
	return result
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
