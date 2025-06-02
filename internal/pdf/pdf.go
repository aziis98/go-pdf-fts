package pdf

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/gen2brain/go-fitz"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	spaceNormalizer = regexp.MustCompile(`\s+`)
)

// Extractor handles PDF text extraction operations
type Extractor struct {
	verbose bool
}

// New creates a new PDF extractor
func New(verbose bool) *Extractor {
	return &Extractor{
		verbose: verbose,
	}
}

// HashFile calculates the SHA1 hash of a file
func (e *Extractor) HashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// openPDFReader opens a PDF file and returns a fitz document
func (e *Extractor) openPDFReader(pdfPath string) (*fitz.Document, error) {
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("opening PDF file %s: %w", pdfPath, err)
	}

	return doc, nil
}

// logWarning logs a warning message if verbose mode is enabled
func (e *Extractor) logWarning(format string, args ...interface{}) {
	if e.verbose {
		fmt.Printf("Warning: "+format+"\n", args...)
	}
}

// extractPageText extracts text from a single page using go-fitz
func (e *Extractor) extractPageText(doc *fitz.Document, pageNum int, pdfPath string) (string, error) {
	text, err := doc.Text(pageNum)
	if err != nil {
		return "", fmt.Errorf("extracting text from page %d of %s: %w", pageNum, pdfPath, err)
	}

	return text, nil
}

// ExtractText extracts text content from a PDF file using github.com/gen2brain/go-fitz
func (e *Extractor) ExtractText(pdfPath string) (string, error) {
	doc, err := e.openPDFReader(pdfPath)
	if err != nil {
		return "", err
	}
	defer doc.Close()

	numPages := doc.NumPage()

	var allText strings.Builder
	for pageIndex := 0; pageIndex < numPages; pageIndex++ {
		text, err := e.extractPageText(doc, pageIndex, pdfPath)
		if err != nil {
			// Log error but continue to extract from other pages if possible
			e.logWarning("could not extract text from page %d of %s: %v", pageIndex+1, pdfPath, err)
			continue
		}
		allText.WriteString(text)
		allText.WriteString("\n") // Add a newline between pages
	}

	return allText.String(), nil
}

var removeDiacritics = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

func normalizeUnicode(s string) string {
	result, _, err := transform.String(removeDiacritics, s)
	if err != nil {
		panic(fmt.Sprintf("normalizing string failed: %v", err))
	}

	return result
}

// CleanText normalizes and cleans extracted text
func (e *Extractor) CleanText(text string) string {
	// Normalize Unicode
	text = normalizeUnicode(text)

	// Replace multiple whitespace with single space
	text = spaceNormalizer.ReplaceAllString(text, " ")

	// Trim whitespace
	text = strings.TrimSpace(text)

	return text
}

// ExtractAllText extracts and cleans text from a PDF file
func (e *Extractor) ExtractAllText(filePath string) (string, error) {
	// Extract text
	rawText, err := e.ExtractText(filePath)
	if err != nil {
		return "", fmt.Errorf("extracting text from %s: %w", filePath, err)
	}

	// Clean text
	cleanedText := e.CleanText(rawText)

	return cleanedText, nil
}

// ExtractPagesText extracts text from each page of a PDF and returns a list of cleaned strings.
func (e *Extractor) ExtractPagesText(pdfPath string) ([]string, error) {
	doc, err := e.openPDFReader(pdfPath)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	numPages := doc.NumPage()

	var pagesText []string
	for pageIndex := 0; pageIndex < numPages; pageIndex++ {
		text, err := e.extractPageText(doc, pageIndex, pdfPath)
		if err != nil {
			e.logWarning("could not extract text from page %d of %s: %v", pageIndex+1, pdfPath, err)
			pagesText = append(pagesText, "") // Add empty string for this page
			continue
		}
		pagesText = append(pagesText, e.CleanText(text))
	}

	return pagesText, nil
}
