package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type colorConfig struct {
	SelectedBg string `toml:"selected_bg"`
	SelectedFg string `toml:"selected_fg"`
	Label      string `toml:"label"`
	Path       string `toml:"path"`
	Filter     string `toml:"filter"`
	FilterDim  string `toml:"filter_dim"`
	Border     string `toml:"border"`
	Preview    string `toml:"preview"`
	Header     string `toml:"header"`
	Hint       string `toml:"hint"`
	Status     string `toml:"status"`
}

func defaultColors() colorConfig {
	return colorConfig{
		SelectedBg: "#316168",
		SelectedFg: "#aaedf5",
		Label:      "#66D9E8",
		Path:       "#7e7e7e",
		Filter:     "#FD971F",
		FilterDim:  "#75715E",
		Border:     "#316168",
		Preview:    "#96b0bd",
		Header:     "#FD971F",
		Hint:       "#5b6c81",
		Status:     "#A6E22E",
	}
}

type hopConfig struct {
	Colors colorConfig `toml:"colors"`
}

func loadConfig() colorConfig {
	dir, err := configDir()
	if err != nil {
		return defaultColors()
	}
	path := filepath.Join(dir, "config.toml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultColors()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hop: warning: could not read config:", err)
		return defaultColors()
	}
	cfg := hopConfig{Colors: defaultColors()}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintln(os.Stderr, "hop: warning: could not parse config:", err)
		return defaultColors()
	}
	return cfg.Colors
}
