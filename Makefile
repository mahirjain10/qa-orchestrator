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
	$(BINARY) --browser real $(CAMPAIGN) $(ARGS)

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
	@$(foreach c,$(CAMPAIGNS),echo "  $(c)";)

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
	@echo "=== Reasoning / Thinking ==="
	@$(if $(LLM_REASONING_EFFORT), echo "OK: LLM_REASONING_EFFORT=$(LLM_REASONING_EFFORT)", echo "INFO: LLM_REASONING_EFFORT not set (model default applies)")
	@$(if $(LLM_THINKING_TYPE), echo "OK: LLM_THINKING_TYPE=$(LLM_THINKING_TYPE)", echo "INFO: LLM_THINKING_TYPE not set (thinking disabled by default)")
	@$(if $(LLM_THINKING_BUDGET), \
		echo "OK: LLM_THINKING_BUDGET=$(LLM_THINKING_BUDGET)", \
		echo "INFO: LLM_THINKING_BUDGET not set (no explicit budget)")
	@$(if $(LLM_HTTP_REFERER), echo "OK: LLM_HTTP_REFERER=$(LLM_HTTP_REFERER)", echo "INFO: LLM_HTTP_REFERER not set")
	@$(if $(LLM_APP_TITLE), echo "OK: LLM_APP_TITLE=$(LLM_APP_TITLE)", echo "INFO: LLM_APP_TITLE not set")
	@echo ""
	@echo "=== Provider Routing (OpenRouter only) ==="
	@$(if $(LLM_PROVIDER_PRIORITY), echo "OK: LLM_PROVIDER_PRIORITY=$(LLM_PROVIDER_PRIORITY)", echo "INFO: LLM_PROVIDER_PRIORITY not set")
	@$(if $(LLM_PROVIDER_ONLY), echo "OK: LLM_PROVIDER_ONLY=$(LLM_PROVIDER_ONLY)", echo "INFO: LLM_PROVIDER_ONLY not set")
	@$(if $(LLM_ALLOW_FALLBACKS), echo "OK: LLM_ALLOW_FALLBACKS=$(LLM_ALLOW_FALLBACKS)", echo "INFO: LLM_ALLOW_FALLBACKS not set")
	@echo ""
	@echo "Usage:"
	@echo "  OpenRouter (GPT):          LLM_API_KEY=sk-or-... LLM_MODEL=openai/gpt-4o-mini make run CAMPAIGN='campaigns/my.yaml'"
	@echo "  OpenRouter (DeepSeek V4):  LLM_API_KEY=sk-or-... LLM_MODEL=deepseek/deepseek-v4-pro LLM_REASONING_EFFORT=high make run CAMPAIGN='campaigns/my.yaml'"
	@echo "  OpenRouter (thinking):     LLM_API_KEY=sk-or-... LLM_MODEL=deepseek/deepseek-v4-pro LLM_THINKING_TYPE=enabled LLM_THINKING_BUDGET=4000 make run"
	@echo "  Gemini:                    LLM_PROVIDER=gemini GEMINI_API_KEY=AIza-... make run CAMPAIGN='campaigns/my.yaml'"

clean:
ifeq ($(OS),Windows_NT)
	-if exist "$(BINARY)" del /Q "$(BINARY)"
	-if exist "./coverage.out" del /Q "./coverage.out"
else
	@rm -f $(BINARY) coverage.out
endif
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
