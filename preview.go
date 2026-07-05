package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type previewData struct {
	path    string
	branch  string
	dirs    []string
	files   []string
	missing bool
	failed  bool
}

type previewMsg previewData

type dirEntriesMsg struct {
	path    string
	entries []string
}

type deadMsg map[string]bool

// readDirSplit lists path's entries split into directories and files,
// each sorted, honouring the hidden-file toggle.
func readDirSplit(path string, showHidden bool) (dirs, files []string, err error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}
	slices.Sort(dirs)
	slices.Sort(files)
	return dirs, files, nil
}

func fetchPreview(path string, showHidden bool) tea.Cmd {
	return func() tea.Msg {
		d := previewData{path: path}
		dirs, files, err := readDirSplit(path, showHidden)
		if err != nil {
			d.missing = os.IsNotExist(err)
			d.failed = true
			return previewMsg(d)
		}
		d.dirs = dirs
		d.files = files
		d.branch = gitBranch(path)
		return previewMsg(d)
	}
}

func loadDirEntries(path string, showHidden bool) tea.Cmd {
	return func() tea.Msg {
		dirs, _, _ := readDirSplit(path, showHidden)
		return dirEntriesMsg{path: path, entries: dirs}
	}
}

func checkDead(paths []string) tea.Cmd {
	return func() tea.Msg {
		dead := make(map[string]bool)
		for _, p := range paths {
			if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
				dead[p] = true
			}
		}
		return deadMsg(dead)
	}
}

// gitBranch reads .git/HEAD directly (no git binary needed) and returns the
// current branch name, a short hash when detached, or "" if dir is no repo.
func gitBranch(dir string) string {
	gitPath := filepath.Join(dir, ".git")
	fi, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}
	gitDir := gitPath
	if !fi.IsDir() {
		// worktree / submodule: .git is a file pointing at the real git dir
		data, err := os.ReadFile(gitPath)
		if err != nil {
			return ""
		}
		line := strings.TrimSpace(string(data))
		target, ok := strings.CutPrefix(line, "gitdir: ")
		if !ok {
			return ""
		}
		gitDir = target
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(dir, gitDir)
		}
	}
	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(head))
	if branch, ok := strings.CutPrefix(s, "ref: refs/heads/"); ok {
		return branch
	}
	if len(s) >= 7 {
		return s[:7] // detached HEAD
	}
	return ""
}
