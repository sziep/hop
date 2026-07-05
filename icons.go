package main

import (
	"path/filepath"
	"strings"
)

// Nerd Font glyphs. Rendered only when nerd_font = true in the config.
const (
	iconDirDefault  = "" // nf-fa-folder
	iconFileDefault = "" // nf-fa-file
	iconGitBranch   = "" // nf-dev-git_branch
)

// builtinDirIcons maps directory basenames (lowercase) to glyphs,
// VS Code-style. Anything not listed gets the generic folder.
var builtinDirIcons = map[string]string{
	".git":         "", // nf-custom-folder_git
	".github":      "", // nf-custom-folder_github
	".config":      "", // nf-custom-folder_config
	"config":       "",
	"conf":         "",
	".vscode":      "", // nf-dev-visualstudio
	"node_modules": "", // nf-custom-folder_npm
	"src":          "", // nf-fa-code
	"test":         "", // nf-fa-flask
	"tests":        "",
	"__tests__":    "",
	"spec":         "",
	"docs":         "", // nf-fa-book
	"doc":          "",
	"bin":          "", // nf-oct-terminal
	"scripts":      "",
	"build":        "", // nf-oct-package
	"dist":         "",
	"out":          "",
	"target":       "",
	"lib":          "", // nf-fa-archive
	"vendor":       "",
	"third_party":  "",
	"assets":       "", // nf-fa-file_image_o
	"images":       "",
	"img":          "",
	"icons":        "",
	"media":        "",
	"downloads":    "", // nf-fa-download
	"desktop":      "", // nf-fa-desktop
	"music":        "", // nf-fa-music
	"videos":       "", // nf-fa-film
	"movies":       "",
	"pictures":     "", // nf-fa-picture_o
	"photos":       "",
}

// builtinFileIcons maps full filenames and extensions (both lowercase,
// extensions without the dot) to glyphs. Full filenames win over extensions.
var builtinFileIcons = map[string]string{
	// filenames
	"dockerfile":          "", // nf-linux-docker
	"docker-compose.yml":  "",
	"docker-compose.yaml": "",
	".dockerignore":       "",
	"makefile":            "", // nf-dev-gnu
	"license":             "", // nf-fa-gavel
	"license.md":          "",
	"license.txt":         "",
	"copying":             "",
	"readme":              "", // nf-fa-book
	"readme.md":           "",
	"readme.txt":          "",
	"go.mod":              "",
	"go.sum":              "",
	".gitignore":          "", // nf-fa-git
	".gitattributes":      "",
	".gitmodules":         "",
	"package.json":        "", // nf-dev-npm
	"package-lock.json":   "",
	"cargo.toml":          "",
	"cargo.lock":          "",
	".env":                "", // nf-fa-cog

	// languages
	"go":    "", // nf-seti-go
	"py":    "", // nf-seti-python
	"js":    "", // nf-dev-javascript_badge
	"mjs":   "",
	"cjs":   "",
	"ts":    "", // nf-seti-typescript
	"tsx":   "", // nf-dev-react
	"jsx":   "",
	"rb":    "", // nf-dev-ruby
	"rs":    "", // nf-dev-rust
	"java":  "", // nf-dev-java
	"c":     "", // nf-seti-c
	"h":     "",
	"cpp":   "", // nf-seti-cpp
	"cc":    "",
	"hpp":   "",
	"php":   "", // nf-dev-php
	"swift": "", // nf-dev-swift
	"kt":    "", // nf-seti-kotlin
	"lua":   "", // nf-seti-lua
	"vim":   "", // nf-custom-vim

	// web / markup / config
	"html": "", // nf-dev-html5
	"htm":  "",
	"css":  "", // nf-dev-css3
	"scss": "",
	"sass": "",
	"less": "",
	"md":   "", // nf-dev-markdown
	"json": "", // nf-seti-json
	"yaml": "", // nf-seti-config
	"yml":  "",
	"toml": "",
	"ini":  "",
	"conf": "",
	"env":  "",

	// shells / data
	"sh":     "", // nf-oct-terminal
	"bash":   "",
	"zsh":    "",
	"fish":   "",
	"sql":    "", // nf-dev-database
	"db":     "",
	"sqlite": "",
	"lock":   "", // nf-fa-lock

	// documents / media / archives
	"txt":  "", // nf-fa-file_text
	"log":  "", // nf-fa-file_text_o
	"pdf":  "",
	"csv":  "",
	"xls":  "",
	"xlsx": "",
	"doc":  "",
	"docx": "",
	"ppt":  "",
	"pptx": "",
	"png":  "",
	"jpg":  "",
	"jpeg": "",
	"gif":  "",
	"svg":  "",
	"ico":  "",
	"webp": "",
	"bmp":  "",
	"mp3":  "",
	"wav":  "",
	"flac": "",
	"ogg":  "",
	"mp4":  "",
	"mov":  "",
	"mkv":  "",
	"avi":  "",
	"webm": "",
	"zip":  "",
	"tar":  "",
	"gz":   "",
	"tgz":  "",
	"bz2":  "",
	"xz":   "",
	"7z":   "",
	"rar":  "",
}

// iconSet is the merged builtin + user icon mapping used by the TUI.
type iconSet struct {
	dirs  map[string]string
	files map[string]string
}

func newIconSet(cfg hopConfig) iconSet {
	is := iconSet{
		dirs:  make(map[string]string, len(builtinDirIcons)+len(cfg.Icons.Dirs)),
		files: make(map[string]string, len(builtinFileIcons)+len(cfg.Icons.Files)),
	}
	for k, v := range builtinDirIcons {
		is.dirs[k] = v
	}
	for k, v := range builtinFileIcons {
		is.files[k] = v
	}
	// user overrides win; keys are matched lowercase
	for k, v := range cfg.Icons.Dirs {
		is.dirs[strings.ToLower(k)] = v
	}
	for k, v := range cfg.Icons.Files {
		is.files[strings.ToLower(k)] = v
	}
	return is
}

// dir returns the glyph (plus separator space) for a directory basename.
func (is iconSet) dir(name string) string {
	if g, ok := is.dirs[strings.ToLower(name)]; ok {
		return g + " "
	}
	return iconDirDefault + " "
}

// file returns the glyph for a filename: full name match first, then
// extension, then the generic file icon.
func (is iconSet) file(name string) string {
	n := strings.ToLower(name)
	if g, ok := is.files[n]; ok {
		return g + " "
	}
	if ext := strings.TrimPrefix(filepath.Ext(n), "."); ext != "" {
		if g, ok := is.files[ext]; ok {
			return g + " "
		}
	}
	return iconFileDefault + " "
}
