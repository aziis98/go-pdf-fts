package main

import (
	"github.com/aziis98/pdf-fts/internal/ui"
	"github.com/aziis98/pdf-fts/internal/util"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var liveCmd = &cobra.Command{
	Use:   "live",
	Short: "Interactive live search",
	Long: util.Dedent(`
		Start an interactive terminal UI for live searching through
		indexed PDF content. Provides real-time search results as you type.
	`),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Enable tea logging only when the TUI is actually used
		if cfg.Verbose {
			f, _ := tea.LogToFile("debug.log", "debug")
			defer f.Close()
		}

		uiHandler := ui.New(db, cfg.Verbose)
		return uiHandler.HandleLiveSearchCommand()
	},
}

func init() {
	rootCmd.AddCommand(liveCmd)
}
