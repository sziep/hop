# hop

A fast, keyboard-driven filesystem bookmark navigator for the terminal. Jump to frequently-used directories instantly, browse the filesystem from any bookmark, and manage bookmarks without leaving your shell.

## Installation

### Prerequisites

- Docker (used to compile the Go binary — no local Go installation needed)
- zsh

### Build

```bash
make build
```

This produces `./bin/hop` (darwin/arm64 static binary).

### Install

```bash
cp bin/hop /usr/local/bin/hop
```

### Shell integration

Add to `~/.zshrc`:

```zsh
eval "$(hop init zsh)"
```

Reload:

```bash
source ~/.zshrc
```

Optional alias to match legacy `cv` muscle memory:

```zsh
alias cv=hop
```

---

## Usage

### Open the TUI

```bash
hop
```

Launches the picker. Select a bookmark and press Enter — your shell `cd`s to that directory.

### Add a bookmark

```bash
hop -a <label>
```

Bookmarks the current working directory with the given label.

```bash
hop -a work          # bookmark CWD as "work"
```

### Delete a bookmark

```bash
hop -d <label>
```

### List bookmarks

```bash
hop -l
```

Output is listed in the order bookmarks were added (same order as the TUI).

---

## TUI key bindings

### Bookmarks view

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move cursor |
| `Shift+↑` / `Shift+↓` | Move selected bookmark up / down |
| `→` | Enter filesystem browser at selected bookmark's path |
| `←` | Enter filesystem browser at parent of selected bookmark |
| `Enter` | Navigate to selected bookmark |
| `Ctrl+A` | Add current working directory as a bookmark |
| `Ctrl+D` | Delete selected bookmark |
| Type anything | Fuzzy filter bookmarks by label or path |
| `Backspace` | Remove last filter character |
| `Esc` | Quit without navigating |
| `Ctrl+C` | Quit without navigating |

### Browser view

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move cursor |
| `→` | Enter selected subdirectory |
| `←` | Go to parent directory |
| `Enter` | Navigate to selected directory |
| `Ctrl+A` | Bookmark the selected directory |
| Type anything | Fuzzy filter visible directories |
| `Backspace` | Remove last filter character |
| `Esc` | Return to bookmarks view |
| `Ctrl+C` | Quit without navigating |

---

## Theming

Create `~/.config/hop/config.toml` to override any colors. All values are hex color codes. Missing keys fall back to the defaults below.

```toml
[colors]
selected_bg = "#316168"  # cursor highlight background
selected_fg = "#aaedf5"  # cursor highlight foreground
label       = "#66D9E8"  # bookmark label brackets
path        = "#7e7e7e"  # bookmark path
filter      = "#FD971F"  # active filter text
filter_dim  = "#75715E"  # filter placeholder / empty
border      = "#316168"  # pane border
preview     = "#96b0bd"  # preview pane text
header      = "#FD971F"  # pane header
hint        = "#5b6c81"  # hint bar text
status      = "#A6E22E"  # status message (add/delete confirmation)
```

You only need to include the keys you want to change. A copy of the default theme is in `themes/default.toml`.

To start customizing:

```bash
cp themes/default.toml ~/.config/hop/config.toml
```

---

## Storage

Bookmarks are stored in `~/.config/hop/bookmarks.json`. The file is written atomically (temp file + rename) to prevent corruption.

---

## Limitations

- **zsh only** — the shell integration (`hop init zsh`) targets zsh. Other shells are not supported.
- **darwin/arm64 only** — the build script produces a macOS Apple Silicon binary. Adjust `GOOS`/`GOARCH` in `build.sh` for other targets.
- **Directories only** — the browser shows directories only; files are not listed.
- **Hidden directories excluded** — directories starting with `.` are not shown in the browser.
- **Label-based deduplication** — `hop -a <label>` silently overwrites an existing bookmark with the same label. Labels must be unique.
- **No label prompt in TUI** — `Ctrl+A` in the TUI always uses the directory's basename as the label. To bookmark with a custom label, use `hop -a <label>` from the shell.
