package configstore

// Context: This file belongs to the MetricsNode application layer around store.

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Store struct {
	mu   sync.RWMutex
	path string
	log  *slog.Logger
	data map[string]string
}

func New(path string, defaults map[string]string, log *slog.Logger) (*Store, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return nil, errors.New("path is required")
	}
	if log == nil {
		log = slog.Default()
	}
	s := &Store{
		path: path,
		log:  log,
		data: make(map[string]string),
	}
	for k, v := range defaults {
		s.data[k] = v
	}
	_ = s.load()
	if err := s.save(); err != nil {
		// Keep it non-fatal: in-memory config still works.
		s.log.Warn("config save failed", "err", err.Error())
	}
	return s, nil
}

func (s *Store) Keys() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	s.mu.RUnlock()
	sort.Strings(keys)
	return keys
}

func (s *Store) Get(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}
	s.mu.RLock()
	val, ok := s.data[key]
	s.mu.RUnlock()
	return val, ok
}

func (s *Store) Set(key, val string) error {
	if s == nil {
		return errors.New("config store not initialized")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("key is required")
	}
	s.mu.Lock()
	if s.data == nil {
		s.data = make(map[string]string)
	}
	s.data[key] = val
	s.mu.Unlock()
	return s.save()
}

func (s *Store) load() error {
	if s == nil {
		return errors.New("config store not initialized")
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	s.mu.Lock()
	for k, v := range m {
		s.data[k] = v
	}
	s.mu.Unlock()
	return nil
}

func (s *Store) save() error {
	if s == nil {
		return errors.New("config store not initialized")
	}
	dir := filepath.Dir(s.path)
	if dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}

	s.mu.RLock()
	snapshot := make(map[string]string, len(s.data))
	for k, v := range s.data {
		snapshot[k] = v
	}
	s.mu.RUnlock()

	raw, _ := json.MarshalIndent(snapshot, "", "  ")
	return writeFileAtomic(s.path, raw, 0o600)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return errors.New("path is required")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err == nil {
		return nil
	}
	_ = os.Remove(path)
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
