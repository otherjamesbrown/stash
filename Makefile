# Stash Makefile

.PHONY: build test clean dev-reset lint

# Build the stash binary
build:
	go build -o stash ./cmd/stash

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-v:
	go test -v ./...

# Run tests with coverage
test-cover:
	go test -cover ./...

# Run linter
lint:
	golangci-lint run || go vet ./...

# Clean build artifacts
clean:
	rm -f stash
	rm -f coverage.out

# Build and create test instance with sample data
# This drops the existing 'world' stash, rebuilds, and populates with countries/cities
dev-reset:
	./scripts/dev-reset.sh

# Quick rebuild and test
dev: build dev-reset

# Full quality check
check: lint test build
