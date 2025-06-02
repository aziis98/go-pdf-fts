package main

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aziis98/pdf-fts/internal/util"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	spaceNormalizer       = regexp.MustCompile(`\s+`)
	sqliteTimestampFormat = "2006-01-02 15:04:05"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for text in PDFs",
	Long: util.Dedent(`
		Search for text content within indexed PDF files using full-text search.
		Returns matching documents with highlighted snippets showing the search context.
	`),
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		limit, _ := cmd.Flags().GetInt("limit")

		return runSearchCommand(query, limit)
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().IntP("limit", "l", 5, "maximum number of results")
}

func runSearchCommand(queryTerm string, limit int) error {
	if cfg.Verbose {
		log.Printf("Search for: '%s', limit: %d", queryTerm, limit)
	}

	searchResults, err := db.Search(queryTerm, limit)
	if err != nil {
		return fmt.Errorf("search query failed: %w", err)
	}

	// Define lipgloss styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("13")).
		Bold(true)

	queryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).
		Bold(true)

	fileStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")).
		Bold(true)

	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)

	resultBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")).
		Padding(0, 1)

	snippetStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250"))

	countStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).
		Bold(true)

	noResultsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")).
		Bold(true)

	pageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Width(5).
		Bold(true)

	// Group results by file path while maintaining order
	type FileResult struct {
		Path  string
		Pages []string
	}

	var groupedResults []FileResult
	resultMap := make(map[string]*FileResult)

	for _, result := range searchResults {
		if _, exists := resultMap[result.Path]; !exists {
			fileResult := FileResult{Path: result.Path, Pages: []string{}}
			groupedResults = append(groupedResults, fileResult)
			resultMap[result.Path] = &groupedResults[len(groupedResults)-1]
		}

		// Process and highlight snippet
		snippet := strings.ReplaceAll(result.Snippet, "\n", " ")
		snippet = spaceNormalizer.ReplaceAllString(snippet, " ")
		highlightedSnippet := highlightMatches(snippet, queryTerm)

		// Format snippet with page number
		resultMap[result.Path].Pages = append(resultMap[result.Path].Pages,
			lipgloss.JoinHorizontal(lipgloss.Left,
				pageStyle.Render(fmt.Sprintf("p.%d", result.PageNum)),
				" ",
				lipgloss.NewStyle().
					Width(90).
					Render(highlightedSnippet),
			),
		)
	}

	var results []string
	var resultsFound int

	// Header
	fmt.Println(headerStyle.Render("Search Results") + " for " + queryStyle.Render("'"+queryTerm+"'"))

	for _, fileResult := range groupedResults {
		resultsFound++

		// Format filename
		base := filepath.Base(fileResult.Path)
		maxBaseLen := 82 // Reduced to make room for page number
		if len(base) > maxBaseLen {
			base = base[:maxBaseLen-3] + "..."
		}

		baseWithPath := fmt.Sprintf(
			"%s\n%s",
			fileStyle.Render(base),
			pathStyle.Render(filepath.Dir(fileResult.Path)+"/"),
		)

		// Combine snippets
		combinedSnippets := strings.Join(fileResult.Pages, "\n\n")

		// Build result content
		resultContent := lipgloss.JoinVertical(
			lipgloss.Left,
			baseWithPath,
			snippetStyle.Render(combinedSnippets),
		)

		results = append(results, resultBoxStyle.Render(resultContent))
	}

	// Display all results
	for _, result := range results {
		fmt.Println(strings.TrimSpace(result))
	}

	// Summary
	if resultsFound == 0 {
		fmt.Println(noResultsStyle.Render("No results found."))
	} else {
		fmt.Println(countStyle.Render(fmt.Sprintf("Found %d result(s).", resultsFound)))
	}
	fmt.Println()

	return nil
}

// highlightMatches enhances the snippet by highlighting search terms
func highlightMatches(snippet, queryTerm string) string {
	// highlightColor := color.New(color.BgHiWhite, color.FgHiBlack, color.Bold)
	highlightStyle := lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{Light: "7", Dark: "8"}).
		Foreground(lipgloss.AdaptiveColor{Light: "0", Dark: "15"}).
		Bold(true)

	// Handle SQLite FTS highlighting markers [HL] and [/HL]
	highlighted := regexp.MustCompile(`\[HL\](.*?)\[/HL\]`).ReplaceAllStringFunc(snippet, func(match string) string {
		// Extract the text between the markers
		text := regexp.MustCompile(`\[HL\](.*?)\[/HL\]`).FindStringSubmatch(match)
		if len(text) > 1 {
			return highlightStyle.Render(text[1])
		}
		return match
	})

	// If no FTS markers, try to highlight the query term manually
	if highlighted == snippet && queryTerm != "" {
		// Split query into words and highlight each
		words := strings.Fields(strings.ToLower(queryTerm))
		for _, word := range words {
			if len(word) > 2 { // Only highlight words longer than 2 characters
				re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
				highlighted = re.ReplaceAllStringFunc(highlighted, func(match string) string {
					return highlightStyle.Render(match)
				})
			}
		}
	}

	return highlighted
}
