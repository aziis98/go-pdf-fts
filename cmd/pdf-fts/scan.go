package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aziis98/pdf-fts/internal/pdf"
	"github.com/aziis98/pdf-fts/internal/util"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan [folders...]",
	Short: "Scan PDFs and extract text",
	Long: util.Dedent(`
		Scan directories for PDF files, extract their text content,
		and store it in the database for full-text search. Only processes
		files that have changed since the last scan unless --force is used.
		
		If no folders are specified, scans the current directory.
	`),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		folders := args
		if len(folders) == 0 {
			folders = []string{"."}
		}

		return runScanCommand(folders, force)
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().BoolP("force", "f", false, "force re-scan of all PDFs")
}

func runScanCommand(folders []string, forceRescan bool) error {
	pdfProcessor := pdf.New(cfg.Verbose)

	if cfg.Verbose {
		log.Printf("Scanning folders: %v (force: %t)", folders, forceRescan)
	}

	// Phase 1: PDF Discovery/Crawl
	fmt.Println("Phase 1: Discovering PDF files...")
	var allPdfFiles []string
	for _, folder := range folders {
		if cfg.Verbose {
			log.Printf("Crawling folder: %s", folder)
		}
		pdfFiles, err := crawlPDFs(folder)
		if err != nil {
			return fmt.Errorf("crawling PDFs in %s: %w", folder, err)
		}
		if cfg.Verbose {
			log.Printf("Found %d PDF files in %s", len(pdfFiles), folder)
		}
		allPdfFiles = append(allPdfFiles, pdfFiles...)
	}

	if len(allPdfFiles) == 0 {
		fmt.Println("No PDF files found.")

		// Show database file size
		if dbSize, err := getDatabaseSize(); err == nil {
			fmt.Printf("Database size: %s\n", formatFileSize(dbSize))
		} else if cfg.Verbose {
			log.Printf("Warning: Could not determine database size: %v", err)
		}

		return nil
	}

	fmt.Printf("Found %d PDF files.\n\n", len(allPdfFiles))

	// Phase 2: Hash Checking
	fmt.Println("Phase 2: Checking file hashes...")
	filesToProcess, err := checkHashes(pdfProcessor, allPdfFiles, forceRescan)
	if err != nil {
		return fmt.Errorf("checking hashes: %w", err)
	}

	if len(filesToProcess) == 0 {
		fmt.Println("All files are up to date. No processing needed.")

		// Show database file size
		if dbSize, err := getDatabaseSize(); err == nil {
			fmt.Printf("Database size: %s\n", formatFileSize(dbSize))
		} else if cfg.Verbose {
			log.Printf("Warning: Could not determine database size: %v", err)
		}

		return nil
	}

	fmt.Printf("%d files need processing.\n\n", len(filesToProcess))

	// Phase 3: PDF Processing
	fmt.Println("Phase 3: Processing PDF content...")
	processedCount, err := processPDFs(pdfProcessor, filesToProcess)
	if err != nil {
		return fmt.Errorf("processing PDFs: %w", err)
	}

	fmt.Printf("\nScan completed. Processed %d PDFs, updated %d entries.\n", len(allPdfFiles), processedCount)

	// Show database file size
	if dbSize, err := getDatabaseSize(); err == nil {
		fmt.Printf("Database size: %s\n", formatFileSize(dbSize))
	} else if cfg.Verbose {
		log.Printf("Warning: Could not determine database size: %v", err)
	}

	return nil
}

// PDFFileInfo holds information about a PDF file to be processed
type PDFFileInfo struct {
	Path        string
	CurrentHash string
	StoredHash  string
	NeedsUpdate bool
}

// crawlPDFs discovers all PDF files in the given folder
func crawlPDFs(folder string) ([]string, error) {
	var pdfFiles []string

	err := filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if cfg.Verbose {
				log.Printf("Warning: Error accessing %s: %v", path, err)
			}
			return nil // Continue walking
		}

		if d.IsDir() {
			return nil
		}

		if strings.HasSuffix(strings.ToLower(path), ".pdf") {
			pdfFiles = append(pdfFiles, path)
		}

		return nil
	})

	return pdfFiles, err
}

// checkHashes checks which files need to be processed based on hash comparison
func checkHashes(pdfProcessor *pdf.Extractor, pdfFiles []string, forceRescan bool) ([]PDFFileInfo, error) {
	var filesToProcess []PDFFileInfo

	// Create progress bar for hash checking (only if not in verbose mode)
	var bar *progressbar.ProgressBar
	if !cfg.Verbose {
		bar = progressbar.NewOptions(len(pdfFiles),
			progressbar.OptionSetDescription("Checking hashes"),
			progressbar.OptionSetWidth(50),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "=",
				SaucerHead:    ">",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}))
	}

	for i, path := range pdfFiles {
		if cfg.Verbose {
			log.Printf("[%d/%d] Checking hash for: %s", i+1, len(pdfFiles), path)
		}

		// Calculate current file hash
		currentHash, err := pdfProcessor.HashFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to calculate hash for %s: %v\n", path, err)
			if bar != nil {
				bar.Add(1)
			}
			continue
		}

		// Get stored hash from database
		storedHash, err := db.GetStoredHash(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to get stored hash for %s: %v\n", path, err)
			if bar != nil {
				bar.Add(1)
			}
			continue
		}

		needsUpdate := forceRescan || currentHash != storedHash

		if needsUpdate {
			if cfg.Verbose {
				if storedHash == "" {
					log.Printf("File is new, will be processed: %s", path)
				} else if forceRescan {
					log.Printf("Force rescan enabled, will process: %s", path)
				} else {
					log.Printf("File hash changed (stored: %s, current: %s), will be processed: %s",
						storedHash[:min(8, len(storedHash))],
						currentHash[:min(8, len(currentHash))],
						path)
				}
			}

			filesToProcess = append(filesToProcess, PDFFileInfo{
				Path:        path,
				CurrentHash: currentHash,
				StoredHash:  storedHash,
				NeedsUpdate: true,
			})
		} else {
			if cfg.Verbose {
				log.Printf("File up to date (hash: %s): %s", currentHash[:min(8, len(currentHash))], path)
			}
		}

		if bar != nil {
			bar.Add(1)
		}
	}

	if bar != nil {
		fmt.Println() // New line after progress bar
	}
	return filesToProcess, nil
}

// processPDFs processes the PDF content for files that need updating
func processPDFs(pdfProcessor *pdf.Extractor, filesToProcess []PDFFileInfo) (int, error) {
	processedCount := 0

	// Create progress bar for PDF processing (only if not in verbose mode)
	var bar *progressbar.ProgressBar
	if !cfg.Verbose {
		bar = progressbar.NewOptions(len(filesToProcess),
			progressbar.OptionSetDescription("Processing PDFs"),
			progressbar.OptionSetWidth(50),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "=",
				SaucerHead:    ">",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}))
	}

	for i, fileInfo := range filesToProcess {
		if cfg.Verbose {
			log.Printf("[%d/%d] Processing PDF content: %s", i+1, len(filesToProcess), fileInfo.Path)
		}

		// Extract text content per page
		pageContents, err := pdfProcessor.ExtractPagesText(fileInfo.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to process %s: %v\n", fileInfo.Path, err)
			if bar != nil {
				bar.Add(1)
			}
			continue
		}

		if cfg.Verbose {
			log.Printf("Extracted text from %d pages in: %s", len(pageContents), fileInfo.Path)
		}

		// Update database
		if err := db.UpsertPDFData(fileInfo.Path, fileInfo.CurrentHash, pageContents); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to store data for %s: %v\n", fileInfo.Path, err)
			if bar != nil {
				bar.Add(1)
			}
			continue
		}

		processedCount++
		if cfg.Verbose {
			log.Printf("Successfully updated database entry for: %s", fileInfo.Path)
		}

		if bar != nil {
			bar.Add(1)
		}
	}

	if bar != nil {
		fmt.Println() // New line after progress bar
	}
	return processedCount, nil
}

// getDatabaseSize returns the size of the database file in bytes
func getDatabaseSize() (int64, error) {
	dbPath := cfg.DBPath
	if dbPath == "" {
		return 0, fmt.Errorf("database path not configured")
	}

	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		return 0, err
	}

	return fileInfo.Size(), nil
}

// formatFileSize formats a file size in bytes into a human-readable string
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}
