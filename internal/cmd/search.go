package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
	Long: strings.TrimSpace(`
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

	rows, err := db.Search(queryTerm, limit)
	if err != nil {
		return fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	// Define lipgloss styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("13")).
		Bold(true)

	queryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).
		Bold(true)

	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3"))

	fileStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")).
		Bold(true)

	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)

	resultBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")).
		Padding(0, 1).
		Width(100 - 2)

	snippetStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		Width(100 - 2 - 4)

	countStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).
		Bold(true)

	noResultsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")).
		Bold(true)

	var resultsFound int
	var results []string

	// Header
	fmt.Println(headerStyle.Render("Search Results") + " for " + queryStyle.Render("'"+queryTerm+"'"))
	fmt.Println(separatorStyle.Render(strings.Repeat("═", 100)))

	for rows.Next() {
		resultsFound++
		var path, snippet, lastScannedStr string
		var pageNum int
		if err := rows.Scan(&path, &pageNum, &snippet, &lastScannedStr); err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning search result row: %v\n", err)
			continue
		}

		// Format filename with page number
		base := filepath.Base(path)
		maxBaseLen := 82 // Reduced to make room for page number
		if len(base) > maxBaseLen {
			base = base[:maxBaseLen-3] + "..."
		}

		pageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)

		baseWithPage := fmt.Sprintf("%s %s",
			fileStyle.Render(base),
			pageStyle.Render(fmt.Sprintf("(pag. %d)", pageNum)),
		)

		// Format directory path
		dir := filepath.Dir(path) + "/"
		var pathDisplay string
		if dir != "." {
			maxDirLen := 88
			if len(dir) > maxDirLen {
				dir = "..." + dir[len(dir)-(maxDirLen-3):]
			}
			pathDisplay = pathStyle.Render(dir)
		}

		// Process and highlight snippet
		snippet = strings.ReplaceAll(snippet, "\n", " ")
		snippet = spaceNormalizer.ReplaceAllString(snippet, " ")
		highlightedSnippet := highlightMatches(snippet, queryTerm)

		// Build result content
		resultHeader := fmt.Sprintf("%d. %s", resultsFound, baseWithPage)
		if pathDisplay != "" {
			// Ensure pathDisplay also respects the width constraints indirectly
			// by limiting its content length above.
			resultHeader += "\n   " + pathDisplay
		}

		// Ensure the content fits within the resultBoxStyle width
		// Snippet style already has MaxWidth.
		// Header and dateInfo are typically shorter but their content was also truncated.
		resultContent := lipgloss.JoinVertical(
			lipgloss.Left,
			resultHeader,
			snippetStyle.Render(highlightedSnippet),
		)

		results = append(results, resultBoxStyle.Render(resultContent))
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterating search results: %w", err)
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
