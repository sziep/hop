package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type bookmark struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

type store struct {
	path      string
	bookmarks []*bookmark
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "hop")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func loadStore() (*store, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "bookmarks.json")
	s := &store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &s.bookmarks); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *store) save() error {
	data, err := json.MarshalIndent(s.bookmarks, "", "  ")
	if err != nil {
		return err
	}
	// write temp to same dir to guarantee atomic rename (no EXDEV)
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func normalizeLabel(label string) string {
	return strings.TrimSpace(label)
}

func (s *store) add(label, path string) {
	label = normalizeLabel(label)
	for _, b := range s.bookmarks {
		if b.Label == label {
			b.Path = path
			return
		}
	}
	s.bookmarks = append(s.bookmarks, &bookmark{Label: label, Path: path})
}

func (s *store) remove(label string) bool {
	label = normalizeLabel(label)
	for i, b := range s.bookmarks {
		if b.Label == label {
			s.bookmarks = append(s.bookmarks[:i], s.bookmarks[i+1:]...)
			return true
		}
	}
	return false
}

func (s *store) all() []*bookmark {
	out := make([]*bookmark, len(s.bookmarks))
	copy(out, s.bookmarks)
	return out
}

func (s *store) move(i, delta int) {
	j := i + delta
	if j < 0 || j >= len(s.bookmarks) {
		return
	}
	s.bookmarks[i], s.bookmarks[j] = s.bookmarks[j], s.bookmarks[i]
}

