# qa-orchestrator Makefile

.PHONY: build run run-sample run-guided run-real run-real-sample resume test test-short test-cover test-campaigns lint vet fmt tidy deps check-env clean verify help install-playwright run-all run-real-all list-campaigns $(addprefix run-,$(CAMPAIGNS)) $(addprefix run-real-,$(CAMPAIGNS))

ifeq ($(OS),Windows_NT)
    BINARY := ./bin/qa-orchestrator.exe
else
    BINARY := ./bin/qa-orchestrator
endif
APP := ./apps/tui/cmd
CAMPAIGN ?= campaigns/sample-autonomous.yaml

# All available campaigns (derived from files in campaigns/ directory)
CAMPAIGNS := $(patsubst campaigns/%.yaml,%,$(wildcard campaigns/*.yaml))

build:
	go build -v -o $(BINARY) $(APP)

run: build
	$(BINARY) $(ARGS)

run-sample:
	$(MAKE) run ARGS="campaigns/sample-autonomous.yaml"

run-guided:
	$(MAKE) run ARGS="campaigns/sample-guided.yaml"

run-real: build
	$(BROWSER_MODE) $(BINARY) --browser real $(CAMPAIGN) $(ARGS)

# Generate per-campaign targets for mock and real modes
define gen-campaign-targets
run-$(1):
	$$(MAKE) run ARGS="campaigns/$(1).yaml"

run-real-$(1):
	$$(MAKE) run-real CAMPAIGN="campaigns/$(1).yaml"

endef

$(foreach c,$(CAMPAIGNS),$(eval $(call gen-campaign-targets,$(c))))

# Aggregate targets — run all campaigns sequentially
.PHONY: run-all run-real-all list-campaigns
run-all: $(addprefix run-,$(CAMPAIGNS))
run-real-all: $(addprefix run-real-,$(CAMPAIGNS))

# Legacy convenience aliases (kept for backward compatibility)
run-sample: run-sample-autonomous
run-guided: run-sample-guided
run-real-sample: run-real-sample-autonomous
run-real-large: run-real-large-parallel

list-campaigns:
	@echo "Available campaigns (use 'run-<name>' or 'run-real-<name>'):"
	@for c in $(CAMPAIGNS); do echo "  $$c"; done

resume: build
	$(BINARY) --resume $(RUN_ID) $(CAMPAIGN)

install-playwright:
	go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps

test:
	go test -v ./...

test-short:
	go test -v -short ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-campaigns:
	go test -v ./packages/agents/engine/ -run 'TestAllCampaigns'

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet

verify: test build

deps:
	go mod download
	$(MAKE) install-playwright

tidy:
	go mod tidy

check-env:
	@echo "=== LLM Configuration ==="
	@$(if $(LLM_PROVIDER), echo "OK: LLM_PROVIDER=$(LLM_PROVIDER)", echo "INFO: LLM_PROVIDER not set (default: auto)")
	@$(if $(LLM_API_KEY), echo "OK: LLM_API_KEY is configured.", echo "WARN: LLM_API_KEY is not set.")
	@$(if $(LLM_MODEL), echo "OK: LLM_MODEL=$(LLM_MODEL)", echo "INFO: LLM_MODEL not set (default: openai/gpt-4o-mini)")
	@$(if $(GEMINI_API_KEY), echo "OK: GEMINI_API_KEY is configured.", echo "INFO: GEMINI_API_KEY not set.")
	@$(if $(GEMINI_MODEL), echo "OK: GEMINI_MODEL=$(GEMINI_MODEL)", echo "INFO: GEMINI_MODEL not set (default: gemini-2.0-flash)")
	@echo ""
	@echo "Usage:"
	@echo "  OpenRouter:  LLM_API_KEY=xxx LLM_MODEL=openai/gpt-4o make run CAMPAIGN='campaigns/my.yaml'"
	@echo "  Gemini:      LLM_PROVIDER=gemini GEMINI_API_KEY=xxx make run CAMPAIGN='campaigns/my.yaml'"

clean:
	powershell -NoProfile -Command "if (Test-Path '$(BINARY)') { Remove-Item '$(BINARY)' -Force }"
	powershell -NoProfile -Command "if (Test-Path './coverage.out') { Remove-Item './coverage.out' -Force }"
	go clean

help:
	@echo "qa-orchestrator - Available Make targets:"
	@echo ""
	@echo "  build              Build TUI binary to bin/qa-orchestrator"
	@echo "  run                Run with campaign: make run ARGS='campaigns/sample-guided.yaml'"
	@echo "  run-<name>         Run campaign in mock browser: make run-autonomous-campaign-01"
	@echo "  run-real-<name>    Run campaign in real browser: make run-real-large-parallel"
	@echo "  run-all            Run all $(words $(CAMPAIGNS)) campaigns sequentially (mock)"
	@echo "  run-real-all       Run all $(words $(CAMPAIGNS)) campaigns sequentially (real)"
	@echo "  run-sample         Alias for run-sample-autonomous"
	@echo "  run-guided         Alias for run-sample-guided"
	@echo "  run-real-sample    Alias for run-real-sample-autonomous"
	@echo "  run-real-large     Alias for run-real-large-parallel"
	@echo "  list-campaigns     Show all available campaign names"
	@echo "  install-playwright Install Playwright browsers (required for --browser real)"
	@echo "  resume             Resume a session: make resume RUN_ID=run_xxx CAMPAIGN=campaigns/sample-guided.yaml"
	@echo "  test               Run all tests"
	@echo "  test-short         Run short tests only"
	@echo "  test-cover         Generate coverage summary"
	@echo "  test-campaigns     Validate all $(words $(CAMPAIGNS)) campaign YAMLs parse correctly"
	@echo "  fmt                Run go fmt"
	@echo "  vet                Run go vet"
	@echo "  lint               Run fmt + vet"
	@echo "  verify             Run tests then build"
	@echo "  deps               Download Go deps + install Playwright"
	@echo "  tidy               Run go mod tidy"
	@echo "  check-env          Show LLM/Gemini configuration status"
	@echo "  clean              Remove build/coverage artifacts"
	@echo "  help               Show this help"
