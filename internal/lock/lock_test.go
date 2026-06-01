package lock

import (
	"testing"
)

func TestModuleLeaseIsSupersededByNewerAcquire(t *testing.T) {
	manager := NewManager(t.TempDir())
	first, err := manager.AcquireModule("ifintech-frank")
	if err != nil {
		t.Fatal(err)
	}
	if first.Superseded() {
		t.Fatal("fresh lease should not be superseded")
	}
	_, err = manager.AcquireModule("ifintech-frank")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Superseded() {
		t.Fatal("older lease should be superseded")
	}
}
