package cmd

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aziis98/pdf-fts/internal/pdf"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan PDFs and extract text",
	Long: `Scan a directory for PDF files, extract their text content,
and store it in the database for full-text search. Only processes
files that have changed since the last scan unless --force is used.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		folder, _ := cmd.Flags().GetString("folder")
		force, _ := cmd.Flags().GetBool("force")

		return runScanCommand(folder, force)
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringP("folder", "f", ".", "folder to scan for PDFs")
	scanCmd.Flags().Bool("force", false, "force re-scan of all PDFs")
}

func runScanCommand(folder string, forceRescan bool) error {
	pdfProcessor := pdf.New(cfg.Verbose)

	if cfg.Verbose {
		log.Printf("Scanning folder: %s (force: %t)", folder, forceRescan)
	}

	// Phase 1: PDF Discovery/Crawl
	fmt.Println("Phase 1: Discovering PDF files...")
	pdfFiles, err := crawlPDFs(folder)
	if err != nil {
		return fmt.Errorf("crawling PDFs: %w", err)
	}

	if len(pdfFiles) == 0 {
		fmt.Println("No PDF files found.")
		return nil
	}

	fmt.Printf("Found %d PDF files.\n\n", len(pdfFiles))

	// Phase 2: Hash Checking
	fmt.Println("Phase 2: Checking file hashes...")
	filesToProcess, err := checkHashes(pdfProcessor, pdfFiles, forceRescan)
	if err != nil {
		return fmt.Errorf("checking hashes: %w", err)
	}

	if len(filesToProcess) == 0 {
		fmt.Println("All files are up to date. No processing needed.")
		return nil
	}

	fmt.Printf("%d files need processing.\n\n", len(filesToProcess))

	// Phase 3: PDF Processing
	fmt.Println("Phase 3: Processing PDF content...")
	processedCount, err := processPDFs(pdfProcessor, filesToProcess)
	if err != nil {
		return fmt.Errorf("processing PDFs: %w", err)
	}

	fmt.Printf("\nScan completed. Processed %d PDFs, updated %d entries.\n", len(pdfFiles), processedCount)
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

	// Create progress bar for hash checking
	bar := progressbar.NewOptions(len(pdfFiles),
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

	for _, path := range pdfFiles {
		// Calculate current file hash
		currentHash, err := pdfProcessor.HashFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to calculate hash for %s: %v\n", path, err)
			bar.Add(1)
			continue
		}

		// Get stored hash from database
		storedHash, err := db.GetStoredHash(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to get stored hash for %s: %v\n", path, err)
			bar.Add(1)
			continue
		}

		needsUpdate := forceRescan || currentHash != storedHash

		if needsUpdate {
			if cfg.Verbose {
				log.Printf("File needs processing: %s", path)
			}

			filesToProcess = append(filesToProcess, PDFFileInfo{
				Path:        path,
				CurrentHash: currentHash,
				StoredHash:  storedHash,
				NeedsUpdate: true,
			})
		} else {
			if cfg.Verbose {
				log.Printf("File up to date: %s", path)
			}
		}

		bar.Add(1)
	}

	fmt.Println() // New line after progress bar
	return filesToProcess, nil
}

// processPDFs processes the PDF content for files that need updating
func processPDFs(pdfProcessor *pdf.Extractor, filesToProcess []PDFFileInfo) (int, error) {
	processedCount := 0

	// Create progress bar for PDF processing
	bar := progressbar.NewOptions(len(filesToProcess),
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

	for _, fileInfo := range filesToProcess {
		if cfg.Verbose {
			log.Printf("Processing PDF content: %s", fileInfo.Path)
		}

		// Extract text content per page
		pageContents, err := pdfProcessor.ExtractPagesText(fileInfo.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to process %s: %v\n", fileInfo.Path, err)
			bar.Add(1)
			continue
		}

		// Update database
		if err := db.UpsertPDFData(fileInfo.Path, fileInfo.CurrentHash, pageContents); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to store data for %s: %v\n", fileInfo.Path, err)
			bar.Add(1)
			continue
		}

		processedCount++
		if cfg.Verbose {
			log.Printf("Updated database entry for: %s", fileInfo.Path)
		}

		bar.Add(1)
	}

	fmt.Println() // New line after progress bar
	return processedCount, nil
}
