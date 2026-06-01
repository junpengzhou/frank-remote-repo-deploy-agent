package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Entry struct {
	Module string `json:"module"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

type Store struct {
	path    string
	mu      sync.Mutex
	Entries map[string]Entry `json:"entries"`
}

func Load(path string) (*Store, error) {
	store := &Store{path: path, Entries: map[string]Entry{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return store, nil
	}
	if err := json.Unmarshal(data, store); err != nil {
		return nil, err
	}
	if store.Entries == nil {
		store.Entries = map[string]Entry{}
	}
	return store, nil
}

func (s *Store) Changed(module, branch, commit string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.Entries[key(module, branch)]
	return !ok || entry.Commit != commit
}

func (s *Store) Update(module, branch, commit string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Entries[key(module, branch)] = Entry{Module: module, Branch: branch, Commit: commit}
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func key(module, branch string) string {
	return module + "@" + branch
}
