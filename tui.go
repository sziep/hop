package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

type pickResult struct {
	path    string
	aborted bool
}

type viewMode int

const (
	modeBookmarks viewMode = iota
	modeBrowser
)

type previewMsg string

type dirEntriesMsg struct {
	path    string
	entries []string
}

type tuiStyles struct {
	selected  lipgloss.Style
	label     lipgloss.Style
	path      lipgloss.Style
	filter    lipgloss.Style
	filterDim lipgloss.Style
	border    lipgloss.Style
	preview   lipgloss.Style
	header    lipgloss.Style
	hint      lipgloss.Style
	status    lipgloss.Style
}

func newStyles(r *lipgloss.Renderer, c colorConfig) tuiStyles {
	return tuiStyles{
		selected:  r.NewStyle().Bold(true).Foreground(lipgloss.Color(c.SelectedFg)).Background(lipgloss.Color(c.SelectedBg)),
		label:     r.NewStyle().Foreground(lipgloss.Color(c.Label)),
		path:      r.NewStyle().Foreground(lipgloss.Color(c.Path)),
		filter:    r.NewStyle().Foreground(lipgloss.Color(c.Filter)),
		filterDim: r.NewStyle().Foreground(lipgloss.Color(c.FilterDim)),
		border:    r.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(c.Border)),
		preview:   r.NewStyle().Foreground(lipgloss.Color(c.Preview)),
		header:    r.NewStyle().Bold(true).Foreground(lipgloss.Color(c.Header)),
		hint:      r.NewStyle().Foreground(lipgloss.Color(c.Hint)),
		status:    r.NewStyle().Foreground(lipgloss.Color(c.Status)),
	}
}

type paneDims struct {
	listWidth    int
	previewWidth int
	innerH       int
	listItemH    int
	showPreview  bool
}

type model struct {
	// bookmarks state
	all       []*bookmark
	filtered  []*bookmark
	cursor    int
	filter    string
	storeRef  *store
	cwd       string
	statusMsg string

	// browser state
	mode              viewMode
	browserPath       string
	browserEntries    []string
	browserFiltered   []string
	browserFilter     string
	browserCursor     int
	browserFocusEntry string // re-focus cursor on this entry when dirEntriesMsg arrives
	showHidden        bool

	// shared state
	previewLines []string
	width        int
	height       int
	aborted      bool
	selected     string
	styles       tuiStyles
}

// fuzzyMatch reports whether s contains all runes in pattern as a subsequence.
// pattern must already be lowercased by the caller.
func fuzzyMatch(s string, pattern []rune) bool {
	runes := []rune(strings.ToLower(s))
	si := 0
	for _, p := range pattern {
		found := false
		for ; si < len(runes); si++ {
			if runes[si] == p {
				found = true
				si++
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (m model) applyFilter() []*bookmark {
	if m.filter == "" {
		return m.all
	}
	pattern := []rune(strings.ToLower(m.filter))
	var out []*bookmark
	for _, b := range m.all {
		if fuzzyMatch(b.Label, pattern) || fuzzyMatch(b.Path, pattern) {
			out = append(out, b)
		}
	}
	return out
}

func (m model) applyBrowserFilter() []string {
	if m.browserFilter == "" {
		return m.browserEntries
	}
	pattern := []rune(strings.ToLower(m.browserFilter))
	var out []string
	for _, name := range m.browserEntries {
		if fuzzyMatch(name, pattern) {
			out = append(out, name)
		}
	}
	return out
}

// fetchDirPreview returns a clean list of navigable subdirectories for the preview pane.
func fetchDirPreview(path string, showHidden bool) tea.Cmd {
	return func() tea.Msg {
		dirs := readSubdirs(path, showHidden)
		if len(dirs) == 0 {
			return previewMsg("(no subdirectories)")
		}
		lines := make([]string, len(dirs))
		for i, d := range dirs {
			lines[i] = d + "/"
		}
		return previewMsg(strings.Join(lines, "\n"))
	}
}

func loadDirEntries(path string, showHidden bool) tea.Cmd {
	return func() tea.Msg {
		entries := readSubdirs(path, showHidden)
		return dirEntriesMsg{path: path, entries: entries}
	}
}

func readSubdirs(path string, showHidden bool) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		dirs = append(dirs, name)
	}
	slices.Sort(dirs)
	return dirs
}

func newModel(s tuiStyles, storeRef *store, cwd string) model {
	all := storeRef.all()
	return model{
		all:      all,
		filtered: all,
		styles:   s,
		storeRef: storeRef,
		cwd:      cwd,
	}
}

func (m model) Init() tea.Cmd {
	if len(m.filtered) > 0 {
		return fetchDirPreview(m.filtered[0].Path, m.showHidden)
	}
	return nil
}

func (m model) moveCursor(delta int) (model, tea.Cmd) {
	if len(m.filtered) == 0 {
		return m, nil
	}
	m.cursor = (m.cursor + delta + len(m.filtered)) % len(m.filtered)
	return m, fetchDirPreview(m.filtered[m.cursor].Path, m.showHidden)
}

func (m model) moveBrowserCursor(delta int) model {
	if len(m.browserFiltered) == 0 {
		return m
	}
	m.browserCursor = (m.browserCursor + delta + len(m.browserFiltered)) % len(m.browserFiltered)
	return m
}

func (m model) selectedBrowserPath() string {
	if len(m.browserFiltered) > 0 {
		return filepath.Join(m.browserPath, m.browserFiltered[m.browserCursor])
	}
	return m.browserPath
}

func (m model) enterBrowser(path string) (model, tea.Cmd) {
	m.mode = modeBrowser
	m.browserPath = path
	m.browserCursor = 0
	m.browserEntries = nil
	m.browserFiltered = nil
	m.browserFilter = ""
	return m, loadDirEntries(path, m.showHidden)
}

// refreshBookmarks rebuilds all/filtered from the store and clamps cursor.
// Called after any add/delete mutation.
func (m model) refreshBookmarks() (model, tea.Cmd) {
	m.all = m.storeRef.all()
	m.filtered = m.applyFilter()
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	if len(m.filtered) > 0 {
		return m, fetchDirPreview(m.filtered[m.cursor].Path, m.showHidden)
	}
	return m, nil
}

func (m model) doAdd() (model, tea.Cmd) {
	var addPath string
	if m.mode == modeBrowser {
		addPath = m.selectedBrowserPath()
	} else {
		addPath = m.cwd
	}
	label := filepath.Base(addPath)
	if slices.ContainsFunc(m.all, func(b *bookmark) bool { return b.Path == addPath }) {
		m.statusMsg = fmt.Sprintf("[%s] already bookmarked", label)
		return m, nil
	}
	m.storeRef.add(label, addPath)
	m.statusMsg = fmt.Sprintf("added [%s]", label)
	return m.refreshBookmarks()
}

func (m model) doDelete() (model, tea.Cmd) {
	if m.mode == modeBrowser {
		return m, nil
	}
	if len(m.filtered) == 0 {
		return m, nil
	}
	label := m.filtered[m.cursor].Label
	m.storeRef.remove(label)
	m.statusMsg = fmt.Sprintf("deleted [%s]", label)
	return m.refreshBookmarks()
}

func (m model) doMove(delta int) (model, tea.Cmd) {
	if m.mode == modeBrowser || m.filter != "" || len(m.filtered) == 0 {
		return m, nil
	}
	newIdx := m.cursor + delta
	if newIdx < 0 || newIdx >= len(m.filtered) {
		return m, nil
	}
	// m.cursor == index in s.bookmarks only when filter is empty (guarded above)
	m.storeRef.move(m.cursor, delta)
	m.cursor = newIdx
	return m.refreshBookmarks()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case previewMsg:
		m.previewLines = strings.Split(string(msg), "\n")
		return m, nil

	case dirEntriesMsg:
		if msg.path == m.browserPath {
			m.browserEntries = msg.entries
			m.browserFiltered = m.applyBrowserFilter()
			// re-focus cursor on the directory we navigated up from
			if m.browserFocusEntry != "" {
				if idx := slices.Index(m.browserFiltered, m.browserFocusEntry); idx >= 0 {
					m.browserCursor = idx
				}
				m.browserFocusEntry = ""
			}
			if len(m.browserFiltered) > 0 {
				return m, fetchDirPreview(filepath.Join(m.browserPath, m.browserFiltered[m.browserCursor]), m.showHidden)
			}
			return m, fetchDirPreview(m.browserPath, m.showHidden)
		}
		return m, nil

	case tea.KeyMsg:
		m.statusMsg = ""

		switch msg.Type {
		case tea.KeyCtrlC:
			m.aborted = true
			return m, tea.Quit

		case tea.KeyCtrlA:
			return m.doAdd()

		case tea.KeyCtrlD:
			return m.doDelete()

		case tea.KeyShiftUp:
			return m.doMove(-1)

		case tea.KeyShiftDown:
			return m.doMove(1)

		case tea.KeyEsc:
			if m.mode == modeBrowser {
				m.mode = modeBookmarks
				m.browserPath = ""
				m.browserEntries = nil
				var cmd tea.Cmd
				if len(m.filtered) > 0 {
					cmd = fetchDirPreview(m.filtered[m.cursor].Path, m.showHidden)
				}
				return m, cmd
			}
			m.aborted = true
			return m, tea.Quit

		case tea.KeyEnter:
			if m.mode == modeBrowser {
				m.selected = m.selectedBrowserPath()
			} else if len(m.filtered) > 0 {
				m.selected = m.filtered[m.cursor].Path
			}
			return m, tea.Quit

		case tea.KeyUp:
			if m.mode == modeBrowser {
				m = m.moveBrowserCursor(-1)
				if len(m.browserFiltered) > 0 {
					return m, fetchDirPreview(filepath.Join(m.browserPath, m.browserFiltered[m.browserCursor]), m.showHidden)
				}
				return m, nil
			}
			return m.moveCursor(-1)

		case tea.KeyDown:
			if m.mode == modeBrowser {
				m = m.moveBrowserCursor(1)
				if len(m.browserFiltered) > 0 {
					return m, fetchDirPreview(filepath.Join(m.browserPath, m.browserFiltered[m.browserCursor]), m.showHidden)
				}
				return m, nil
			}
			return m.moveCursor(1)

		case tea.KeyRight:
			if m.mode == modeBrowser {
				if len(m.browserFiltered) == 0 {
					return m, nil
				}
				return m.enterBrowser(m.selectedBrowserPath())
			}
			if len(m.filtered) == 0 {
				return m, nil
			}
			return m.enterBrowser(m.filtered[m.cursor].Path)

		case tea.KeyLeft:
			if m.mode == modeBrowser {
				parent := filepath.Dir(m.browserPath)
				if parent == m.browserPath {
					return m, nil
				}
				m.browserFocusEntry = filepath.Base(m.browserPath)
				return m.enterBrowser(parent)
			}
			if len(m.filtered) == 0 {
				return m, nil
			}
			return m.enterBrowser(filepath.Dir(m.filtered[m.cursor].Path))

		case tea.KeyBackspace:
			if m.mode == modeBrowser {
				if len(m.browserFilter) > 0 {
					runes := []rune(m.browserFilter)
					m.browserFilter = string(runes[:len(runes)-1])
					m.browserFiltered = m.applyBrowserFilter()
					if m.browserCursor >= len(m.browserFiltered) {
						m.browserCursor = max(0, len(m.browserFiltered)-1)
					}
					if len(m.browserFiltered) > 0 {
						return m, fetchDirPreview(filepath.Join(m.browserPath, m.browserFiltered[m.browserCursor]), m.showHidden)
					}
				}
				return m, nil
			}
			if len(m.filter) > 0 {
				runes := []rune(m.filter)
				m.filter = string(runes[:len(runes)-1])
				m.filtered = m.applyFilter()
				if m.cursor >= len(m.filtered) {
					m.cursor = max(0, len(m.filtered)-1)
				}
				if len(m.filtered) > 0 {
					return m, fetchDirPreview(m.filtered[m.cursor].Path, m.showHidden)
				}
			}
			return m, nil

		case tea.KeyRunes:
			if m.mode == modeBrowser {
				if len(msg.Runes) == 1 && msg.Runes[0] == '.' && m.browserFilter == "" {
					m.showHidden = !m.showHidden
					m.browserEntries = nil
					m.browserFiltered = nil
					m.browserCursor = 0
					return m, loadDirEntries(m.browserPath, m.showHidden)
				}
				for _, r := range msg.Runes {
					if unicode.IsPrint(r) {
						m.browserFilter += string(r)
					}
				}
				m.browserFiltered = m.applyBrowserFilter()
				m.browserCursor = 0
				if len(m.browserFiltered) > 0 {
					return m, fetchDirPreview(filepath.Join(m.browserPath, m.browserFiltered[0]), m.showHidden)
				}
				return m, nil
			}
			for _, r := range msg.Runes {
				if unicode.IsPrint(r) {
					m.filter += string(r)
				}
			}
			m.filtered = m.applyFilter()
			m.cursor = 0
			if len(m.filtered) > 0 {
				return m, fetchDirPreview(m.filtered[m.cursor].Path, m.showHidden)
			}
			return m, nil
		}
	}
	return m, nil
}

// truncatePath shortens path to maxLen display chars, keeping the tail.
func truncatePath(path string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(path)
	if len(runes) <= maxLen {
		return path
	}
	return "…" + string(runes[len(runes)-(maxLen-1):])
}

func (m model) dims() paneDims {
	const splitThreshold = 90
	showPreview := m.width >= splitThreshold
	var listWidth int
	if showPreview {
		listWidth = m.width * 44 / 100
	} else {
		listWidth = m.width - 2
	}
	innerH := m.height - 2
	if innerH < 4 {
		innerH = 4
	}
	return paneDims{
		listWidth:    listWidth,
		previewWidth: m.width - (m.width*44/100) - 5,
		innerH:       innerH,
		listItemH:    innerH - 3,
		showPreview:  showPreview,
	}
}

func (m model) renderPreviewPane(d paneDims) string {
	lines := m.previewLines
	if maxLines := d.innerH - 1; len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return m.styles.border.Width(d.previewWidth).Height(d.innerH).Render(
		m.styles.preview.Render(strings.Join(lines, "\n")),
	)
}

func (m model) statusOrHint(hintText string) string {
	if m.statusMsg != "" {
		return m.styles.status.Render("  " + m.statusMsg)
	}
	return m.styles.hint.Render("  " + hintText)
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.mode == modeBrowser {
		return m.viewBrowser()
	}
	return m.viewBookmarks()
}

func (m model) viewBookmarks() string {
	d := m.dims()

	labelWidth := 0
	for _, b := range m.all {
		if len(b.Label) > labelWidth {
			labelWidth = len(b.Label)
		}
	}

	start := 0
	if m.cursor >= d.listItemH {
		start = m.cursor - d.listItemH + 1
	}
	end := start + d.listItemH
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	maxPathLen := d.listWidth - labelWidth - 6

	visible := make([]string, d.listItemH)
	for i := start; i < end; i++ {
		b := m.filtered[i]
		labelStr := fmt.Sprintf("[%-*s]", labelWidth, b.Label)
		path := truncatePath(b.Path, maxPathLen)
		if i == m.cursor {
			visible[i-start] = m.styles.selected.Render("> " + labelStr + "  " + path)
		} else {
			visible[i-start] = "  " + m.styles.label.Render(labelStr) + "  " + m.styles.path.Render(path)
		}
	}

	var bottomLine string
	if m.filter != "" {
		bottomLine = m.styles.filter.Render("  /" + m.filter + "█")
	} else {
		bottomLine = m.statusOrHint("[^a] add  [^d] del  [⇧↑/↓] move  [→] browse")
	}

	header := m.styles.header.Render("  bookmarks")
	listContent := header + "\n\n" + strings.Join(visible, "\n") + "\n" + bottomLine
	listPane := m.styles.border.Width(d.listWidth).Height(d.innerH).Render(listContent)

	if d.showPreview {
		return lipgloss.JoinHorizontal(lipgloss.Top, listPane, " ", m.renderPreviewPane(d))
	}
	return listPane
}

func (m model) viewBrowser() string {
	d := m.dims()

	start := 0
	if m.browserCursor >= d.listItemH {
		start = m.browserCursor - d.listItemH + 1
	}
	end := start + d.listItemH
	if end > len(m.browserFiltered) {
		end = len(m.browserFiltered)
	}

	labelWidth := 0
	for _, n := range m.browserFiltered {
		if len(n) > labelWidth {
			labelWidth = len(n)
		}
	}

	maxPathLen := d.listWidth - labelWidth - 6

	visible := make([]string, d.listItemH)
	if len(m.browserFiltered) == 0 {
		visible[0] = m.styles.filterDim.Render("  (empty)")
	}
	for i := start; i < end; i++ {
		name := m.browserFiltered[i]
		labelStr := fmt.Sprintf("[%-*s]", labelWidth, name)
		fullPath := truncatePath(filepath.Join(m.browserPath, name)+"/", maxPathLen)
		if i == m.browserCursor {
			visible[i-start] = m.styles.selected.Render("> " + labelStr + "  " + fullPath)
		} else {
			visible[i-start] = "  " + m.styles.label.Render(labelStr) + "  " + m.styles.path.Render(fullPath)
		}
	}

	var bottomLine string
	if m.browserFilter != "" {
		bottomLine = m.styles.filter.Render("  /" + m.browserFilter + "█")
	} else {
		bottomLine = m.statusOrHint("[esc] back  [↵] select  [^a] add  [.] hidden")
	}

	header := m.styles.header.Render("  " + truncatePath(m.browserPath, d.listWidth-2))
	listContent := header + "\n\n" + strings.Join(visible, "\n") + "\n" + bottomLine
	listPane := m.styles.border.Width(d.listWidth).Height(d.innerH).Render(listContent)

	if d.showPreview {
		return lipgloss.JoinHorizontal(lipgloss.Top, listPane, " ", m.renderPreviewPane(d))
	}
	return listPane
}

func runPicker(storeRef *store, cwd string, colors colorConfig) pickResult {
	var renderer *lipgloss.Renderer
	var opts []tea.ProgramOption

	if term.IsTerminal(int(os.Stdout.Fd())) {
		renderer = lipgloss.NewRenderer(os.Stdout)
		opts = []tea.ProgramOption{tea.WithAltScreen()}
	} else {
		// stdout is a pipe (shell function's $(...) capture) — open /dev/tty for TUI
		tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, "hop: cannot open terminal:", err)
			return pickResult{aborted: true}
		}
		defer tty.Close()
		// Set renderer before newStyles so lipgloss detects color capability from tty
		renderer = lipgloss.NewRenderer(tty)
		termenv.SetDefaultOutput(termenv.NewOutput(tty))
		opts = []tea.ProgramOption{tea.WithInputTTY(), tea.WithOutput(tty), tea.WithAltScreen()}
	}

	s := newStyles(renderer, colors)
	m := newModel(s, storeRef, cwd)
	final, err := tea.NewProgram(m, opts...).Run()
	if err != nil {
		return pickResult{aborted: true}
	}
	result := final.(model)
	return pickResult{path: result.selected, aborted: result.aborted}
}
