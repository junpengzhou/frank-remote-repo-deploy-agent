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
	if !store.Changed("example-common", "test", "abc") {
		t.Fatal("expected missing entry to be changed")
	}
	store.Update("example-common", "test", "abc")
	if store.Changed("example-common", "test", "abc") {
		t.Fatal("expected same commit to be cache hit")
	}
	if !store.Changed("example-common", "test", "def") {
		t.Fatal("expected new commit to be changed")
	}
}

func TestSaveAndLoadPersistsEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	store, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	store.Update("example-common", "test", "abc")
	store.UpdateSnapshot("example-app", "test", map[string]string{
		"example-common": "abc",
		"example-app":    "main1",
	})
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Changed("example-common", "test", "abc") {
		t.Fatal("expected persisted commit to be cache hit")
	}
	if loaded.SnapshotChanged("example-app", "test", map[string]string{
		"example-common": "abc",
		"example-app":    "main1",
	}) {
		t.Fatal("expected persisted snapshot to be cache hit")
	}
}

func TestSnapshotChangedDetectsMissHitAndChangedDependency(t *testing.T) {
	store, err := Load(filepath.Join(t.TempDir(), "cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := map[string]string{
		"example-common": "abc",
		"example-app":    "main1",
	}
	if !store.SnapshotChanged("example-app", "test", snapshot) {
		t.Fatal("expected missing snapshot to be changed")
	}
	store.UpdateSnapshot("example-app", "test", snapshot)
	if store.SnapshotChanged("example-app", "test", snapshot) {
		t.Fatal("expected same snapshot to be cache hit")
	}
	if !store.SnapshotChanged("example-app", "test", map[string]string{
		"example-common": "def",
		"example-app":    "main1",
	}) {
		t.Fatal("expected changed dependency snapshot to be changed")
	}
}
