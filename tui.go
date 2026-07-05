package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
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

type inputMode int

const (
	inputNone inputMode = iota
	inputAdd
	inputRename
)

type tuiStyles struct {
	selected   lipgloss.Style
	label      lipgloss.Style
	path       lipgloss.Style
	filter     lipgloss.Style
	filterDim  lipgloss.Style
	border     lipgloss.Style
	preview    lipgloss.Style
	previewDim lipgloss.Style
	header     lipgloss.Style
	hint       lipgloss.Style
	status     lipgloss.Style
	match      lipgloss.Style
	dead       lipgloss.Style
	git        lipgloss.Style
}

func newStyles(r *lipgloss.Renderer, c colorConfig) tuiStyles {
	return tuiStyles{
		selected:   r.NewStyle().Bold(true).Foreground(lipgloss.Color(c.SelectedFg)).Background(lipgloss.Color(c.SelectedBg)),
		label:      r.NewStyle().Foreground(lipgloss.Color(c.Label)),
		path:       r.NewStyle().Foreground(lipgloss.Color(c.Path)),
		filter:     r.NewStyle().Foreground(lipgloss.Color(c.Filter)),
		filterDim:  r.NewStyle().Foreground(lipgloss.Color(c.FilterDim)),
		border:     r.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(c.Border)),
		preview:    r.NewStyle().Foreground(lipgloss.Color(c.Preview)),
		previewDim: r.NewStyle().Foreground(lipgloss.Color(c.PreviewDim)),
		header:     r.NewStyle().Bold(true).Foreground(lipgloss.Color(c.Header)),
		hint:       r.NewStyle().Foreground(lipgloss.Color(c.Hint)),
		status:     r.NewStyle().Foreground(lipgloss.Color(c.Status)),
		match:      r.NewStyle().Bold(true).Foreground(lipgloss.Color(c.Match)),
		dead:       r.NewStyle().Foreground(lipgloss.Color(c.Dead)),
		git:        r.NewStyle().Foreground(lipgloss.Color(c.Git)),
	}
}

type paneDims struct {
	listWidth    int
	previewWidth int
	innerH       int
	listItemH    int
	showPreview  bool
}

// bmItem is a bookmark plus the fuzzy-match highlight indexes (rune offsets
// into the label / display path) for the active filter.
type bmItem struct {
	b       *bookmark
	labelHl []int
	pathHl  []int
}

type brItem struct {
	name string
	hl   []int
}

type model struct {
	// bookmarks state
	all          []*bookmark
	filtered     []bmItem
	cursor       int
	filter       string
	storeRef     *store
	cwd          string
	statusMsg    string
	sortFrecency bool
	dead         map[string]bool
	lastDeleted  *bookmark
	lastDelIdx   int

	// input prompt (add / rename label)
	inputMode  inputMode
	input      string
	inputErr   string
	inputPath  string // pending path for add
	renameFrom string

	// browser state
	mode              viewMode
	browserPath       string
	browserEntries    []string
	browserFiltered   []brItem
	browserFilter     string
	browserCursor     int
	browserFocusEntry string // re-focus cursor on this entry when dirEntriesMsg arrives
	showHidden        bool

	// shared state
	preview       previewData
	previewTarget string
	width         int
	height        int
	aborted       bool
	selected      string
	styles        tuiStyles
	nerdFont      bool
	icons         iconSet
}

// currentOrder returns bookmarks in manual store order, or frecency-ranked
// when the sort toggle is active (stable: ties keep manual order).
func (m model) currentOrder() []*bookmark {
	all := m.storeRef.all()
	if m.sortFrecency {
		now := time.Now().Unix()
		sort.SliceStable(all, func(i, j int) bool {
			return frecencyScore(all[i], now) > frecencyScore(all[j], now)
		})
	}
	return all
}

func (m model) applyFilter() []bmItem {
	if m.filter == "" {
		out := make([]bmItem, len(m.all))
		for i, b := range m.all {
			out[i] = bmItem{b: b}
		}
		return out
	}
	pattern := []rune(strings.ToLower(m.filter))
	query := normalizeLabel(m.filter)
	type scored struct {
		item  bmItem
		score int
	}
	var res []scored
	for _, b := range m.all {
		labelRes, labelOK := fuzzyScore(b.Label, pattern)
		pathRes, pathOK := fuzzyScore(displayPath(b.Path), pattern)
		if !labelOK && !pathOK {
			continue
		}
		it := bmItem{b: b}
		score := 0
		if labelOK {
			it.labelHl = labelRes.indexes
			score = labelRes.score + 64 // label hits outrank path hits
			if strings.EqualFold(b.Label, query) {
				score += 1000
			}
		}
		if pathOK {
			it.pathHl = pathRes.indexes
			if !labelOK || pathRes.score > score {
				score = pathRes.score
			}
		}
		res = append(res, scored{item: it, score: score})
	}
	sort.SliceStable(res, func(i, j int) bool { return res[i].score > res[j].score })
	out := make([]bmItem, len(res))
	for i, r := range res {
		out[i] = r.item
	}
	return out
}

func (m model) applyBrowserFilter() []brItem {
	if m.browserFilter == "" {
		out := make([]brItem, len(m.browserEntries))
		for i, name := range m.browserEntries {
			out[i] = brItem{name: name}
		}
		return out
	}
	pattern := []rune(strings.ToLower(m.browserFilter))
	type scored struct {
		item  brItem
		score int
	}
	var res []scored
	for _, name := range m.browserEntries {
		if r, ok := fuzzyScore(name, pattern); ok {
			res = append(res, scored{item: brItem{name: name, hl: r.indexes}, score: r.score})
		}
	}
	sort.SliceStable(res, func(i, j int) bool { return res[i].score > res[j].score })
	out := make([]brItem, len(res))
	for i, r := range res {
		out[i] = r.item
	}
	return out
}

func newModel(s tuiStyles, storeRef *store, cwd, initialFilter string, nerdFont bool, icons iconSet) model {
	m := model{
		styles:   s,
		storeRef: storeRef,
		cwd:      cwd,
		nerdFont: nerdFont,
		icons:    icons,
		dead:     map[string]bool{},
	}
	m.all = m.currentOrder()
	m.filter = initialFilter
	m.filtered = m.applyFilter()
	if len(m.all) == 0 {
		// first run: drop straight into the browser instead of erroring out
		m.mode = modeBrowser
		m.browserPath = cwd
		m.statusMsg = "no bookmarks yet — pick a directory and press ^a"
	} else if len(m.filtered) > 0 {
		m.previewTarget = m.filtered[0].b.Path
	}
	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.checkDeadCmd()}
	if m.mode == modeBrowser {
		cmds = append(cmds, loadDirEntries(m.browserPath, m.showHidden))
	} else if m.previewTarget != "" {
		cmds = append(cmds, fetchPreview(m.previewTarget, m.showHidden))
	}
	return tea.Batch(cmds...)
}

func (m model) checkDeadCmd() tea.Cmd {
	bs := m.storeRef.all()
	paths := make([]string, len(bs))
	for i, b := range bs {
		paths[i] = b.Path
	}
	return checkDead(paths)
}

// withPreview records the preview target (so stale async results are dropped)
// and returns the command that fetches it.
func (m model) withPreview(path string) (model, tea.Cmd) {
	m.previewTarget = path
	return m, fetchPreview(path, m.showHidden)
}

func (m model) moveCursor(delta int) (model, tea.Cmd) {
	if len(m.filtered) == 0 {
		return m, nil
	}
	m.cursor = (m.cursor + delta + len(m.filtered)) % len(m.filtered)
	return m.withPreview(m.filtered[m.cursor].b.Path)
}

func (m model) moveBrowserCursor(delta int) (model, tea.Cmd) {
	if len(m.browserFiltered) == 0 {
		return m, nil
	}
	m.browserCursor = (m.browserCursor + delta + len(m.browserFiltered)) % len(m.browserFiltered)
	return m.withPreview(m.selectedBrowserPath())
}

// scrollCursor moves without wrapping — used by mouse wheel, paging, home/end.
func (m model) scrollCursor(delta int) (model, tea.Cmd) {
	if m.mode == modeBrowser {
		if len(m.browserFiltered) == 0 {
			return m, nil
		}
		c := clamp(m.browserCursor+delta, 0, len(m.browserFiltered)-1)
		if c == m.browserCursor {
			return m, nil
		}
		m.browserCursor = c
		return m.withPreview(m.selectedBrowserPath())
	}
	if len(m.filtered) == 0 {
		return m, nil
	}
	c := clamp(m.cursor+delta, 0, len(m.filtered)-1)
	if c == m.cursor {
		return m, nil
	}
	m.cursor = c
	return m.withPreview(m.filtered[m.cursor].b.Path)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m model) selectedBrowserPath() string {
	if len(m.browserFiltered) > 0 {
		return filepath.Join(m.browserPath, m.browserFiltered[m.browserCursor].name)
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
// Called after any add/delete/rename/reorder mutation.
func (m model) refreshBookmarks() (model, tea.Cmd) {
	m.all = m.currentOrder()
	m.filtered = m.applyFilter()
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	cmds := []tea.Cmd{m.checkDeadCmd()}
	if m.mode == modeBookmarks && len(m.filtered) > 0 {
		var cmd tea.Cmd
		m, cmd = m.withPreview(m.filtered[m.cursor].b.Path)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// startAdd opens the label prompt for bookmarking cwd (bookmarks view) or the
// cursor-selected directory (browser view).
func (m model) startAdd() (model, tea.Cmd) {
	var addPath string
	if m.mode == modeBrowser {
		addPath = m.selectedBrowserPath()
	} else {
		addPath = m.cwd
	}
	if ex := m.storeRef.byPath(addPath); ex != nil {
		m.statusMsg = fmt.Sprintf("already bookmarked as [%s]", ex.Label)
		return m, nil
	}
	m.inputMode = inputAdd
	m.inputPath = addPath
	m.input = filepath.Base(addPath)
	m.inputErr = ""
	return m, nil
}

func (m model) startRename() (model, tea.Cmd) {
	if m.mode != modeBookmarks || len(m.filtered) == 0 {
		return m, nil
	}
	m.inputMode = inputRename
	m.renameFrom = m.filtered[m.cursor].b.Label
	m.input = m.renameFrom
	m.inputErr = ""
	return m, nil
}

func (m model) commitInput() (tea.Model, tea.Cmd) {
	label := normalizeLabel(m.input)
	if label == "" {
		m.inputErr = "empty label"
		return m, nil
	}
	switch m.inputMode {
	case inputAdd:
		if ex := m.storeRef.byLabel(label); ex != nil && ex.Path != m.inputPath {
			m.inputErr = "label taken"
			return m, nil
		}
		m.storeRef.add(label, m.inputPath)
		m.statusMsg = fmt.Sprintf("added [%s]", label)
	case inputRename:
		if label != m.renameFrom {
			if m.storeRef.byLabel(label) != nil {
				m.inputErr = "label taken"
				return m, nil
			}
			m.storeRef.rename(m.renameFrom, label)
			m.statusMsg = fmt.Sprintf("renamed [%s] to [%s]", m.renameFrom, label)
		}
	}
	m.inputMode = inputNone
	m.input = ""
	m.inputErr = ""
	return m.refreshBookmarks()
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.aborted = true
		return m, tea.Quit
	case tea.KeyEsc:
		m.inputMode = inputNone
		m.input = ""
		m.inputErr = ""
		return m, nil
	case tea.KeyEnter:
		return m.commitInput()
	case tea.KeyBackspace:
		if len(m.input) > 0 {
			runes := []rune(m.input)
			m.input = string(runes[:len(runes)-1])
		}
		m.inputErr = ""
		return m, nil
	case tea.KeyCtrlU:
		m.input = ""
		m.inputErr = ""
		return m, nil
	case tea.KeySpace:
		m.input += " "
		m.inputErr = ""
		return m, nil
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			if unicode.IsPrint(r) {
				m.input += string(r)
			}
		}
		m.inputErr = ""
		return m, nil
	}
	return m, nil
}

func (m model) doDelete() (tea.Model, tea.Cmd) {
	if m.mode == modeBrowser || len(m.filtered) == 0 {
		return m, nil
	}
	b := m.filtered[m.cursor].b
	m.lastDelIdx = m.storeRef.indexOf(b.Label)
	deleted := *b
	m.lastDeleted = &deleted
	m.storeRef.remove(b.Label)
	m.statusMsg = fmt.Sprintf("deleted [%s] — ^z to undo", b.Label)
	return m.refreshBookmarks()
}

func (m model) doUndo() (tea.Model, tea.Cmd) {
	if m.lastDeleted == nil {
		return m, nil
	}
	if m.storeRef.byLabel(m.lastDeleted.Label) != nil {
		m.lastDeleted = nil
		return m, nil
	}
	m.storeRef.insertAt(m.lastDelIdx, m.lastDeleted)
	m.statusMsg = fmt.Sprintf("restored [%s]", m.lastDeleted.Label)
	m.lastDeleted = nil
	return m.refreshBookmarks()
}

func (m model) doMove(delta int) (tea.Model, tea.Cmd) {
	if m.mode == modeBrowser || m.filter != "" || len(m.filtered) == 0 {
		return m, nil
	}
	if m.sortFrecency {
		m.statusMsg = "reordering needs manual sort — toggle with ^f"
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

func (m model) toggleSort() (tea.Model, tea.Cmd) {
	m.sortFrecency = !m.sortFrecency
	if m.sortFrecency {
		m.statusMsg = "sorted by frecency"
	} else {
		m.statusMsg = "manual order"
	}
	m.cursor = 0
	return m.refreshBookmarks()
}

// clearFilter resets the active filter in whichever view is showing.
func (m model) clearFilter() (model, tea.Cmd) {
	if m.mode == modeBrowser {
		if m.browserFilter == "" {
			return m, nil
		}
		m.browserFilter = ""
		m.browserFiltered = m.applyBrowserFilter()
		m.browserCursor = clamp(m.browserCursor, 0, max(0, len(m.browserFiltered)-1))
		if len(m.browserFiltered) > 0 {
			return m.withPreview(m.selectedBrowserPath())
		}
		return m, nil
	}
	if m.filter == "" {
		return m, nil
	}
	m.filter = ""
	m.filtered = m.applyFilter()
	m.cursor = clamp(m.cursor, 0, max(0, len(m.filtered)-1))
	if len(m.filtered) > 0 {
		return m.withPreview(m.filtered[m.cursor].b.Path)
	}
	return m, nil
}

// listStart is the scroll offset keeping the cursor visible; shared by the
// views and mouse hit-testing.
func listStart(cursor, itemH int) int {
	if cursor >= itemH {
		return cursor - itemH + 1
	}
	return 0
}

func (m model) handleClick(x, y int) (tea.Model, tea.Cmd) {
	d := m.dims()
	if x > d.listWidth+1 {
		return m, nil
	}
	row := y - 3 // border + header + blank line
	if row < 0 || row >= d.listItemH {
		return m, nil
	}
	if m.mode == modeBrowser {
		idx := listStart(m.browserCursor, d.listItemH) + row
		if idx >= len(m.browserFiltered) {
			return m, nil
		}
		if idx == m.browserCursor {
			m.selected = m.selectedBrowserPath()
			return m.quitSelected()
		}
		m.browserCursor = idx
		return m.withPreview(m.selectedBrowserPath())
	}
	idx := listStart(m.cursor, d.listItemH) + row
	if idx >= len(m.filtered) {
		return m, nil
	}
	if idx == m.cursor {
		m.selected = m.filtered[m.cursor].b.Path
		return m.quitSelected()
	}
	m.cursor = idx
	return m.withPreview(m.filtered[m.cursor].b.Path)
}

// quitSelected records frecency for the chosen path and quits.
func (m model) quitSelected() (tea.Model, tea.Cmd) {
	if b := m.storeRef.byPath(m.selected); b != nil {
		m.storeRef.touch(b.Label)
	}
	return m, tea.Quit
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case previewMsg:
		if msg.path == m.previewTarget {
			m.preview = previewData(msg)
		}
		return m, nil

	case deadMsg:
		m.dead = msg
		return m, nil

	case dirEntriesMsg:
		if msg.path == m.browserPath {
			m.browserEntries = msg.entries
			m.browserFiltered = m.applyBrowserFilter()
			// re-focus cursor on the directory we navigated up from
			if m.browserFocusEntry != "" {
				if idx := slices.IndexFunc(m.browserFiltered, func(it brItem) bool { return it.name == m.browserFocusEntry }); idx >= 0 {
					m.browserCursor = idx
				}
				m.browserFocusEntry = ""
			}
			return m.withPreview(m.selectedBrowserPath())
		}
		return m, nil

	case tea.MouseMsg:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			return m.scrollCursor(-1)
		case tea.MouseButtonWheelDown:
			return m.scrollCursor(1)
		case tea.MouseButtonLeft:
			if m.inputMode == inputNone {
				return m.handleClick(msg.X, msg.Y)
			}
		}
		return m, nil

	case tea.KeyMsg:
		if m.inputMode != inputNone {
			return m.updateInput(msg)
		}
		m.statusMsg = ""

		switch msg.Type {
		case tea.KeyCtrlC:
			m.aborted = true
			return m, tea.Quit

		case tea.KeyCtrlA:
			return m.startAdd()

		case tea.KeyCtrlR:
			return m.startRename()

		case tea.KeyCtrlD:
			return m.doDelete()

		case tea.KeyCtrlZ:
			return m.doUndo()

		case tea.KeyCtrlF:
			return m.toggleSort()

		case tea.KeyCtrlU:
			return m.clearFilter()

		case tea.KeyShiftUp:
			return m.doMove(-1)

		case tea.KeyShiftDown:
			return m.doMove(1)

		case tea.KeyEsc:
			// esc peels back one layer: filter → view → quit
			if m.mode == modeBrowser {
				if m.browserFilter != "" {
					return m.clearFilter()
				}
				m.mode = modeBookmarks
				m.browserPath = ""
				m.browserEntries = nil
				if len(m.filtered) > 0 {
					return m.withPreview(m.filtered[m.cursor].b.Path)
				}
				return m, nil
			}
			if m.filter != "" {
				return m.clearFilter()
			}
			m.aborted = true
			return m, tea.Quit

		case tea.KeyEnter:
			if m.mode == modeBrowser {
				m.selected = m.selectedBrowserPath()
			} else if len(m.filtered) > 0 {
				m.selected = m.filtered[m.cursor].b.Path
			} else {
				m.aborted = true
				return m, tea.Quit
			}
			return m.quitSelected()

		case tea.KeyUp:
			if m.mode == modeBrowser {
				return m.moveBrowserCursor(-1)
			}
			return m.moveCursor(-1)

		case tea.KeyDown:
			if m.mode == modeBrowser {
				return m.moveBrowserCursor(1)
			}
			return m.moveCursor(1)

		case tea.KeyPgUp:
			return m.scrollCursor(-m.dims().listItemH)

		case tea.KeyPgDown:
			return m.scrollCursor(m.dims().listItemH)

		case tea.KeyHome:
			return m.scrollCursor(-1 << 30)

		case tea.KeyEnd:
			return m.scrollCursor(1 << 30)

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
			return m.enterBrowser(m.filtered[m.cursor].b.Path)

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
			return m.enterBrowser(filepath.Dir(m.filtered[m.cursor].b.Path))

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
						return m.withPreview(m.selectedBrowserPath())
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
					return m.withPreview(m.filtered[m.cursor].b.Path)
				}
			}
			return m, nil

		case tea.KeySpace:
			return m.typeFilter(" ")

		case tea.KeyRunes:
			if m.mode == modeBrowser && len(msg.Runes) == 1 && msg.Runes[0] == '.' && m.browserFilter == "" {
				m.showHidden = !m.showHidden
				m.browserEntries = nil
				m.browserFiltered = nil
				m.browserCursor = 0
				return m, loadDirEntries(m.browserPath, m.showHidden)
			}
			var sb strings.Builder
			for _, r := range msg.Runes {
				if unicode.IsPrint(r) {
					sb.WriteRune(r)
				}
			}
			return m.typeFilter(sb.String())
		}
	}
	return m, nil
}

func (m model) typeFilter(s string) (tea.Model, tea.Cmd) {
	if s == "" {
		return m, nil
	}
	if m.mode == modeBrowser {
		m.browserFilter += s
		m.browserFiltered = m.applyBrowserFilter()
		m.browserCursor = 0
		if len(m.browserFiltered) > 0 {
			return m.withPreview(m.selectedBrowserPath())
		}
		return m, nil
	}
	m.filter += s
	m.filtered = m.applyFilter()
	m.cursor = 0
	if len(m.filtered) > 0 {
		return m.withPreview(m.filtered[0].b.Path)
	}
	return m, nil
}

// truncatePath shortens path to maxLen display chars, keeping the tail.
func truncatePath(path string, maxLen int) string {
	s, _ := truncatePathHl(path, nil, maxLen)
	return s
}

// truncatePathHl truncates keeping the tail and shifts highlight indexes to
// match the truncated string (dropping ones that fell off the front).
func truncatePathHl(path string, hl []int, maxLen int) (string, []int) {
	if maxLen <= 0 {
		return "", nil
	}
	runes := []rune(path)
	if len(runes) <= maxLen {
		return path, hl
	}
	offset := len(runes) - (maxLen - 1)
	out := "…" + string(runes[offset:])
	var shifted []int
	for _, i := range hl {
		if i >= offset {
			shifted = append(shifted, i-offset+1)
		}
	}
	return out, shifted
}

// truncateEnd shortens s to maxLen display chars, keeping the head.
func truncateEnd(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

// renderHl renders s with the runes at hl indexes in the match style.
func renderHl(s string, hl []int, base, match lipgloss.Style) string {
	if len(hl) == 0 {
		return base.Render(s)
	}
	set := make(map[int]bool, len(hl))
	for _, i := range hl {
		set[i] = true
	}
	runes := []rune(s)
	var b strings.Builder
	for i := 0; i < len(runes); {
		j := i
		matched := set[i]
		for j < len(runes) && set[j] == matched {
			j++
		}
		seg := string(runes[i:j])
		if matched {
			b.WriteString(match.Render(seg))
		} else {
			b.WriteString(base.Render(seg))
		}
		i = j
	}
	return b.String()
}

func (m model) dims() paneDims {
	const splitThreshold = 90
	showPreview := m.width >= splitThreshold
	var listWidth, previewWidth int
	if showPreview {
		listWidth = m.width * 44 / 100
		previewWidth = m.width - listWidth - 5
	} else {
		listWidth = m.width - 2
	}
	innerH := m.height - 2
	if innerH < 4 {
		innerH = 4
	}
	return paneDims{
		listWidth:    listWidth,
		previewWidth: previewWidth,
		innerH:       innerH,
		listItemH:    innerH - 3,
		showPreview:  showPreview,
	}
}

func (m model) renderPreviewPane(d paneDims) string {
	w := d.previewWidth - 2
	pv := m.preview
	var lines []string
	if pv.path != "" {
		lines = append(lines, m.styles.header.Render(truncatePath(displayPath(pv.path), w)))
		if pv.branch != "" {
			icon := "⎇ "
			if m.nerdFont {
				icon = iconGitBranch + " "
			}
			lines = append(lines, m.styles.git.Render(truncateEnd(icon+pv.branch, w)))
		}
		lines = append(lines, "")
	}
	total := len(pv.dirs) + len(pv.files)
	switch {
	case pv.missing:
		lines = append(lines, m.styles.dead.Render("(missing)"))
	case pv.failed:
		lines = append(lines, m.styles.filterDim.Render("(unreadable)"))
	case total == 0:
		lines = append(lines, m.styles.filterDim.Render("(empty)"))
	default:
		budget := d.innerH - len(lines)
		if total > budget {
			budget-- // reserve a line for the "+N more" marker
		}
		count := 0
		for _, dir := range pv.dirs {
			if count >= budget {
				break
			}
			icon := ""
			if m.nerdFont {
				icon = m.icons.dir(dir)
			}
			lines = append(lines, m.styles.preview.Render(truncateEnd(icon+dir+"/", w)))
			count++
		}
		for _, f := range pv.files {
			if count >= budget {
				break
			}
			icon := ""
			if m.nerdFont {
				icon = m.icons.file(f)
			}
			lines = append(lines, m.styles.previewDim.Render(truncateEnd(icon+f, w)))
			count++
		}
		if total > count {
			lines = append(lines, m.styles.previewDim.Render(fmt.Sprintf("… +%d more", total-count)))
		}
	}
	if len(lines) > d.innerH {
		lines = lines[:d.innerH]
	}
	return m.styles.border.Width(d.previewWidth).Height(d.innerH).Render(strings.Join(lines, "\n"))
}

// renderHeader lays out a left title and right-aligned counter within width.
func (m model) renderHeader(left, right string, width int) string {
	pad := width - runewidth.StringWidth(left) - runewidth.StringWidth(right) - 1
	if pad < 1 {
		pad = 1
	}
	return m.styles.header.Render(left) + strings.Repeat(" ", pad) + m.styles.hint.Render(right)
}

// bottomLine renders prompt > filter > status/hint, truncated so it never
// wraps and breaks the pane layout on narrow terminals.
func (m model) bottomLine(filter, hint string, maxW int) string {
	if m.inputMode != inputNone {
		verb := "add"
		if m.inputMode == inputRename {
			verb = "rename"
		}
		text := "  " + verb + ": " + m.input + "█"
		if m.inputErr != "" {
			err := "  (" + m.inputErr + ")"
			return m.styles.filter.Render(truncatePath(text, maxW-len(err))) + m.styles.dead.Render(err)
		}
		return m.styles.filter.Render(truncatePath(text, maxW))
	}
	if filter != "" {
		// keep the tail — that's where the user is typing
		return m.styles.filter.Render(truncatePath("  /"+filter+"█", maxW))
	}
	if m.statusMsg != "" {
		return m.styles.status.Render(truncateEnd("  "+m.statusMsg, maxW))
	}
	return m.styles.hint.Render(truncateEnd("  "+hint, maxW))
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
		if w := runewidth.StringWidth(b.Label); w > labelWidth {
			labelWidth = w
		}
	}

	iconW := 0
	if m.nerdFont {
		iconW = 2
	}

	start := listStart(m.cursor, d.listItemH)
	end := start + d.listItemH
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	maxPathLen := d.listWidth - labelWidth - 6 - iconW

	visible := make([]string, d.listItemH)
	if len(m.filtered) == 0 {
		if m.filter != "" {
			visible[0] = m.styles.filterDim.Render("  (no matches)")
		} else {
			visible[0] = m.styles.filterDim.Render("  (no bookmarks — ^a adds the current directory)")
		}
	}
	for i := start; i < end; i++ {
		it := m.filtered[i]
		b := it.b
		icon := ""
		if m.nerdFont {
			icon = m.icons.dir(filepath.Base(b.Path))
		}
		pad := strings.Repeat(" ", max(0, labelWidth-runewidth.StringWidth(b.Label)))
		shown, pathHl := truncatePathHl(displayPath(b.Path), it.pathHl, maxPathLen)
		isDead := m.dead[b.Path]
		if i == m.cursor {
			line := "> " + icon + "[" + b.Label + pad + "]  " + shown
			if isDead {
				line += " ✗"
			}
			visible[i-start] = m.styles.selected.Render(line)
			continue
		}
		labelStyle, pathStyle := m.styles.label, m.styles.path
		if isDead {
			labelStyle, pathStyle = m.styles.dead, m.styles.dead
		}
		row := "  " + m.styles.path.Render(icon) +
			labelStyle.Render("[") + renderHl(b.Label, it.labelHl, labelStyle, m.styles.match) + labelStyle.Render(pad+"]") +
			"  " + renderHl(shown, pathHl, pathStyle, m.styles.match)
		if isDead {
			row += m.styles.dead.Render(" ✗")
		}
		visible[i-start] = row
	}

	title := "  bookmarks"
	if m.sortFrecency {
		title += " · frecency"
	}
	var counter string
	if m.filter != "" {
		counter = fmt.Sprintf("%d/%d ", len(m.filtered), len(m.all))
	} else if len(m.filtered) > 0 {
		counter = fmt.Sprintf("%d/%d ", m.cursor+1, len(m.filtered))
	}
	header := m.renderHeader(title, counter, d.listWidth)

	bottomLine := m.bottomLine(m.filter, "[^a] add  [^r] rename  [^d] del  [^f] sort  [→] browse", d.listWidth-1)

	listContent := header + "\n\n" + strings.Join(visible, "\n") + "\n" + bottomLine
	listPane := m.styles.border.Width(d.listWidth).Height(d.innerH).Render(listContent)

	if d.showPreview {
		return lipgloss.JoinHorizontal(lipgloss.Top, listPane, " ", m.renderPreviewPane(d))
	}
	return listPane
}

func (m model) viewBrowser() string {
	d := m.dims()

	start := listStart(m.browserCursor, d.listItemH)
	end := start + d.listItemH
	if end > len(m.browserFiltered) {
		end = len(m.browserFiltered)
	}

	labelWidth := 0
	for _, it := range m.browserFiltered {
		if w := runewidth.StringWidth(it.name); w > labelWidth {
			labelWidth = w
		}
	}

	iconW := 0
	if m.nerdFont {
		iconW = 2
	}

	maxPathLen := d.listWidth - labelWidth - 6 - iconW

	visible := make([]string, d.listItemH)
	if len(m.browserFiltered) == 0 {
		visible[0] = m.styles.filterDim.Render("  (empty)")
	}
	for i := start; i < end; i++ {
		it := m.browserFiltered[i]
		icon := ""
		if m.nerdFont {
			icon = m.icons.dir(it.name)
		}
		pad := strings.Repeat(" ", max(0, labelWidth-runewidth.StringWidth(it.name)))
		fullPath := truncatePath(displayPath(filepath.Join(m.browserPath, it.name))+"/", maxPathLen)
		if i == m.browserCursor {
			visible[i-start] = m.styles.selected.Render("> " + icon + "[" + it.name + pad + "]  " + fullPath)
			continue
		}
		visible[i-start] = "  " + m.styles.path.Render(icon) +
			m.styles.label.Render("[") + renderHl(it.name, it.hl, m.styles.label, m.styles.match) + m.styles.label.Render(pad+"]") +
			"  " + m.styles.path.Render(fullPath)
	}

	var counter string
	if m.browserFilter != "" {
		counter = fmt.Sprintf("%d/%d ", len(m.browserFiltered), len(m.browserEntries))
	} else if len(m.browserFiltered) > 0 {
		counter = fmt.Sprintf("%d/%d ", m.browserCursor+1, len(m.browserFiltered))
	}
	title := "  " + truncatePath(displayPath(m.browserPath), d.listWidth-len(counter)-4)
	header := m.renderHeader(title, counter, d.listWidth)

	bottomLine := m.bottomLine(m.browserFilter, "[esc] back  [↵] select  [^a] add  [.] hidden", d.listWidth-1)

	listContent := header + "\n\n" + strings.Join(visible, "\n") + "\n" + bottomLine
	listPane := m.styles.border.Width(d.listWidth).Height(d.innerH).Render(listContent)

	if d.showPreview {
		return lipgloss.JoinHorizontal(lipgloss.Top, listPane, " ", m.renderPreviewPane(d))
	}
	return listPane
}

func runPicker(storeRef *store, cwd string, cfg hopConfig, initialFilter string) pickResult {
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
	opts = append(opts, tea.WithMouseCellMotion())

	// only probe the terminal background when we actually need to pick a
	// default theme — the OSC query costs a round-trip on startup
	dark := true
	if cfg.Theme == "" {
		dark = renderer.HasDarkBackground()
	}
	colors := resolveColors(cfg, dark)
	s := newStyles(renderer, colors)
	m := newModel(s, storeRef, cwd, initialFilter, cfg.NerdFont, newIconSet(cfg))
	final, err := tea.NewProgram(m, opts...).Run()
	if err != nil {
		return pickResult{aborted: true}
	}
	result := final.(model)
	return pickResult{path: result.selected, aborted: result.aborted}
}
