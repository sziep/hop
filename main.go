package main

import (
	"flag"
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "init" && os.Args[2] == "zsh" {
		fmt.Print(zshInit)
		return
	}
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
		fmt.Println(version)
		return
	}

	var addLabel string
	var delLabel string
	var listFlag bool
	// --pick is used internally by the zsh shell function
	flag.StringVar(&addLabel, "a", "", "bookmark CWD with `label`")
	flag.StringVar(&delLabel, "d", "", "delete bookmark with `label`")
	flag.BoolVar(&listFlag, "l", false, "list bookmarks")
	flag.Bool("pick", false, "open TUI picker (used by shell function)")
	flag.Parse()

	s, err := loadStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hop: failed to load bookmarks:", err)
		os.Exit(1)
	}

	switch {
	case addLabel != "":
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "hop: cannot get current directory:", err)
			os.Exit(1)
		}
		s.add(addLabel, cwd)
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

	case listFlag:
		for _, b := range s.all() {
			fmt.Printf("[%-12s]  %s\n", b.Label, b.Path)
		}

	default:
		// covers both: `hop` (zero args) and `hop --pick` (called by shell function)
		if len(s.all()) == 0 {
			fmt.Fprintln(os.Stderr, "hop: no bookmarks — add one with: hop -a <label>")
			os.Exit(1)
		}
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "/"
		}
		result := runPicker(s, cwd, loadConfig())
		// always save — TUI may have added/deleted/reordered bookmarks
		if err := s.save(); err != nil {
			fmt.Fprintln(os.Stderr, "hop: warning: failed to save:", err)
		}
		if !result.aborted {
			fmt.Println(result.path)
		}
	}
}
