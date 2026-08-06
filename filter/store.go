package filter

import (
	"encoding/json"
	"io"
	"os"
	"sync"
)

// Store defines an interface for managing PII replacement mappings.
// The key is the generated replacement string, and the value is the original string.
// Implementing this interface allows storing mappings in memory, on disk, or
// in external services (such as Redis or Memcached).
type Store interface {
	// Get retrieves the original value for a given replacement key.
	Get(replacement string) (original string, found bool)

	// Set saves a replacement mapping.
	Set(replacement string, original string)

	// Iterate executes a callback for each replacement-original pair in the store.
	// If the callback returns false, iteration stops.
	Iterate(fn func(replacement, original string) bool)
}

// MapStore is an in-memory implementation of Store, protected by a mutex
// for thread safety.
type MapStore struct {
	mu   sync.RWMutex
	data map[string]string
}

// NewMapStore creates a new thread-safe MapStore.
func NewMapStore() *MapStore {
	return &MapStore{
		data: make(map[string]string),
	}
}

// Get retrieves the original value for a given replacement key.
func (s *MapStore) Get(replacement string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	orig, found := s.data[replacement]
	return orig, found
}

// Set saves a replacement mapping.
func (s *MapStore) Set(replacement string, original string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[replacement] = original
}

// Iterate executes a callback for each replacement-original pair in the store.
func (s *MapStore) Iterate(fn func(replacement, original string) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, v := range s.data {
		if !fn(k, v) {
			break
		}
	}
}

// FileStore is an on-disk, thread-safe implementation of Store that persists
// mappings to a JSON file.
type FileStore struct {
	mu       sync.RWMutex
	filePath string
	data     map[string]string
}

// NewFileStore creates a new FileStore at the specified path. It attempts to
// load any existing mappings from that file.
func NewFileStore(filePath string) (*FileStore, error) {
	fs := &FileStore{
		filePath: filePath,
		data:     make(map[string]string),
	}

	if err := fs.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return fs, nil
}

// Get retrieves the original value for a given replacement key.
func (s *FileStore) Get(replacement string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	orig, found := s.data[replacement]
	return orig, found
}

// Set saves a replacement mapping and persists it to disk.
func (s *FileStore) Set(replacement string, original string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[replacement] = original
	_ = s.save() // ignore write errors for robustness, or handle appropriately
}

// Iterate executes a callback for each replacement-original pair in the store.
func (s *FileStore) Iterate(fn func(replacement, original string) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, v := range s.data {
		if !fn(k, v) {
			break
		}
	}
}

func (s *FileStore) load() error {
	file, err := os.Open(s.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&s.data); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func (s *FileStore) save() error {
	file, err := os.Create(s.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(s.data)
}
