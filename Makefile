BINARY_NAME := vgrep
PREFIX ?= /usr/local
BINDIR := $(PREFIX)/bin
GO ?= go
GOFLAGS ?=
LDFLAGS ?= -s -w

.PHONY: all build build-all run test test-race test-cover fmt vet lint clean install uninstall health manual help

# Default target
all: build

## build: Build the vgrep binary
build:
	@echo "==> Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) main.go

## build-all: Build binaries for all supported platforms
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64

build-linux-amd64:
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-linux-amd64 main.go

build-linux-arm64:
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-linux-arm64 main.go

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-darwin-amd64 main.go

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-darwin-arm64 main.go

build-windows-amd64:
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-windows-amd64.exe main.go

## run: Build and run with arguments (usage: make run ARGS="pattern")
run: build
	./$(BINARY_NAME) $(ARGS)

## test: Run all unit tests
test:
	@echo "==> Running unit tests..."
	$(GO) test -v ./...

## test-race: Run unit tests with race detector
test-race:
	@echo "==> Running tests with race detector..."
	$(GO) test -race -v ./...

## test-cover: Run tests with coverage report
test-cover:
	@echo "==> Running test coverage..."
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

## fmt: Format Go source files
fmt:
	@echo "==> Formatting code..."
	$(GO) fmt ./...

## vet: Analyze code for potential bugs
vet:
	@echo "==> Running go vet..."
	$(GO) vet ./...

## lint: Run golangci-lint if installed
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "==> Running golangci-lint..."; \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found, skipping (install via https://golangci-lint.run/)"; \
	fi

## health: Run environment health check
health: build
	./$(BINARY_NAME) --health

## install: Install binary to $(BINDIR)
install: build
	@echo "==> Installing $(BINARY_NAME) to $(BINDIR)..."
	install -d $(BINDIR)
	install -m 755 $(BINARY_NAME) $(BINDIR)/$(BINARY_NAME)

## uninstall: Remove binary from $(BINDIR)
uninstall:
	@echo "==> Removing $(BINARY_NAME) from $(BINDIR)..."
	rm -f $(BINDIR)/$(BINARY_NAME)

## manual: Compile LaTeX documentation to manual.pdf
manual:
	@if command -v pdflatex >/dev/null 2>&1; then \
		echo "==> Compiling manual.tex with pdflatex..."; \
		pdflatex -interaction=nonstopmode manual.tex >/dev/null && \
		pdflatex -interaction=nonstopmode manual.tex >/dev/null; \
		echo "==> Generated manual.pdf"; \
	elif command -v pdftex >/dev/null 2>&1; then \
		echo "==> Compiling manual.tex with pdftex..."; \
		pdftex -interaction=nonstopmode manual.tex >/dev/null && \
		pdftex -interaction=nonstopmode manual.tex >/dev/null; \
		echo "==> Generated manual.pdf"; \
	else \
		echo "Neither pdflatex nor pdftex found, skipping manual compilation (install via texlive/mactex)"; \
	fi

## clean: Remove build artifacts, coverage, and LaTeX files
clean:
	@echo "==> Cleaning build artifacts..."
	rm -f $(BINARY_NAME) coverage.out
	rm -f manual.aux manual.log manual.out manual.toc manual.pdf

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | awk -F ':' '{printf "  %-12s %s\n", $$1, $$2}'
