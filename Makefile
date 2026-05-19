.PHONY: setup test lint build release

## setup: Download all Go module dependencies
setup:
	go mod download

## test: Run the test suite with race detection
test:
	go test -race ./pkg/...

## lint: Run golangci-lint across the codebase
lint:
	golangci-lint run ./...

## build: Build the astra server binary
build:
	go build ./cmd/astra/...

## release: Instructions to tag and trigger the CI release job
release:
	@echo ""
	@echo "  To publish a new release:"
	@echo "    1. git tag -a vX.Y.Z -m 'Release vX.Y.Z'"
	@echo "    2. git push origin main --tags"
	@echo ""
	@echo "  The CI pipeline will then run GoReleaser automatically."
	@echo ""
