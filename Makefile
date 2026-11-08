BINARY_NAME=similar-images
GO=go

.PHONY: all build test clean run desktop lint tidy help

all: build

## build: Build the single binary (pure Go, CGO_ENABLED=0)
build:
	CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o $(BINARY_NAME) .

## test: Run unit tests across all packages
test:
	$(GO) test -v ./...

## tidy: Format dependencies and clean up go.mod
tidy:
	$(GO) mod tidy

## lint: Run go vet static analysis
lint:
	$(GO) vet ./...

## run: Run CLI help
run: build
	./$(BINARY_NAME) --help

## desktop: Launch the GPU native desktop UI
desktop: build
	./$(BINARY_NAME) desktop

## clean: Remove build artifacts and temporary files
clean:
	rm -f $(BINARY_NAME)
	rm -rf samples/

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':'
