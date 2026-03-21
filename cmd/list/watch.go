package list

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gigurra/bm/pkg/db"
	"github.com/gigurra/bm/pkg/ollama"
	"github.com/gigurra/bm/pkg/table"
)

var (
	wHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250"))
	wHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	wSearchStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	wSemanticStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	wSelectedStyle = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("238"))
	wErrorStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
)

type searchMode int

const (
	modeNone     searchMode = iota
	modeText                // FTS5 text search
	modeSemantic            // Embedding-based semantic search
)

// Messages
type bookmarksLoadedMsg []db.Bookmark

type ftsResultMsg struct {
	results []db.Bookmark
	query   string
	err     error
}

type semanticResultMsg struct {
	results []semanticMatch
	query   string
	err     error
}

type semanticMatch struct {
	bookmark   db.Bookmark
	similarity float32
}

type debounceMsg struct {
	seq   int
	query string
	mode  searchMode
}

type watchModel struct {
	// Data
	allBookmarks []db.Bookmark
	displayed    []db.Bookmark
	scores       map[string]float32 // URL -> similarity (semantic mode)

	// Filters
	profileFilter string

	// Navigation
	cursor         int
	viewportOffset int
	width          int
	height         int

	// Sort
	sortState table.SortState

	// Text search
	searchInput   textinput.Model
	searchFocused bool

	// Semantic search
	semanticInput   textinput.Model
	semanticFocused bool

	// Shared search state
	searchMode  searchMode
	lastQuery   string
	searching   bool
	searchError string
	searchSeq   int // incremented on each keystroke, used for debounce

	// Help
	helpView bool
}

func newWatchModel() *watchModel {
	si := textinput.New()
	si.Prompt = ""
	si.CharLimit = 200

	sem := textinput.New()
	sem.Prompt = ""
	sem.CharLimit = 500

	return &watchModel{
		searchInput:   si,
		semanticInput: sem,
		scores:        make(map[string]float32),
		sortState:     table.SortState{Key: "added", Direction: table.SortDesc},
	}
}

func (m *watchModel) Init() tea.Cmd {
	return m.loadBookmarks()
}

func (m *watchModel) loadBookmarks() tea.Cmd {
	return func() tea.Msg {
		var bookmarks []db.Bookmark
		var err error
		if m.profileFilter != "" {
			bookmarks, err = db.ListBookmarksBySource(m.profileFilter)
		} else {
			bookmarks, err = db.ListBookmarks()
		}
		if err != nil {
			return ftsResultMsg{err: err}
		}
		return bookmarksLoadedMsg(bookmarks)
	}
}

func (m *watchModel) columns() []table.Column {
	cols := []table.Column{
		{Header: "", Width: 1},
		{Header: "TITLE", MinWidth: 20, Weight: 0.3, Truncate: true, SortKey: "title"},
		{Header: "URL", MinWidth: 20, Weight: 0.3, Truncate: true, TruncateMode: table.TruncateStart, SortKey: "url"},
		{Header: "FOLDER", MinWidth: 10, Weight: 0.25, Truncate: true, TruncateMode: table.TruncateStart, SortKey: "folder"},
		{Header: "ADDED", Width: 12, SortKey: "added"},
	}
	if m.searchMode == modeSemantic && m.scores != nil {
		cols = append(cols, table.Column{Header: "SCORE", Width: 8, Align: table.AlignRight})
	}
	return cols
}

func (m *watchModel) viewportRows() int {
	rows := m.height - 7
	if rows < 5 {
		rows = 5
	}
	return rows
}

func (m *watchModel) ensureVisible() {
	vp := m.viewportRows()
	if m.cursor < m.viewportOffset {
		m.viewportOffset = m.cursor
	}
	if m.cursor >= m.viewportOffset+vp {
		m.viewportOffset = m.cursor - vp + 1
	}
}

// --- tea.Model interface ---

func (m *watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case bookmarksLoadedMsg:
		m.allBookmarks = msg
		if m.searchMode == modeNone {
			m.displayed = make([]db.Bookmark, len(msg))
			copy(m.displayed, msg)
			m.resortDisplayed()
		}
		return m, nil

	case ftsResultMsg:
		m.searching = false
		if msg.err != nil {
			m.searchError = msg.err.Error()
			return m, nil
		}
		m.searchError = ""
		m.displayed = msg.results
		m.scores = nil
		m.cursor = 0
		m.viewportOffset = 0
		return m, nil

	case semanticResultMsg:
		m.searching = false
		if msg.err != nil {
			m.searchError = msg.err.Error()
			return m, nil
		}
		m.searchError = ""
		m.scores = make(map[string]float32)
		var bookmarks []db.Bookmark
		for _, r := range msg.results {
			bookmarks = append(bookmarks, r.bookmark)
			m.scores[r.bookmark.URL] = r.similarity
		}
		m.displayed = bookmarks
		m.cursor = 0
		m.viewportOffset = 0
		return m, nil

	case debounceMsg:
		if msg.seq != m.searchSeq {
			return m, nil // stale, user typed more since this debounce was scheduled
		}
		query := strings.TrimSpace(msg.query)
		if query == "" {
			m.searchMode = modeNone
			m.lastQuery = ""
			m.searchError = ""
			m.displayed = m.allBookmarks
			m.scores = nil
			m.cursor = 0
			m.viewportOffset = 0
			m.resortDisplayed()
			return m, nil
		}
		m.searchMode = msg.mode
		m.lastQuery = query
		m.searching = true
		if msg.mode == modeText {
			return m, m.runFTSSearch(query)
		}
		return m, m.runSemanticSearch(query)

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *watchModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Help view: any key closes
	if m.helpView {
		m.helpView = false
		return m, nil
	}

	// Semantic input mode
	if m.semanticFocused {
		switch key {
		case "esc", "ctrl+c":
			m.semanticFocused = false
			m.semanticInput.Blur()
			m.semanticInput.SetValue("")
			if m.searchMode == modeSemantic && m.lastQuery == "" {
				m.searchMode = modeNone
				m.displayed = m.allBookmarks
				m.scores = nil
			}
		case "enter":
			query := strings.TrimSpace(m.semanticInput.Value())
			if query != "" {
				m.semanticFocused = false
				m.semanticInput.Blur()
				m.searchMode = modeSemantic
				m.lastQuery = query
				m.searching = true
				return m, m.runSemanticSearch(query)
			}
		case "up", "down":
			m.semanticFocused = false
			m.semanticInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.semanticInput, cmd = m.semanticInput.Update(msg)
			m.searchSeq++
			seq := m.searchSeq
			query := m.semanticInput.Value()
			debounceCmd := tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
				return debounceMsg{seq: seq, query: query, mode: modeSemantic}
			})
			return m, tea.Batch(cmd, debounceCmd)
		}
		return m, nil
	}

	// Text search input mode
	if m.searchFocused {
		switch key {
		case "esc", "ctrl+c":
			if m.searchInput.Value() != "" {
				m.searchInput.SetValue("")
				m.searchMode = modeNone
				m.lastQuery = ""
				m.searchError = ""
				m.displayed = m.allBookmarks
				m.scores = nil
				m.cursor = 0
				m.viewportOffset = 0
				m.searchSeq++ // cancel pending debounce
			} else {
				m.searchFocused = false
				m.searchInput.Blur()
			}
		case "enter":
			query := strings.TrimSpace(m.searchInput.Value())
			if query != "" {
				m.searchFocused = false
				m.searchInput.Blur()
				m.searchMode = modeText
				m.lastQuery = query
				m.searching = true
				m.searchSeq++ // cancel pending debounce
				return m, m.runFTSSearch(query)
			}
		case "up", "down":
			m.searchFocused = false
			m.searchInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.searchSeq++
			seq := m.searchSeq
			query := m.searchInput.Value()
			debounceCmd := tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
				return debounceMsg{seq: seq, query: query, mode: modeText}
			})
			return m, tea.Batch(cmd, debounceCmd)
		}
		return m, nil
	}

	// Normal mode
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.searchMode != modeNone {
			m.searchMode = modeNone
			m.lastQuery = ""
			m.searchError = ""
			m.displayed = m.allBookmarks
			m.scores = nil
			m.cursor = 0
			m.viewportOffset = 0
			return m, nil
		}
		return m, tea.Quit
	case "/":
		m.searchFocused = true
		m.searchInput.Focus()
		return m, nil
	case "s":
		m.semanticFocused = true
		m.semanticInput.Focus()
		return m, nil
	case "r":
		return m, m.loadBookmarks()
	case "o", "enter":
		if m.cursor < len(m.displayed) {
			url := m.displayed[m.cursor].URL
			return m, openURL(url)
		}
	case "h", "?":
		m.helpView = true
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
		}
	case "down", "j":
		if m.cursor < len(m.displayed)-1 {
			m.cursor++
			m.ensureVisible()
		}
	case "pgup", "ctrl+b":
		m.cursor -= m.viewportRows()
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.ensureVisible()
	case "pgdown", "ctrl+f":
		m.cursor += m.viewportRows()
		if m.cursor >= len(m.displayed) {
			m.cursor = len(m.displayed) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.ensureVisible()
	case "home", "g":
		m.cursor = 0
		m.viewportOffset = 0
	case "end", "G":
		m.cursor = max(len(m.displayed)-1, 0)
		m.ensureVisible()
	default:
		if m.sortState.HandleSortKey(m.columns(), key) {
			m.resortDisplayed()
		}
	}
	return m, nil
}

func (m *watchModel) View() tea.View {
	if m.helpView {
		return tea.View{Content: m.renderHelp(), AltScreen: true}
	}

	var b strings.Builder

	// Search bar
	b.WriteString("\n  ")
	var leftContent string
	if m.semanticFocused {
		leftContent = wSemanticStyle.Render("Semantic: ") + m.semanticInput.View()
	} else if m.searchFocused {
		leftContent = wSearchStyle.Render("Search: ") + m.searchInput.View()
	} else if m.searchError != "" {
		leftContent = wErrorStyle.Render(m.searchError)
	} else if m.searchMode == modeSemantic {
		leftContent = wSemanticStyle.Render("Semantic: [" + m.lastQuery + "]")
	} else if m.searchMode == modeText {
		leftContent = wSearchStyle.Render("Search: [" + m.lastQuery + "]")
	} else {
		leftContent = wHelpStyle.Render("/ search  s semantic")
	}
	b.WriteString(leftContent)

	// Count
	var countStr string
	if len(m.displayed) != len(m.allBookmarks) {
		countStr = fmt.Sprintf("  [%d of %d]", len(m.displayed), len(m.allBookmarks))
	} else if len(m.allBookmarks) > 0 {
		countStr = fmt.Sprintf("  [%d bookmarks]", len(m.allBookmarks))
	}
	b.WriteString(wHelpStyle.Render(countStr))

	// Right-aligned "searching..." indicator
	if m.searching {
		indicator := "searching..."
		renderedCount := wHelpStyle.Render(countStr)
		usedWidth := 2 + lipgloss.Width(leftContent) + lipgloss.Width(renderedCount)
		gap := m.width - usedWidth - len(indicator) - 1
		if gap < 2 {
			gap = 2
		}
		b.WriteString(strings.Repeat(" ", gap))
		style := wSearchStyle
		if m.searchMode == modeSemantic {
			style = wSemanticStyle
		}
		b.WriteString(style.Render(indicator))
	}
	b.WriteString("\n\n")

	// Empty state
	if len(m.displayed) == 0 {
		if len(m.allBookmarks) == 0 {
			b.WriteString("  No bookmarks. Run 'bm import' first.\n")
		} else {
			b.WriteString("  No results.\n")
		}
		b.WriteString("\n")
		b.WriteString(wHelpStyle.Render("  / search  s semantic  r refresh  q quit"))
		b.WriteString("\n")
		return tea.View{Content: b.String(), AltScreen: true}
	}

	// Build table
	tableWidth := max(m.width-3, 60)
	cols := m.columns()
	tbl := table.New(cols...)
	tbl.Padding = 3
	tbl.SetTerminalWidth(tableWidth)
	tbl.HeaderStyle = wHeaderStyle
	tbl.SelectedStyle = wSelectedStyle
	tbl.SelectedIndex = m.cursor
	tbl.ViewportOffset = m.viewportOffset
	tbl.ViewportHeight = m.viewportRows()
	tbl.Sort = m.sortState.ToConfig(cols)

	for _, bk := range m.displayed {
		status := " "
		if bk.FetchedAt != "" {
			status = "+"
		}

		cells := []string{status, bk.Title, bk.URL, bk.FolderPath, formatDate(bk.ChromeAddedAt)}
		if m.searchMode == modeSemantic && m.scores != nil {
			score := ""
			if s, ok := m.scores[bk.URL]; ok {
				score = fmt.Sprintf("%.4f", s)
			}
			cells = append(cells, score)
		}
		tbl.AddRow(table.Row{Cells: cells})
	}

	b.WriteString(tbl.RenderWithScroll(&wHelpStyle))
	b.WriteString("\n\n")

	// Footer
	if m.searchMode == modeSemantic {
		b.WriteString(wHelpStyle.Render("  s new search  / text search  o open  esc clear  h help  q quit"))
	} else if m.searchMode == modeText {
		b.WriteString(wHelpStyle.Render("  / new search  s semantic  o open  esc clear  h help  q quit"))
	} else {
		b.WriteString(wHelpStyle.Render("  / search  s semantic  o open  r refresh  h help  q quit"))
	}
	b.WriteString("\n")

	return tea.View{Content: b.String(), AltScreen: true}
}

func (m *watchModel) renderHelp() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(wSearchStyle.Render("  Bookmark Browser - Keyboard Shortcuts"))
	b.WriteString("\n\n")

	b.WriteString(wHeaderStyle.Render("  Navigation"))
	b.WriteString("\n")
	b.WriteString("    up/k       Move cursor up\n")
	b.WriteString("    down/j     Move cursor down\n")
	b.WriteString("    PgUp/^B    Page up\n")
	b.WriteString("    PgDn/^F    Page down\n")
	b.WriteString("    g/Home     Go to first\n")
	b.WriteString("    G/End      Go to last\n")
	b.WriteString("    o/enter    Open URL in browser\n")
	b.WriteString("    q/esc      Quit\n")
	b.WriteString("\n")

	b.WriteString(wHeaderStyle.Render("  Search"))
	b.WriteString("\n")
	b.WriteString("    /          Start text search (FTS5)\n")
	b.WriteString("    s          Start semantic search (requires Ollama)\n")
	b.WriteString("    enter      Submit search\n")
	b.WriteString("    esc        Clear search / exit\n")
	b.WriteString("\n")

	b.WriteString(wHeaderStyle.Render("  Sort"))
	b.WriteString("\n")
	for _, line := range table.SortableColumnsHelp(m.columns()) {
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")

	b.WriteString(wHeaderStyle.Render("  Other"))
	b.WriteString("\n")
	b.WriteString("    r          Refresh bookmark list\n")
	b.WriteString("    h/?        Show this help\n")
	b.WriteString("\n")

	b.WriteString(wHeaderStyle.Render("  Indicators"))
	b.WriteString("\n")
	b.WriteString("    +          Page content fetched\n")
	b.WriteString("    (space)    Not yet fetched\n")
	b.WriteString("\n")

	b.WriteString(wHelpStyle.Render("  Press any key to close"))
	b.WriteString("\n")
	return b.String()
}

// FTS search command
func (m *watchModel) runFTSSearch(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := db.SearchFTS(query, 200)
		return ftsResultMsg{results: results, query: query, err: err}
	}
}

// Semantic search command
func (m *watchModel) runSemanticSearch(query string) tea.Cmd {
	return func() tea.Msg {
		client := ollama.NewClient("", "")

		queryEmbedding, err := client.EmbedOne(query)
		if err != nil {
			return semanticResultMsg{query: query, err: fmt.Errorf("Ollama error: %w (is Ollama running?)", err)}
		}

		allEmbeddings, err := db.ListAllEmbeddings()
		if err != nil {
			return semanticResultMsg{query: query, err: err}
		}

		if len(allEmbeddings) == 0 {
			return semanticResultMsg{query: query, err: fmt.Errorf("no embeddings found. Run 'bm index' first")}
		}

		bookmarks, err := db.ListBookmarks()
		if err != nil {
			return semanticResultMsg{query: query, err: err}
		}
		byURL := make(map[string]db.Bookmark, len(bookmarks))
		for _, bk := range bookmarks {
			byURL[bk.URL] = bk
		}

		// Best match per URL
		type match struct {
			url        string
			similarity float32
		}
		bestByURL := make(map[string]*match)
		for _, emb := range allEmbeddings {
			vec := ollama.BytesToFloat32(emb.Embedding)
			sim := ollama.CosineSimilarity(queryEmbedding, vec)
			if best, ok := bestByURL[emb.URL]; !ok || sim > best.similarity {
				bestByURL[emb.URL] = &match{url: emb.URL, similarity: sim}
			}
		}

		var results []semanticMatch
		for _, mt := range bestByURL {
			if bk, ok := byURL[mt.url]; ok {
				results = append(results, semanticMatch{bookmark: bk, similarity: mt.similarity})
			}
		}

		// Sort by similarity desc
		for i := 0; i < len(results); i++ {
			for j := i + 1; j < len(results); j++ {
				if results[j].similarity > results[i].similarity {
					results[i], results[j] = results[j], results[i]
				}
			}
		}

		if len(results) > 100 {
			results = results[:100]
		}

		return semanticResultMsg{results: results, query: query}
	}
}

func formatDate(rfc3339 string) string {
	if rfc3339 == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func (m *watchModel) resortDisplayed() {
	key := m.sortState.Key
	dir := m.sortState.Direction
	if key == "" || dir == table.SortNone {
		return
	}

	sort.SliceStable(m.displayed, func(i, j int) bool {
		a, b := m.displayed[i], m.displayed[j]
		var va, vb string
		switch key {
		case "title":
			va, vb = strings.ToLower(a.Title), strings.ToLower(b.Title)
		case "url":
			va, vb = strings.ToLower(a.URL), strings.ToLower(b.URL)
		case "folder":
			va, vb = strings.ToLower(a.FolderPath), strings.ToLower(b.FolderPath)
		case "added":
			va, vb = a.ChromeAddedAt, b.ChromeAddedAt
		default:
			return false
		}
		if dir == table.SortAsc {
			return va < vb
		}
		return va > vb
	})

	m.cursor = 0
	m.viewportOffset = 0
}

func openURL(urlStr string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("open", urlStr).Start()
		return nil
	}
}

// RunWatch starts the interactive watch mode.
func RunWatch(profile string) error {
	m := newWatchModel()
	m.profileFilter = profile
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
