package main

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

//go:embed themes/*.toml
var themesFS embed.FS

type colorConfig struct {
	SelectedBg string `toml:"selected_bg"`
	SelectedFg string `toml:"selected_fg"`
	Label      string `toml:"label"`
	Path       string `toml:"path"`
	Filter     string `toml:"filter"`
	FilterDim  string `toml:"filter_dim"`
	Border     string `toml:"border"`
	Preview    string `toml:"preview"`
	PreviewDim string `toml:"preview_dim"`
	Header     string `toml:"header"`
	Hint       string `toml:"hint"`
	Status     string `toml:"status"`
	Match      string `toml:"match"`
	Dead       string `toml:"dead"`
	Git        string `toml:"git"`
}

// iconOverrides lets users extend or replace the builtin icon maps, VS Code
// style: [icons.dirs] keys are directory basenames, [icons.files] keys are
// full filenames or extensions (without the dot).
type iconOverrides struct {
	Dirs  map[string]string `toml:"dirs"`
	Files map[string]string `toml:"files"`
}

type hopConfig struct {
	Theme    string        `toml:"theme"`
	NerdFont bool          `toml:"nerd_font"`
	Colors   colorConfig   `toml:"colors"`
	Icons    iconOverrides `toml:"icons"`
}

func themeNames() []string {
	entries, _ := themesFS.ReadDir("themes")
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".toml"))
	}
	sort.Strings(names)
	return names
}

func loadThemeColors(name string) (colorConfig, error) {
	data, err := themesFS.ReadFile("themes/" + name + ".toml")
	if err != nil {
		return colorConfig{}, fmt.Errorf("unknown theme %q (available: %s)", name, strings.Join(themeNames(), ", "))
	}
	var t struct {
		Colors colorConfig `toml:"colors"`
	}
	if err := toml.Unmarshal(data, &t); err != nil {
		return colorConfig{}, fmt.Errorf("theme %q: %w", name, err)
	}
	return t.Colors, nil
}

// loadConfig reads ~/.config/hop/config.toml. Empty color values mean "unset"
// and fall back to the selected theme in resolveColors.
func loadConfig() hopConfig {
	var cfg hopConfig
	dir, err := configDir()
	if err != nil {
		return cfg
	}
	path := filepath.Join(dir, "config.toml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hop: warning: could not read config:", err)
		return cfg
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintln(os.Stderr, "hop: warning: could not parse config:", err)
		return hopConfig{}
	}
	return cfg
}

// resolveColors layers user overrides on top of an embedded theme. With no
// explicit theme configured, dark terminals get "default", light ones "light".
func resolveColors(cfg hopConfig, darkBackground bool) colorConfig {
	name := cfg.Theme
	if name == "" {
		if darkBackground {
			name = "default"
		} else {
			name = "light"
		}
	}
	base, err := loadThemeColors(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hop: warning:", err)
		base, _ = loadThemeColors("default")
	}

	pick := func(over string, dst *string) {
		if over != "" {
			*dst = over
		}
	}
	o := cfg.Colors
	pick(o.SelectedBg, &base.SelectedBg)
	pick(o.SelectedFg, &base.SelectedFg)
	pick(o.Label, &base.Label)
	pick(o.Path, &base.Path)
	pick(o.Filter, &base.Filter)
	pick(o.FilterDim, &base.FilterDim)
	pick(o.Border, &base.Border)
	pick(o.Preview, &base.Preview)
	pick(o.PreviewDim, &base.PreviewDim)
	pick(o.Header, &base.Header)
	pick(o.Hint, &base.Hint)
	pick(o.Status, &base.Status)
	pick(o.Match, &base.Match)
	pick(o.Dead, &base.Dead)
	pick(o.Git, &base.Git)
	return base
}
