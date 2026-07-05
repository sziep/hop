package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type bookmark struct {
	Label    string `json:"label"`
	Path     string `json:"path"`
	UseCount int    `json:"use_count,omitempty"`
	LastUsed int64  `json:"last_used,omitempty"`
}

type store struct {
	path      string
	bookmarks []*bookmark
}

var homeDir, _ = os.UserHomeDir()

// displayPath abbreviates the home directory to ~ for rendering.
func displayPath(p string) string {
	if homeDir != "" && (p == homeDir || strings.HasPrefix(p, homeDir+string(filepath.Separator))) {
		return "~" + p[len(homeDir):]
	}
	return p
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
	if b := s.byLabel(label); b != nil {
		b.Path = path
		return
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

func (s *store) byLabel(label string) *bookmark {
	for _, b := range s.bookmarks {
		if b.Label == label {
			return b
		}
	}
	return nil
}

func (s *store) byPath(path string) *bookmark {
	for _, b := range s.bookmarks {
		if b.Path == path {
			return b
		}
	}
	return nil
}

func (s *store) indexOf(label string) int {
	for i, b := range s.bookmarks {
		if b.Label == label {
			return i
		}
	}
	return -1
}

func (s *store) insertAt(i int, b *bookmark) {
	if i < 0 {
		i = 0
	}
	if i > len(s.bookmarks) {
		i = len(s.bookmarks)
	}
	s.bookmarks = append(s.bookmarks[:i], append([]*bookmark{b}, s.bookmarks[i:]...)...)
}

func (s *store) rename(from, to string) bool {
	b := s.byLabel(from)
	if b == nil {
		return false
	}
	b.Label = normalizeLabel(to)
	return true
}

// touch records a successful jump for frecency ranking.
func (s *store) touch(label string) {
	if b := s.byLabel(label); b != nil {
		b.UseCount++
		b.LastUsed = time.Now().Unix()
	}
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

// frecencyScore weighs use count by how recently the bookmark was used,
// zoxide-style: hits within the hour count 4x, within a day 2x, within a
// week 1x, older 0.25x.
func frecencyScore(b *bookmark, now int64) float64 {
	if b.UseCount == 0 {
		return 0
	}
	age := now - b.LastUsed
	switch {
	case age < 3600:
		return float64(b.UseCount) * 4
	case age < 86400:
		return float64(b.UseCount) * 2
	case age < 7*86400:
		return float64(b.UseCount)
	default:
		return float64(b.UseCount) * 0.25
	}
}
