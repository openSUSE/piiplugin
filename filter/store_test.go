package filter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMapStore(t *testing.T) {
	store := NewMapStore()

	// Initial checks
	if _, found := store.Get("key1"); found {
		t.Error("Expected key1 not to be found")
	}

	// Set and Get
	store.Set("key1", "val1")
	val, found := store.Get("key1")
	if !found {
		t.Error("Expected key1 to be found")
	}
	if val != "val1" {
		t.Errorf("Expected val1, got %q", val)
	}

	// Overwrite
	store.Set("key1", "val1_updated")
	val, _ = store.Get("key1")
	if val != "val1_updated" {
		t.Errorf("Expected val1_updated, got %q", val)
	}

	// Iterate
	store.Set("key2", "val2")
	keys := make(map[string]string)
	store.Iterate(func(k, v string) bool {
		keys[k] = v
		return true
	})

	if len(keys) != 2 {
		t.Errorf("Expected 2 elements in iteration, got %d", len(keys))
	}
	if keys["key1"] != "val1_updated" || keys["key2"] != "val2" {
		t.Errorf("Iteration data mismatch: %v", keys)
	}

	// Iterate stop early
	count := 0
	store.Iterate(func(k, v string) bool {
		count++
		return false // stop immediately
	})
	if count != 1 {
		t.Errorf("Expected iteration to stop early at 1, got %d", count)
	}
}

func TestFileStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filestore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "store.json")

	// Create and write
	store, err := NewFileStore(filePath)
	if err != nil {
		t.Fatalf("Failed to create FileStore: %v", err)
	}

	store.Set("key1", "val1")
	store.Set("key2", "val2")

	// Read back from the same instance
	val, found := store.Get("key1")
	if !found || val != "val1" {
		t.Errorf("Expected key1: val1, got %q (found: %t)", val, found)
	}

	// Load from a new instance (persisted)
	newStore, err := NewFileStore(filePath)
	if err != nil {
		t.Fatalf("Failed to load existing FileStore: %v", err)
	}

	val, found = newStore.Get("key1")
	if !found || val != "val1" {
		t.Errorf("Expected persistent key1: val1, got %q (found: %t)", val, found)
	}

	val, found = newStore.Get("key2")
	if !found || val != "val2" {
		t.Errorf("Expected persistent key2: val2, got %q (found: %t)", val, found)
	}
}
