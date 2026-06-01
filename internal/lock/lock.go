package lock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ModuleLease struct {
	path  string
	Token string
}

type Manager struct {
	dir string
}

func NewManager(dir string) *Manager {
	return &Manager{dir: dir}
}

func (m *Manager) AcquireModule(module string) (*ModuleLease, error) {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return nil, err
	}
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	path := filepath.Join(m.dir, sanitize(module)+".generation")
	// 同模块抢占的核心：新部署直接覆盖 generation，旧部署下一次 Check 时会退出。
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		return nil, err
	}
	return &ModuleLease{path: path, Token: token}, nil
}

func (l *ModuleLease) Superseded() bool {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return true
	}
	return strings.TrimSpace(string(data)) != l.Token
}

func (l *ModuleLease) Check() error {
	if l.Superseded() {
		return errors.New("deployment was superseded by a newer run")
	}
	return nil
}

type FileLock struct {
	path string
}

func (m *Manager) AcquireExclusive(ctx context.Context, name string, retry time.Duration) (*FileLock, error) {
	if retry <= 0 {
		retry = 200 * time.Millisecond
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(m.dir, sanitize(name)+".lock")
	for {
		// mkdir 在主流文件系统上具备原子性，适合作为跨进程互斥锁。
		err := os.Mkdir(path, 0o755)
		if err == nil {
			return &FileLock{path: path}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retry):
		}
	}
}

func (l *FileLock) Release() error {
	return os.Remove(l.path)
}

func sanitize(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return replacer.Replace(name)
}
