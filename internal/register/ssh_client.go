package register

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type NativeOptions struct {
	SSHDir   string
	Host     string
	Port     int
	User     string
	Password string
	Timeout  time.Duration
}

type NativeClient struct {
	ssh  *ssh.Client
	sftp *sftp.Client
}

func NewNativeClient(ctx context.Context, opts NativeOptions) (*NativeClient, error) {
	if opts.SSHDir != "" {
		if err := os.MkdirAll(opts.SSHDir, 0o700); err != nil {
			return nil, fmt.Errorf("create ssh dir: %w", err)
		}
	}
	cfg, err := buildSSHClientConfig(opts)
	if err != nil {
		return nil, err
	}

	type dialResult struct {
		client *ssh.Client
		err    error
	}
	done := make(chan dialResult, 1)
	go func() {
		client, err := ssh.Dial("tcp", nativeAddress(opts), cfg)
		done <- dialResult{client: client, err: err}
	}()

	var sshClient *ssh.Client
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-done:
		if result.err != nil {
			return nil, fmt.Errorf("ssh dial %s: %w", nativeAddress(opts), result.err)
		}
		sshClient = result.client
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("open sftp client: %w", err)
	}
	return &NativeClient{ssh: sshClient, sftp: sftpClient}, nil
}

func buildSSHClientConfig(opts NativeOptions) (*ssh.ClientConfig, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	callback, err := hostKeyCallback(opts.SSHDir)
	if err != nil {
		return nil, err
	}
	return &ssh.ClientConfig{
		User:            opts.User,
		Auth:            []ssh.AuthMethod{ssh.Password(opts.Password)},
		HostKeyCallback: callback,
		Timeout:         timeout,
	}, nil
}

func hostKeyCallback(sshDir string) (ssh.HostKeyCallback, error) {
	if sshDir == "" {
		return nil, errors.New("ssh dir is required")
	}
	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return nil, fmt.Errorf("create ssh dir: %w", err)
	}
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if callback != nil {
			err := callback(hostname, remote, key)
			if err == nil {
				return nil
			}
			var keyErr *knownhosts.KeyError
			if !errors.As(err, &keyErr) || len(keyErr.Want) > 0 {
				return err
			}
		}
		line := knownhosts.Line([]string{hostname}, key)
		file, err := os.OpenFile(knownHostsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("open known_hosts: %w", err)
		}
		if _, err := fmt.Fprintln(file, line); err != nil {
			_ = file.Close()
			return fmt.Errorf("write known_hosts: %w", err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close known_hosts: %w", err)
		}
		callback, err = knownhosts.New(knownHostsPath)
		if err != nil {
			return fmt.Errorf("reload known_hosts: %w", err)
		}
		return nil
	}, nil
}

func nativeAddress(opts NativeOptions) string {
	port := opts.Port
	if port == 0 {
		port = 22
	}
	return fmt.Sprintf("%s:%d", opts.Host, port)
}

func (c *NativeClient) Run(ctx context.Context, command string) error {
	session, err := c.ssh.NewSession()
	if err != nil {
		return fmt.Errorf("open ssh session: %w", err)
	}
	defer func() {
		_ = session.Close()
	}()

	type runResult struct {
		output []byte
		err    error
	}
	done := make(chan runResult, 1)
	go func() {
		output, err := session.CombinedOutput(command)
		done <- runResult{output: output, err: err}
	}()

	select {
	case <-ctx.Done():
		_ = session.Close()
		return ctx.Err()
	case result := <-done:
		if result.err != nil {
			return fmt.Errorf("remote command failed: %s: %w: %s", command, result.err, string(result.output))
		}
		return nil
	}
}

func (c *NativeClient) MkdirAll(_ context.Context, remotePath string) error {
	if err := c.sftp.MkdirAll(remotePath); err != nil {
		return fmt.Errorf("mkdir remote path %s: %w", remotePath, err)
	}
	return nil
}

func (c *NativeClient) UploadFile(_ context.Context, localPath, remotePath string, mode os.FileMode) error {
	local, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file %s: %w", localPath, err)
	}
	defer func() {
		_ = local.Close()
	}()
	if err := c.sftp.MkdirAll(path.Dir(remotePath)); err != nil {
		return fmt.Errorf("mkdir remote file dir %s: %w", path.Dir(remotePath), err)
	}
	remote, err := c.sftp.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote file %s: %w", remotePath, err)
	}
	defer func() {
		_ = remote.Close()
	}()
	if _, err := io.Copy(remote, local); err != nil {
		return fmt.Errorf("copy file to %s: %w", remotePath, err)
	}
	if err := c.sftp.Chmod(remotePath, mode); err != nil {
		return fmt.Errorf("chmod remote file %s: %w", remotePath, err)
	}
	return nil
}

func (c *NativeClient) Close() error {
	var err error
	if c.sftp != nil {
		err = c.sftp.Close()
	}
	if c.ssh != nil {
		if closeErr := c.ssh.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}
