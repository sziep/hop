# hop — CLAUDE.md

## What this is
A terminal bookmark navigator written in Go. Jump between frequently-used directories via direct query (`hop <query>`) or a TUI picker. Single binary, no daemon.

## Build
Requires Docker (no local Go needed):
```bash
make build   # bin/hop (native) + cross builds for darwin/linux × arm64/amd64
make test    # go vet + go test inside Docker
```
Version is stamped via `-ldflags "-X main.version=..."` (override: `VERSION=x.y.z make build`).

## Key files
| File | Purpose |
|------|---------|
| `main.go` | CLI entry point — flags (-a, -d, -l, --labels, --themes, --jump, --pick), jump resolution |
| `store.go` | Bookmark persistence — JSON at `~/.config/hop/bookmarks.json`, atomic save, frecency fields |
| `fuzzy.go` | Scored subsequence matcher returning match indexes for highlighting |
| `config.go` | Theme + color + icon config — embedded themes (`themes/*.toml` via go:embed), user overrides from `~/.config/hop/config.toml` |
| `icons.go` | Nerd Font icon tables — VS Code-style dir-name and filename/extension → glyph maps, user-extendable via `[icons.dirs]`/`[icons.files]` |
| `preview.go` | Async preview data: dir/file listing, git branch (reads `.git/HEAD`, no git binary), dead-path checks |
| `tui.go` | Bubbletea TUI — bookmarks view + filesystem browser + label prompt, all key/mouse handling |
| `init.go` | zsh/bash/fish shell integration scripts (printed by `hop init <shell>`) |

## Architecture
Two views plus a modal label prompt share one `model` struct:
- **Bookmarks view** (`modeBookmarks`) — fuzzy-filterable, score-ranked list of saved bookmarks with match highlighting
- **Browser view** (`modeBrowser`) — live filesystem directory browser, entered via `→`
- **Label prompt** (`inputMode != inputNone`) — intercepts all keys for add/rename label entry

Bookmarks are stored in natural insertion order; `Ctrl+F` toggles a frecency-sorted *view* (the store order is never rewritten by sorting). `Shift+↑/↓` reorders manually.

`hop <query>` (via the shell function → `hop --jump`) resolves: exact label match → unique fuzzy match → otherwise opens the picker pre-filtered.

## Key behaviours to preserve
- `Ctrl+A` in browser adds the **cursor-selected** directory, not the parent
- `Ctrl+D` (delete) only works in bookmarks view; `Ctrl+Z` restores the last delete at its old index
- `Shift+↑/↓` reorder is disabled when a filter is active **or** frecency sort is on
- `store.move(i, delta)` operates on raw store indices — only valid when `m.filter == ""` and manual sort
- Atomic save: write to `.tmp` then `os.Rename` (prevents corruption on crash)
- Selecting a bookmark (TUI Enter, click-click, or `--jump`) calls `store.touch` to record frecency
- Async messages carry their source path (`previewMsg.path`, `dirEntriesMsg.path`) and are dropped if stale
- Old `bookmarks.json` files (without `use_count`/`last_used`) must keep loading unchanged
- `build.sh` must `rm` bin/hop before copying over it — macOS SIGKILLs binaries overwritten in place (stale signature cache)
- Terminal background is only probed (OSC 11) when no explicit `theme` is configured, to avoid the startup round-trip
- Icon lookup precedence: full filename → extension → generic (dirs: basename → generic); icons render only with `nerd_font = true`, and row width math assumes glyph width 1 plus a space (`iconW = 2`)
