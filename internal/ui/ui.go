package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aziis98/pdf-fts/internal/database"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	spaceNormalizer = regexp.MustCompile(`\s+`)

	// Lipgloss styles
	docStyle = lipgloss.NewStyle().
			Margin(1, 2, 0, 2)
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("63")).
			Bold(true)
	filePathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
	snippetStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))
	highlightStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("220")).
			Foreground(lipgloss.Color("0"))
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
	loadingTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205"))
	searchBoxStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true).
			Padding(0, 1)
	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3"))
	resultBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("3")).
			Padding(0, 1).
			Width(100 - 2)
	countStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true)
	noResultsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)
)

// UI handles the interactive terminal user interface
type UI struct {
	db      *database.DB
	verbose bool
}

// New creates a new UI handler
func New(db *database.DB, verbose bool) *UI {
	return &UI{
		db:      db,
		verbose: verbose,
	}
}

// HandleLiveSearchCommand starts the interactive live search interface
func (u *UI) HandleLiveSearchCommand() error {
	model := u.initialLiveSearchModel()

	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err := program.Run()
	return err
}

// --- Bubble Tea Model for Live Search ---

type fileResult struct {
	Path  string
	Pages []pageResult
}

type pageResult struct {
	PageNum int
	Snippet string
}

type liveSearchModel struct {
	textInput textinput.Model
	spinner   spinner.Model
	viewport  viewport.Model
	searching bool
	width     int
	height    int
	err       error
	db        *database.DB
	verbose   bool
	results   []fileResult
	query     string
}

type searchResultsMsg struct {
	results []fileResult
	query   string
	err     error
}

type searchErrorMsg struct{ err error }

func (u *UI) initialLiveSearchModel() liveSearchModel {
	ti := textinput.New()
	ti.Placeholder = "Search PDFs..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50 // Initial width, will be updated

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = loadingTextStyle

	vp := viewport.New(78, 10) // Initial size, will be updated
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		PaddingRight(2)

	return liveSearchModel{
		width:     80,
		textInput: ti,
		spinner:   s,
		viewport:  vp,
		searching: false,
		db:        u.db,
		verbose:   u.verbose,
		results:   []fileResult{},
	}
}

func (m liveSearchModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m liveSearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = msg.Width - 20 // Leave some margin

		// Update viewport size - reserve space for header, search box, and help
		headerHeight := 4 // Header + search box + spacing
		footerHeight := 2 // Help text
		availableHeight := m.height - headerHeight - footerHeight
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = max(5, availableHeight)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			// Handle item selection here if needed
			return m, nil
		case "up", "k":
			// Let viewport handle scrolling
			m.viewport.ScrollUp(1)
		case "down", "j":
			// Let viewport handle scrolling
			m.viewport.ScrollDown(1)
		case "home":
			// Go to top
			m.viewport.GotoTop()
		case "end":
			// Go to bottom
			m.viewport.GotoBottom()
		case "pgup":
			// Page up
			m.viewport.PageUp()
		case "pgdn":
			// Page down
			m.viewport.PageDown()
		}

	case searchResultsMsg:
		m.searching = false
		m.results = msg.results
		m.query = msg.query
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.err = nil
		}
		// Update viewport content and reset to top
		m.viewport.SetContent(m.renderResults())
		m.viewport.GotoTop()

	case searchErrorMsg:
		m.searching = false
		m.err = msg.err
		m.results = []fileResult{}
		m.viewport.SetContent("")
		m.viewport.GotoTop()
	}

	// Update text input
	var cmd tea.Cmd
	oldValue := m.textInput.Value()
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	// Trigger search if text changed
	newValue := m.textInput.Value()
	if oldValue != newValue {
		if strings.TrimSpace(newValue) == "" {
			m.results = []fileResult{}
			m.err = nil
			m.viewport.SetContent("")
			m.viewport.GotoTop()
		} else {
			m.searching = true
			cmds = append(cmds, m.performSearchCmd(newValue))
		}
	}

	// Update spinner
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m liveSearchModel) View() string {
	// Search input
	content := searchBoxStyle.Render(fmt.Sprintf("Search: %s", m.textInput.View())) + "\n"

	// Status and results
	if m.searching {
		content += m.spinner.View() + " Searching...\n"
	} else if m.err != nil {
		content += fmt.Sprintf(" Error: %v\n", m.err)
	} else if len(m.results) == 0 && strings.TrimSpace(m.textInput.Value()) != "" {
		content += helpStyle.Render(" No results found.") + "\n"
	} else if len(m.results) > 0 {
		totalResults := len(m.results)
		content += helpStyle.Render(fmt.Sprintf(" Found %d result(s)", totalResults)) + "\n"
	} else {
		content += "\n"
	}

	// Always show viewport (it will be empty if no results)
	content += m.viewport.View()

	// Help text
	content += "\n\n" + helpStyle.Render("Press ctrl+c/esc to quit • Enter to select (NYI)")

	return docStyle.Render(content)
}

func (m liveSearchModel) highlightMatches(snippet, queryTerm string) string {
	highlightStyle := lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{Light: "7", Dark: "8"}).
		Foreground(lipgloss.AdaptiveColor{Light: "0", Dark: "15"}).
		Bold(true)

	// Handle SQLite FTS highlighting markers [HL] and [/HL]
	highlighted := regexp.MustCompile(`\[HL\](.*?)\[/HL\]`).ReplaceAllStringFunc(snippet, func(match string) string {
		text := regexp.MustCompile(`\[HL\](.*?)\[/HL\]`).FindStringSubmatch(match)
		if len(text) > 1 {
			return highlightStyle.Render(text[1])
		}
		return match
	})

	// If no FTS markers, try to highlight the query term manually
	if highlighted == snippet && queryTerm != "" {
		words := strings.Fields(strings.ToLower(queryTerm))
		for _, word := range words {
			if len(word) > 2 {
				re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
				highlighted = re.ReplaceAllStringFunc(highlighted, func(match string) string {
					return highlightStyle.Render(match)
				})
			}
		}
	}

	return highlighted
}

func (m liveSearchModel) performSearchCmd(queryTerm string) tea.Cmd {
	return func() tea.Msg {
		if m.db == nil {
			return searchErrorMsg{err: fmt.Errorf("database not initialized")}
		}
		results, err := m.queryDBForLiveSearch(queryTerm, 10)
		if err != nil {
			return searchErrorMsg{err: err}
		}
		return searchResultsMsg{results: results, query: queryTerm}
	}
}

func (m liveSearchModel) queryDBForLiveSearch(queryTerm string, limit int) ([]fileResult, error) {
	if queryTerm == "" {
		return []fileResult{}, nil
	}

	searchResults, err := m.db.Search(queryTerm, limit)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}

	// Group results by file path while maintaining order
	resultMap := make(map[string]*fileResult)
	var groupedResults []fileResult

	for _, result := range searchResults {
		if _, exists := resultMap[result.Path]; !exists {
			fileRes := fileResult{Path: result.Path, Pages: []pageResult{}}
			groupedResults = append(groupedResults, fileRes)
			resultMap[result.Path] = &groupedResults[len(groupedResults)-1]
		}

		pageRes := pageResult{
			PageNum: result.PageNum,
			Snippet: result.Snippet,
		}
		resultMap[result.Path].Pages = append(resultMap[result.Path].Pages, pageRes)
	}

	return groupedResults, nil
}

func (m liveSearchModel) renderResults() string {
	if len(m.results) == 0 {
		return ""
	}

	var content strings.Builder

	// Render all results (viewport will handle scrolling)
	for _, fileResult := range m.results {
		// Format filename
		base := filepath.Base(fileResult.Path)
		maxBaseLen := 82
		if len(base) > maxBaseLen {
			base = base[:maxBaseLen-3] + "..."
		}

		fileStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Bold(true)

		pathStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)

		pageStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)

		baseWithPath := fmt.Sprintf("%s\n   %s",
			fileStyle.Render(base),
			pathStyle.Render(filepath.Dir(fileResult.Path)+"/"))

		// Combine page snippets
		var pageSnippets []string
		for _, page := range fileResult.Pages {
			snippet := strings.ReplaceAll(page.Snippet, "\n", " ")
			snippet = spaceNormalizer.ReplaceAllString(snippet, " ")
			highlightedSnippet := m.highlightMatches(snippet, m.query)

			formattedSnippet := fmt.Sprintf("%s: %s",
				pageStyle.Render(fmt.Sprintf("Page %d", page.PageNum)),
				highlightedSnippet)
			pageSnippets = append(pageSnippets, formattedSnippet)
		}

		combinedSnippets := strings.Join(pageSnippets, "\n")

		resultContent := lipgloss.JoinVertical(
			lipgloss.Left,
			baseWithPath,
			snippetStyle.Render(combinedSnippets),
		)

		maxWidth := min(100-2, m.width-6)
		resultBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("3")).
			Padding(0, 1).
			Width(maxWidth)

		content.WriteString(resultBox.Render(resultContent) + "\n")
	}

	return content.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
