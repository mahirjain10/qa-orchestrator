# Zenact POC Makefile
# Modernized build and test tools for V2 Architecture

.PHONY: build run test test-unit test-cover clean tidy lint deps check-env help

# Build the TUI application
build:
	go build -v -o ./bin/qa-orchestrator ./apps/tui/cmd/main.go

# Run the TUI application (pass campaign file as argument)
# Usage: make run ARGS="campaigns/sample-autonomous.yaml"
run: build
	./bin/qa-orchestrator $(ARGS)

# Run with sample campaign
run-sample:
	make run ARGS="campaigns/sample-autonomous.yaml"

# Run with guided campaign
run-guided:
	make run ARGS="campaigns/sample-guided.yaml"

# Install dependencies (Go + Playwright)
deps:
	go mod download
	go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps

# Verify environment configuration
check-env:
	@$(if $(LLM_API_KEY), echo "SUCCESS: LLM_API_KEY is configured.", echo "WARNING: LLM_API_KEY is not set. Autonomous mode will fail.")

# Run all tests
test:
	go test -v ./...

# Run only unit tests (skip browser-heavy tests if tagged)
test-unit:
	go test -v -short ./...

# Generate test coverage report
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Linting and Formatting
lint:
	go fmt ./...
	go vet ./...

# Clean up binaries and local data
clean:
	rm -rf ./bin/qa-orchestrator
	rm -rf ./data/*
	rm -rf ./logs/runs/*
	go clean

# Refresh dependencies
tidy:
	go mod tidy

# Show available commands
help:
	@echo "Zenact POC - Available Make targets:"
	@echo ""
	@echo "  build        Build the TUI application"
	@echo "  run          Run with campaign file: make run ARGS='path/to/campaign.yaml'"
	@echo "  run-sample   Run with sample autonomous campaign"
	@echo "  run-guided   Run with sample guided campaign"
	@echo "  deps         Install Go and Playwright dependencies"
	@echo "  check-env    Verify LLM_API_KEY is configured"
	@echo "  test         Run all tests"
	@echo "  test-unit    Run unit tests only"
	@echo "  test-cover   Generate test coverage report"
	@echo "  lint         Format and vet code"
	@echo "  clean        Remove binaries and data"
	@echo "  tidy         Refresh Go dependencies"
	@echo "  help         Show this help message"
