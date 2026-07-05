# hop

A fast, keyboard-driven filesystem bookmark navigator for the terminal. Jump to frequently-used directories instantly — by name from the shell or through a fuzzy-filtered TUI picker — browse the filesystem from any bookmark, and manage bookmarks without leaving your shell.

## Installation

### Prerequisites

- Docker (used to compile the Go binary — no local Go installation needed)
- zsh, bash, or fish

### Build

```bash
make build
```

This produces `./bin/hop` (native binary) plus cross-compiled binaries for darwin/linux × arm64/amd64 in `./bin/`. Override the stamped version with `VERSION=x.y.z make build`.

### Install

```bash
rm -f /usr/local/bin/hop && cp bin/hop /usr/local/bin/hop
```

(The `rm` matters when upgrading: macOS kills binaries that are overwritten in place because of its cached code-signature check.)

### Shell integration

zsh — add to `~/.zshrc`:

```zsh
eval "$(hop init zsh)"
```

bash — add to `~/.bashrc`:

```bash
eval "$(hop init bash)"
```

fish — add to `~/.config/fish/config.fish`:

```fish
hop init fish | source
```

Reload your shell. The integration also installs tab completion for bookmark labels.

Optional alias to match legacy `cv` muscle memory:

```zsh
alias cv=hop
```

---

## Usage

### Direct jump

```bash
hop <query>
```

Resolves the query against your bookmarks and `cd`s straight there — no TUI. An exact label match wins; otherwise a unique fuzzy match (against label or path) jumps directly. If the query is ambiguous or matches nothing, the TUI opens pre-filtered to the query so the best match is one Enter away.

```bash
hop work        # exact label → instant cd
hop alp         # unique fuzzy match on "alpha" → instant cd
hop pr          # ambiguous → picker opens filtered to "pr"
```

Every jump (direct or via picker) updates the bookmark's frecency stats.

### Open the TUI

```bash
hop
```

Launches the picker. Select a bookmark and press Enter — your shell `cd`s to that directory. With zero bookmarks, the picker opens in browser mode so you can navigate somewhere and press `Ctrl+A` to create your first bookmark.

### Manage bookmarks from the shell

```bash
hop -a <label>          # bookmark the current working directory
hop -a <label> <path>   # bookmark an explicit path
hop -d <label>          # delete a bookmark
hop -l                  # list bookmarks
hop --themes            # list built-in color themes
hop --version           # print version
```

---

## TUI key bindings

### Bookmarks view

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move cursor (wraps) |
| `PgUp` / `PgDn` / `Home` / `End` | Page / jump to top or bottom |
| Mouse wheel / click | Scroll / select (click the selected row again to jump) |
| `Shift+↑` / `Shift+↓` | Move selected bookmark up / down (manual sort, no filter) |
| `→` | Enter filesystem browser at selected bookmark's path |
| `←` | Enter filesystem browser at parent of selected bookmark |
| `Enter` | Navigate to selected bookmark |
| `Ctrl+A` | Bookmark the current working directory (prompts for a label) |
| `Ctrl+R` | Rename the selected bookmark |
| `Ctrl+D` | Delete selected bookmark |
| `Ctrl+Z` | Undo the last delete |
| `Ctrl+F` | Toggle sort: manual order ↔ frecency (most used first) |
| Type anything | Fuzzy filter bookmarks by label or path (matches highlighted, best first) |
| `Ctrl+U` | Clear the filter |
| `Backspace` | Remove last filter character |
| `Esc` | Clear filter if active, otherwise quit |
| `Ctrl+C` | Quit without navigating |

Bookmarks whose directory no longer exists are shown in the dead color with a `✗` marker.

### Browser view

| Key | Action |
|-----|--------|
| `↑` / `↓` / `PgUp` / `PgDn` / `Home` / `End` | Move cursor |
| `→` | Enter selected subdirectory |
| `←` | Go to parent directory |
| `Enter` | Navigate to selected directory |
| `Ctrl+A` | Bookmark the selected directory (prompts for a label) |
| `.` | Toggle hidden directories (when no filter is active) |
| Type anything | Fuzzy filter visible directories |
| `Ctrl+U` | Clear the filter |
| `Esc` | Clear filter if active, otherwise return to bookmarks view |
| `Ctrl+C` | Quit without navigating |

### Label prompt (add / rename)

`Enter` confirms, `Esc` cancels, `Ctrl+U` clears. Duplicate labels are rejected inline.

### Preview pane

Shown on terminals ≥ 90 columns wide: the previewed path, its git branch if it's a repository (read from `.git/HEAD` directly — no git binary needed), subdirectories, and files (dimmed).

---

## Configuration

Create `~/.config/hop/config.toml`:

```toml
# Built-in themes: default, light, nord, dracula.
# With no theme set, hop picks default/light based on your terminal background.
theme = "default"

# Prefix entries with Nerd Font icons — see "Icons & Nerd Fonts" below.
nerd_font = false

# Override any individual color on top of the theme. All values are hex codes;
# omitted keys fall back to the theme.
[colors]
selected_bg = "#316168"  # cursor highlight background
selected_fg = "#aaedf5"  # cursor highlight foreground
label       = "#66D9E8"  # bookmark label brackets
path        = "#7e7e7e"  # bookmark path
filter      = "#FD971F"  # active filter text
filter_dim  = "#75715E"  # filter placeholder / empty
border      = "#316168"  # pane border
preview     = "#96b0bd"  # preview pane directories
preview_dim = "#5b6c81"  # preview pane files
header      = "#FD971F"  # pane header
hint        = "#5b6c81"  # hint bar text
status      = "#A6E22E"  # status message (add/delete confirmation)
match       = "#FD971F"  # fuzzy-match highlight
dead        = "#F92672"  # bookmarks whose directory is missing
git         = "#A6E22E"  # git branch line in the preview
```

The theme sources are in `themes/` and are embedded into the binary at build time.

---

## Icons & Nerd Fonts

Set `nerd_font = true` in `~/.config/hop/config.toml` to prefix every entry with an icon:

- **Bookmark and browser rows** get a folder icon based on the directory's basename — special directories (`.git`, `node_modules`, `src`, `docs`, `bin`, `build`, `test`, …) get their own glyphs, VS Code-style.
- **Preview pane files** get an icon by filename or extension — `.go`, `.py`, `.ts`, `.md`, `.json`, images, archives, and ~80 more mappings, with full filenames (`Makefile`, `Dockerfile`, `README.md`, `go.mod`, `LICENSE`, …) taking precedence over extensions.
- **The git line** uses a branch glyph instead of `⎇`.

### Font requirement

The glyphs live in the Unicode private-use area, so your **terminal font must be a [Nerd Font](https://www.nerdfonts.com)** (a font patched with the icon sets). If you see empty boxes or blanks instead of icons, your font isn't patched. On macOS:

```bash
brew install --cask font-jetbrains-mono-nerd-font   # or font-hack-nerd-font, etc.
```

Then select the font (e.g. "JetBrainsMono Nerd Font") in your terminal's profile settings. No font? Set `nerd_font = false` and hop renders plain text — nothing else changes.

### Customizing the mapping

Add `[icons.dirs]` and `[icons.files]` sections to extend or override the builtin tables. Values are the literal glyph — copy one from the [Nerd Fonts cheat sheet](https://www.nerdfonts.com/cheat-sheet):

```toml
nerd_font = true

[icons.dirs]
# keys are directory basenames (matched case-insensitively)
"terraform" = ""
"kubernetes" = "󱃾"

[icons.files]
# keys are full filenames or extensions (without the dot);
# full filenames win over extensions
"tf" = ""
"justfile" = ""
"go" = ""          # override a builtin
```

Lookup order per entry: full filename → extension → generic file icon (directories: basename → generic folder icon).

---

## Storage

Bookmarks are stored in `~/.config/hop/bookmarks.json`, including per-bookmark use counts and last-used timestamps for frecency ranking. The file is written atomically (temp file + rename) to prevent corruption. Files from older hop versions load unchanged.

---

## Limitations

- **Directories only** — the browser and preview list directories for navigation; files appear in the preview only.
- **Symlinked directories** are not listed in the browser.
- **Git dirty state** is not shown — only the branch name (hop reads `.git/HEAD` directly and does not shell out to git).
