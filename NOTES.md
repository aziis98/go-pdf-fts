# Notes

## Potential Features and Improvements

-   [ ] Refactor the `internal/` package to a usable library, this would allow
        other projects to use the PDF text extraction and FTS capabilities as
        the following:

    -   [ ] Add a systemd user service with a simple http backend to serve the
            live search results with a small pdf viewer (maybe with
            <https://data-star.dev/>?)

-   [ ] Add benchmark tests for search performance

-   [ ] Implement concurrent PDF processing during scan

-   [ ] Add metadata extraction (title, author, creation date)

-   [ ] Implement OCR support for scanned PDFs

-   [ ] Add progress indicators for large operations

-   [ ] Support for advanced search operators (AND, OR, NOT)

-   [ ] Add configuration file support (YAML/JSON)

-   [ ] Implement search result pagination

-   [ ] Add unit tests for the `pdf-fts` command

**Miscellaneous.**

-   [ ] Optimize database queries with better indexing

-   [ ] Implement auto-update mechanism

## Development

### Creating a New Release

For maintainers, use the release helper script to create new version tags:

```sh
# Create a new minor version (e.g., v0.2 -> v0.3)
./scripts/new_version minor

# Create a new major version (e.g., v0.2 -> v1.0)
./scripts/new_version major
```

The script will:

1. Check for uncommitted changes
2. Generate the next version number based on the latest git tag
3. Create an annotated git tag
4. Optionally push the tag to trigger GitHub Actions release workflow
