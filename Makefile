BINARY_NAME=pdf-fts
BUILD_TAGS=sqlite_fts5

.PHONY: build
build:
	go build -tags $(BUILD_TAGS) -o $(BINARY_NAME) ./cmd/pdf-fts

.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	go clean

.PHONY: test
test:
	go test -tags $(BUILD_TAGS) ./...

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build      - Build the application with debug info"
	@echo "  clean      - Clean build artifacts"
	@echo "  test       - Run tests"
	@echo "  help       - Show this help message"
