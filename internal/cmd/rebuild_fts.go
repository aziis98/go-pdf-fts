package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rebuildFtsCmd = &cobra.Command{
	Use:   "rebuild-fts",
	Short: "Rebuild the full-text search index",
	Long: `Rebuild the FTS5 full-text search index from the existing data.
This can help improve search performance and fix any index corruption issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Rebuilding Full-Text Search index...")
		return db.RebuildFTS()
	},
}

func init() {
	rootCmd.AddCommand(rebuildFtsCmd)
}
