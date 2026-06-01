package cache

import (
	"path/filepath"
	"testing"
)

func TestChangedDetectsMissHitAndNewCommit(t *testing.T) {
	store, err := Load(filepath.Join(t.TempDir(), "cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !store.Changed("ifintech-common", "test", "abc") {
		t.Fatal("expected missing entry to be changed")
	}
	store.Update("ifintech-common", "test", "abc")
	if store.Changed("ifintech-common", "test", "abc") {
		t.Fatal("expected same commit to be cache hit")
	}
	if !store.Changed("ifintech-common", "test", "def") {
		t.Fatal("expected new commit to be changed")
	}
}

func TestSaveAndLoadPersistsEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	store, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	store.Update("ifintech-common", "test", "abc")
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Changed("ifintech-common", "test", "abc") {
		t.Fatal("expected persisted commit to be cache hit")
	}
}
