# qa-orchestrator Makefile

.PHONY: build run run-sample run-guided test test-short test-cover lint vet fmt tidy deps check-env clean verify help

BINARY := ./bin/qa-orchestrator
APP := ./apps/tui/cmd/main.go

build:
	go build -v -o $(BINARY) $(APP)

run: build
	$(BINARY) $(ARGS)

run-sample:
	$(MAKE) run ARGS="campaigns/sample-autonomous.yaml"

run-guided:
	$(MAKE) run ARGS="campaigns/sample-guided.yaml"

test:
	go test -v ./...

test-short:
	go test -v -short ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet

verify: test build

deps:
	go mod download
	go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps

tidy:
	go mod tidy

check-env:
	@$(if $(LLM_API_KEY), echo "OK: LLM_API_KEY is configured.", echo "WARN: LLM_API_KEY is not set. Autonomous mode will fail.")
	@$(if $(LLM_MODEL), echo "OK: LLM_MODEL is configured.", echo "WARN: LLM_MODEL is not set. Autonomous mode will fail.")

clean:
	powershell -NoProfile -Command "if (Test-Path '$(BINARY)') { Remove-Item '$(BINARY)' -Force }"
	powershell -NoProfile -Command "if (Test-Path './coverage.out') { Remove-Item './coverage.out' -Force }"
	go clean

help:
	@echo "qa-orchestrator - Available Make targets:"
	@echo ""
	@echo "  build       Build TUI binary"
	@echo "  run         Run with campaign: make run ARGS='campaigns/sample-guided.yaml'"
	@echo "  run-sample  Run sample autonomous campaign"
	@echo "  run-guided  Run sample guided campaign"
	@echo "  test        Run all tests"
	@echo "  test-short  Run short tests"
	@echo "  test-cover  Generate coverage summary"
	@echo "  fmt         Run go fmt"
	@echo "  vet         Run go vet"
	@echo "  lint        Run fmt + vet"
	@echo "  verify      Run tests and build"
	@echo "  deps        Download Go deps + install Playwright"
	@echo "  tidy        Run go mod tidy"
	@echo "  check-env   Validate LLM env vars"
	@echo "  clean       Remove build/coverage artifacts"
	@echo "  help        Show this help"
