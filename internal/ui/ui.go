package ui

import (
	"fmt"
	"io"
	"log"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aziis98/pdf-fts/internal/database"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	spaceNormalizer = regexp.MustCompile(`\s+`)

	// Lipgloss styles
	docStyle         = lipgloss.NewStyle().Margin(1, 2)
	titleStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true) // Magenta-ish
	filePathStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))           // Dim gray
	snippetStyle     = lipgloss.NewStyle()
	highlightStyle   = lipgloss.NewStyle().Background(lipgloss.Color("220")).Foreground(lipgloss.Color("0")) // Yellow bg, black text
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
	loadingTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
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

type searchResultItem struct {
	Path    string
	PageNum int
	Snippet string
	Query   string // Store query for highlighting
}

func (i searchResultItem) Title() string {
	filename := filepath.Base(i.Path)
	if i.PageNum > 0 {
		return fmt.Sprintf("%s (page %d)", filename, i.PageNum)
	}
	return filename
}                                              // Display filename with page number as title
func (i searchResultItem) Description() string { return i.Path } // Full path as description
func (i searchResultItem) FilterValue() string { return i.Path + " " + i.Snippet }

// Custom item delegate for rendering search results
type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 3 } // Title + Snippet + blank line
func (d itemDelegate) Spacing() int                              { return 1 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(searchResultItem)
	if !ok {
		return
	}

	fileName := titleStyle.Render(item.Title())
	filePath := filePathStyle.Render("  " + item.Path) // Indent path slightly

	// Simple snippet highlighting (case-insensitive for query terms)
	lowerQuery := strings.ToLower(item.Query)
	var highlightedSnippet strings.Builder

	if len(lowerQuery) > 0 { // Only highlight if there's a query
		queryTerms := strings.Fields(lowerQuery) // Split query into terms for individual highlighting

		// Create a map to quickly check if a part of the snippet is a query term
		termMap := make(map[string]bool)
		for _, term := range queryTerms {
			termMap[term] = true
		}

		// Iterate through the snippet, word by word, to highlight query terms
		words := strings.Fields(item.Snippet)           // Split snippet into words
		originalWords := getOriginalWords(item.Snippet) // Get words with original casing

		currentPos := 0
		for i, word := range words {
			// Find the original word corresponding to this potentially lowercased word
			originalWord := ""
			if i < len(originalWords) {
				originalWord = originalWords[i]
			} else {
				originalWord = word // Fallback, should not happen if getOriginalWords is correct
			}

			startIdx := strings.Index(item.Snippet[currentPos:], originalWord) + currentPos
			if startIdx < currentPos { // Should not happen
				highlightedSnippet.WriteString(originalWord + " ")
				currentPos += len(originalWord) + 1
				continue
			}

			// Append text before the current word
			if startIdx > currentPos {
				highlightedSnippet.WriteString(item.Snippet[currentPos:startIdx])
			}

			if termMap[strings.ToLower(word)] {
				highlightedSnippet.WriteString(highlightStyle.Render(originalWord))
			} else {
				highlightedSnippet.WriteString(originalWord)
			}
			highlightedSnippet.WriteString(" ") // Add space after word
			currentPos = startIdx + len(originalWord)
		}
		// Append any remaining part of the snippet
		if currentPos < len(item.Snippet) {
			highlightedSnippet.WriteString(item.Snippet[currentPos:])
		}

	} else {
		highlightedSnippet.WriteString(item.Snippet) // No query, no highlighting
	}

	str := fmt.Sprintf("%s\n%s\n  %s", fileName, filePath, snippetStyle.Render(highlightedSnippet.String()))

	fmt.Fprint(w, docStyle.Render(str))
}

// getOriginalWords splits a string by spaces while preserving the original casing of the words.
func getOriginalWords(s string) []string {
	var words []string
	var currentWord strings.Builder
	for _, r := range s {
		if r == ' ' {
			if currentWord.Len() > 0 {
				words = append(words, currentWord.String())
				currentWord.Reset()
			}
		} else {
			currentWord.WriteRune(r)
		}
	}
	if currentWord.Len() > 0 {
		words = append(words, currentWord.String())
	}
	return words
}

type liveSearchModel struct {
	textInput textinput.Model
	list      list.Model
	spinner   spinner.Model
	searching bool
	width     int
	height    int
	err       error
	db        *database.DB
	verbose   bool
}

type searchResultsMsg struct {
	items []list.Item
	err   error
}

type searchErrorMsg struct{ err error }

func (u *UI) initialLiveSearchModel() liveSearchModel {
	ti := textinput.New()
	ti.Placeholder = "Search PDFs..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50 // Initial width, will be updated

	// List setup
	delegate := itemDelegate{}
	resultList := list.New([]list.Item{}, delegate, 0, 0) // Width/height set in Update
	resultList.Title = "Search Results"
	resultList.SetShowStatusBar(false)    // We'll manage status ourselves
	resultList.SetFilteringEnabled(false) // We do searching via DB
	resultList.Styles.Title = titleStyle
	resultList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("ctrl+c", "esc"), key.WithHelp("ctrl+c/esc", "quit")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select (NYI)")),
		}
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = loadingTextStyle

	return liveSearchModel{
		textInput: ti,
		list:      resultList,
		spinner:   s,
		searching: false,
		db:        u.db,
		verbose:   u.verbose,
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

		// Update text input width
		m.textInput.Width = msg.Width - 4

		// Update list dimensions
		listHeight := msg.Height - 6 // Leave room for input and help
		m.list.SetSize(msg.Width-4, listHeight)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			// Handle item selection here if needed
			return m, nil
		}

	case searchResultsMsg:
		m.searching = false
		m.list.SetItems(msg.items)
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.err = nil
		}

	case searchErrorMsg:
		m.searching = false
		m.err = msg.err
		m.list.SetItems([]list.Item{})
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
			m.list.SetItems([]list.Item{})
			m.err = nil
		} else {
			m.searching = true
			cmds = append(cmds, m.performSearchCmd(newValue))
		}
	}

	// Update list
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	// Update spinner
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m liveSearchModel) View() string {
	// Search input
	searchBox := fmt.Sprintf("Search: %s", m.textInput.View())

	// Status line
	var status string
	if m.searching {
		status = m.spinner.View() + " Searching..."
	} else if m.err != nil {
		status = fmt.Sprintf("Error: %v", m.err)
	} else {
		itemCount := len(m.list.Items())
		if itemCount == 0 && strings.TrimSpace(m.textInput.Value()) != "" {
			status = "No results found"
		} else if itemCount > 0 {
			status = fmt.Sprintf("Found %d result(s)", itemCount)
		}
	}

	// Help text
	help := helpStyle.Render("Press ctrl+c/esc to quit • Enter to select (NYI)")

	// Combine all elements
	content := fmt.Sprintf("%s\n\n%s\n\n%s\n%s",
		searchBox,
		m.list.View(),
		status,
		help)

	return docStyle.Render(content)
}

func (m liveSearchModel) performSearchCmd(queryTerm string) tea.Cmd {
	return func() tea.Msg {
		if m.db == nil {
			return searchErrorMsg{err: fmt.Errorf("database not initialized")}
		}
		items, err := m.queryDBForLiveSearch(queryTerm, 5) // Limit to 20 for TUI
		if err != nil {
			return searchErrorMsg{err: err}
		}
		return searchResultsMsg{items: items}
	}
}

func (m liveSearchModel) queryDBForLiveSearch(queryTerm string, limit int) ([]list.Item, error) {
	if queryTerm == "" {
		return []list.Item{}, nil
	}

	rows, err := m.db.LiveSearch(queryTerm, limit)
	if err != nil {
		return nil, fmt.Errorf("live search query failed: %w", err)
	}
	if rows == nil {
		return []list.Item{}, nil
	}
	defer rows.Close()

	var results []list.Item
	for rows.Next() {
		var path, snippet string
		var pageNum int
		if err := rows.Scan(&path, &pageNum, &snippet); err != nil {
			if m.verbose {
				log.Printf("Error scanning live search result: %v", err)
			}
			continue
		}
		// Further clean snippet from FTS, replace markers with lipgloss styling later
		snippet = strings.ReplaceAll(snippet, "\n", " ")
		snippet = spaceNormalizer.ReplaceAllString(snippet, " ")
		snippet = strings.ReplaceAll(snippet, ">>>", "") // Placeholder, actual highlight in delegate
		snippet = strings.ReplaceAll(snippet, "<<<", "")

		results = append(results, searchResultItem{Path: path, PageNum: pageNum, Snippet: strings.TrimSpace(snippet), Query: queryTerm})
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating live search results: %w", err)
	}
	return results, nil
}
