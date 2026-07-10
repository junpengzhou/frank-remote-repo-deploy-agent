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

type Snapshot struct {
	MainModule string            `json:"mainModule"`
	Branch     string            `json:"branch"`
	Commits    map[string]string `json:"commits"`
}

type Store struct {
	path string
	mu   sync.Mutex
	// key format: module@branch
	Entries map[string]Entry `json:"entries"`
	// key format: mainModule@branch
	Snapshots map[string]Snapshot `json:"snapshots,omitempty"`
}

func Load(path string) (*Store, error) {
	store := &Store{
		path:      path,
		Entries:   map[string]Entry{},
		Snapshots: map[string]Snapshot{},
	}
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
	if store.Snapshots == nil {
		store.Snapshots = map[string]Snapshot{}
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

func (s *Store) SnapshotChanged(mainModule, branch string, commits map[string]string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, ok := s.Snapshots[key(mainModule, branch)]
	if !ok {
		return true
	}
	if len(snapshot.Commits) != len(commits) {
		return true
	}
	for module, commit := range commits {
		if snapshot.Commits[module] != commit {
			return true
		}
	}
	return false
}

func (s *Store) UpdateSnapshot(mainModule, branch string, commits map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := make(map[string]string, len(commits))
	for module, commit := range commits {
		cloned[module] = commit
	}
	s.Snapshots[key(mainModule, branch)] = Snapshot{
		MainModule: mainModule,
		Branch:     branch,
		Commits:    cloned,
	}
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
