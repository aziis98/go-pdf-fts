# PDF Full-Text Search (pdf-fts)

A fast, lightweight command-line tool to index and search the text content of
PDF files on a per-page basis. Uses SQLite FTS for fast queries and displays
page-specific results in an interactive TUI or via standard terminal output.

## Features

-   Extracts and stores text from each PDF page

-   Full-text search with SQLite FTS5

-   Command-line commands:

    -   `scan`: index or update PDFs

    -   `search`: run ad-hoc queries and see page numbers

    -   `live`: interactive real-time search UI

-   Automatic skipping of unchanged files (uses SHA256 hashes)

-   Cross-platform Go implementation with no external dependencies

## Installation

```sh
go install github.com/aziis98/pdf-fts/cmd/pdf-fts@latest
```

Or clone and build:

```sh
git clone https://github.com/aziis98/pdf-fts.git
cd pdf-fts
make build
```

## Usage

Index your PDF library:

```sh
pdf-fts scan /path/to/pdfs
```

Search from the terminal:

```sh
pdf-fts search "query term" -l 5
```

Start an interactive search UI:

```sh
pdf-fts live
```
