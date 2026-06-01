package organizer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireLockSuccess(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")

	release, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("acquireLock failed: %v", err)
	}
	if release == nil {
		t.Fatal("release function is nil")
	}

	// Lock file should exist
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Fatal("lock file was not created")
	}

	// Release lock
	release()

	// Lock file should be removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("lock file was not removed after release")
	}
}

func TestAcquireLockExclusive(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock2")

	release1, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("first acquireLock failed: %v", err)
	}
	defer release1()

	// Second attempt should fail
	_, err = acquireLock(lockPath)
	if err == nil {
		t.Fatal("expected error for second lock attempt")
	}
}

func TestIsProcessAlive(t *testing.T) {
	// Current process should be alive
	if !isProcessAlive(os.Getpid()) {
		t.Error("current process should be alive")
	}

	// PID 0 or negative should be handled
	if isProcessAlive(0) {
		t.Error("PID 0 should not be alive")
	}

	if isProcessAlive(-1) {
		t.Error("PID -1 should not be alive")
	}

	// Very high PID likely doesn't exist
	if isProcessAlive(999999999) {
		t.Log("PID 999999999 reported alive (unlikely but possible on some systems)")
	}
}
