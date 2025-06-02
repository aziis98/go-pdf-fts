package cmd

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/aziis98/pdf-fts/internal/config"
	"github.com/aziis98/pdf-fts/internal/database"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

var (
	cfg     *config.Config
	db      *database.DB
	verbose bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pdf-fts",
	Short: "PDF Full-Text Search Tool",
	Long: `A powerful tool for indexing and searching text content within PDF files.
It extracts text from PDFs, stores it in a SQLite database with FTS5 support,
and provides fast full-text search capabilities.

If you are running directly using 'go run', use:
  go run -tags sqlite_fts5 main.go <command> [options]`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize configuration
		cfg = config.New()
		cfg.Verbose = verbose

		// Setup logging
		if cfg.Verbose {
			log.SetFlags(log.Ltime | log.Lshortfile)
			log.SetOutput(os.Stderr)
		} else {
			log.SetFlags(0)
			log.SetOutput(io.Discard)
		}

		// Find or create database path based on command
		cmdName := cmd.Name()
		switch cmdName {
		case "scan":
			// Scan can create a new database if none exists
			if err := cfg.FindOrCreateDBPath(); err != nil {
				return fmt.Errorf("finding or creating database path: %w", err)
			}
		case "search", "live", "rebuild-fts":
			// These commands require an existing database
			if err := cfg.FindExistingDBPath(); err != nil {
				return fmt.Errorf("no database found - please run 'scan' first to create and populate the database")
			}
		default:
			// Default behavior: try to find existing, create if not found
			if err := cfg.FindOrCreateDBPath(); err != nil {
				return fmt.Errorf("finding database path: %w", err)
			}
		}

		if cfg.Verbose {
			log.Printf("Using database at: %s", cfg.DBPath)
		}

		// Initialize database
		var err error
		db, err = database.New(cfg.DBPath, cfg.Verbose)
		if err != nil {
			return fmt.Errorf("initializing database: %w", err)
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if db != nil {
			return db.Close()
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}
