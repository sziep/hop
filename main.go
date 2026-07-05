package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-runewidth"
)

// version is stamped at build time via -ldflags "-X main.version=..."
var version = "0.2.0-dev"

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "init" {
		shell := ""
		if len(os.Args) >= 3 {
			shell = os.Args[2]
		}
		switch shell {
		case "zsh":
			fmt.Print(zshInit)
		case "bash":
			fmt.Print(bashInit)
		case "fish":
			fmt.Print(fishInit)
		default:
			fmt.Fprintln(os.Stderr, "hop: usage: hop init <zsh|bash|fish>")
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
		fmt.Println(version)
		return
	}

	var (
		addLabel   string
		delLabel   string
		listFlag   bool
		labelsFlag bool
		themesFlag bool
		jumpFlag   bool
	)
	flag.StringVar(&addLabel, "a", "", "bookmark CWD (or a given path) with `label`")
	flag.StringVar(&delLabel, "d", "", "delete bookmark with `label`")
	flag.BoolVar(&listFlag, "l", false, "list bookmarks")
	flag.BoolVar(&labelsFlag, "labels", false, "print bookmark labels (used by shell completion)")
	flag.BoolVar(&themesFlag, "themes", false, "list built-in themes")
	flag.BoolVar(&jumpFlag, "jump", false, "resolve a query to a bookmark path (used by shell function)")
	flag.Bool("pick", false, "open TUI picker (used by shell function)")
	flag.Parse()

	if themesFlag {
		for _, n := range themeNames() {
			fmt.Println(n)
		}
		return
	}

	s, err := loadStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hop: failed to load bookmarks:", err)
		os.Exit(1)
	}

	switch {
	case addLabel != "":
		target, err := resolveAddTarget(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, "hop:", err)
			os.Exit(1)
		}
		s.add(addLabel, target)
		if err := s.save(); err != nil {
			fmt.Fprintln(os.Stderr, "hop: failed to save:", err)
			os.Exit(1)
		}

	case delLabel != "":
		if !s.remove(delLabel) {
			fmt.Fprintf(os.Stderr, "hop: bookmark %q not found\n", delLabel)
			os.Exit(1)
		}
		if err := s.save(); err != nil {
			fmt.Fprintln(os.Stderr, "hop: failed to save:", err)
			os.Exit(1)
		}

	case labelsFlag:
		for _, b := range s.all() {
			fmt.Println(b.Label)
		}

	case listFlag:
		width := 0
		for _, b := range s.all() {
			if w := runewidth.StringWidth(b.Label); w > width {
				width = w
			}
		}
		for _, b := range s.all() {
			pad := strings.Repeat(" ", width-runewidth.StringWidth(b.Label))
			fmt.Printf("[%s]%s  %s\n", b.Label, pad, displayPath(b.Path))
		}

	case jumpFlag:
		query := strings.TrimSpace(strings.Join(flag.Args(), " "))
		if b, ok := resolveJump(s, query); ok {
			s.touch(b.Label)
			if err := s.save(); err != nil {
				fmt.Fprintln(os.Stderr, "hop: warning: failed to save:", err)
			}
			fmt.Println(b.Path)
			return
		}
		// ambiguous or no match: fall back to the picker, pre-filtered
		pickAndPrint(s, query)

	default:
		// covers both: `hop` (zero args) and `hop --pick` (called by shell function)
		pickAndPrint(s, "")
	}
}

func pickAndPrint(s *store, initialFilter string) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "/"
	}
	result := runPicker(s, cwd, loadConfig(), initialFilter)
	// always save — TUI may have added/deleted/renamed/reordered bookmarks
	if err := s.save(); err != nil {
		fmt.Fprintln(os.Stderr, "hop: warning: failed to save:", err)
	}
	if !result.aborted && result.path != "" {
		fmt.Println(result.path)
	}
}

// resolveAddTarget validates the optional path argument of -a, defaulting to CWD.
func resolveAddTarget(arg string) (string, error) {
	if arg == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot get current directory: %w", err)
		}
		return cwd, nil
	}
	abs, err := filepath.Abs(arg)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(abs)
	if err != nil || !fi.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	return abs, nil
}

// resolveJump maps a query to a bookmark: exact label match wins, otherwise a
// unique fuzzy match (against label or display path) jumps directly.
// Anything ambiguous returns !ok so the caller can open the picker.
func resolveJump(s *store, query string) (*bookmark, bool) {
	if query == "" {
		return nil, false
	}
	for _, b := range s.all() {
		if strings.EqualFold(b.Label, query) {
			return b, true
		}
	}
	pattern := []rune(strings.ToLower(query))
	var only *bookmark
	for _, b := range s.all() {
		_, labelOK := fuzzyScore(b.Label, pattern)
		_, pathOK := fuzzyScore(displayPath(b.Path), pattern)
		if labelOK || pathOK {
			if only != nil {
				return nil, false // ambiguous
			}
			only = b
		}
	}
	if only == nil {
		return nil, false
	}
	return only, true
}
