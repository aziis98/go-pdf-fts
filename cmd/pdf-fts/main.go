// IMPORTANT: To enable FTS5 support, build this application using:
// go build -tags sqlite_fts5
//
// For example:
// go build -tags sqlite_fts5 -o pdf-fts-search .
//
// If you are running directly using `go run`, use:
// go run -tags sqlite_fts5 main.go <command> [options]

package main

import (
	"fmt"
	"os"

	"github.com/aziis98/pdf-fts/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
