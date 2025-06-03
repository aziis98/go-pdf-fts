# PDF Full-Text Search (pdf-fts)

A fast, lightweight command-line tool to index and search the text content of
PDF files on a per-page basis. Uses SQLite FTS for fast queries and displays
page-specific results in an interactive TUI or via standard terminal output.

## Features

-   Extracts and stores text from each PDF page

-   Full-text search with SQLite FTS5

-   Command-line commands:

    -   `scan`: index or update PDFs (with force re-scan option)

    -   `search`: run ad-hoc queries with configurable result limits

    -   `live`: interactive real-time search TUI

    -   `rebuild-fts`: rebuild the full-text search index

-   Automatic skipping of unchanged files (uses SHA256 hashes)

-   Cross-platform Go implementation

-   Built with SQLite FTS5 and MuPDF for robust PDF text extraction

## Installation

```sh
go install github.com/aziis98/pdf-fts/cmd/pdf-fts@latest
```

Or clone and build (requires SQLite FTS5 support):

```sh
git clone https://github.com/aziis98/pdf-fts.git
cd pdf-fts
make build
```

Note: The build uses the `sqlite_fts5` build tag to enable full-text search
capabilities.

## Usage

### Indexing PDFs

Index your PDF library (scans current directory if no path specified):

```sh
pdf-fts scan /path/to/pdfs
```

Force re-scan of all PDFs (ignores unchanged file detection):

```sh
pdf-fts scan /path/to/pdfs --force
```

### Searching

Search from the terminal with limited results:

```sh
pdf-fts search "query term" --limit 5
```

Search with default settings:

```sh
pdf-fts search "query term"
```

### Interactive Search

Start an interactive search UI with real-time results:

```sh
pdf-fts live
```

### Maintenance

Rebuild the full-text search index (useful for performance optimization):

```sh
pdf-fts rebuild-fts
```

### Global Options

Enable verbose logging for any command:

```sh
pdf-fts scan /path/to/pdfs --verbose
```

## How It Works

1. **Scanning**: The tool extracts text from each PDF page using MuPDF and
   stores it in a SQLite database
2. **Indexing**: SQLite FTS5 creates a full-text search index for fast queries
3. **Change Detection**: Files are hashed (SHA256) to skip re-processing
   unchanged PDFs
4. **Searching**: Queries use SQLite FTS5 for fast, ranked results with context
   snippets

## Database

The tool creates a local SQLite database (`fts.db`) in the current directory to
store:

-   PDF file metadata and hashes

-   Extracted text content per page

-   FTS5 search index

## Requirements

-   Go 1.24+ (for building from source)

-   SQLite with FTS5 support (included in build)

-   MuPDF libraries (bundled in dependencies)

## Examples

### Basic Workflow

```sh
# Scan a directory of PDFs
pdf-fts scan ~/Documents/papers

# Search for specific terms
pdf-fts search "machine learning" --limit 10

# Use interactive mode for exploratory search
pdf-fts live
```

### Advanced Usage

```sh
# Force re-scan with verbose output
pdf-fts scan ~/Documents --force --verbose

# Rebuild index if needed
pdf-fts rebuild-fts --verbose
```

## Project Structure

-   `cmd/pdf-fts/` - Command-line interface implementation

-   `internal/database/` - SQLite database and FTS operations

-   `internal/pdf/` - PDF text extraction using MuPDF

-   `internal/ui/` - Interactive terminal UI components

-   `scripts/` - Utility scripts for development and testing
