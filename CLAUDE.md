# hop — CLAUDE.md

## What this is
A terminal bookmark navigator written in Go. Lets you jump between frequently-used directories via a TUI picker. Single binary, no daemon.

## Build
Requires Docker (no local Go needed):
```bash
make build   # produces bin/hop (darwin/arm64)
```

## Key files
| File | Purpose |
|------|---------|
| `main.go` | CLI entry point — flags (-a, -d, -l), TUI launch, save |
| `store.go` | Bookmark persistence — JSON at `~/.config/hop/bookmarks.json`, atomic save |
| `config.go` | Color config — loads `~/.config/hop/config.toml`, falls back to defaults |
| `tui.go` | Bubbletea TUI — bookmarks view + filesystem browser, all key handling |
| `init.go` | zsh shell integration script (printed by `hop init zsh`) |

## Architecture
Two views share one `model` struct:
- **Bookmarks view** (`modeBookmarks`) — fuzzy-filterable list of saved bookmarks
- **Browser view** (`modeBrowser`) — live filesystem directory browser, entered via `→`

Bookmarks are stored in natural insertion order. The TUI exposes `Shift+↑/↓` to reorder manually. No auto-sorting.

## Key behaviours to preserve
- `Ctrl+A` in browser adds the **cursor-selected** directory, not the parent
- `Ctrl+D` (delete) only works in bookmarks view, not browser
- `Shift+↑/↓` reorder is disabled when a fuzzy filter is active
- `store.move(i, delta)` operates on raw store indices — only valid when `m.filter == ""`
- Atomic save: write to `.tmp` then `os.Rename` (prevents corruption on crash)
