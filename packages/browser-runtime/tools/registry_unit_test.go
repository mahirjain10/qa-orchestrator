package tools

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/playwright-community/playwright-go"
	browserruntime "qa-orchestrator/packages/browser-runtime"
)

func TestToolRegistryExecuteAndListToolsWithLocks(t *testing.T) {
	r := &ToolRegistry{
		tools: make(map[string]Tool),
		meta:  make(map[string]ToolInfo),
	}

	r.Register("echo", map[string]ParameterInfo{
		"message": {Type: "string", Description: "Echo value", Required: true},
	}, func(params map[string]any) (any, error) {
		return params["message"], nil
	})

	if _, err := r.Execute("echo", map[string]any{"message": "ok"}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	tools := r.ListTools()
	if len(tools) != 1 || tools[0] != "echo" {
		t.Fatalf("unexpected tools list: %v", tools)
	}
}

func TestToolRegistryConcurrentRegisterRead(t *testing.T) {
	r := &ToolRegistry{
		tools: make(map[string]Tool),
		meta:  make(map[string]ToolInfo),
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "tool"
			r.Register(name, map[string]ParameterInfo{}, func(params map[string]any) (any, error) {
				return idx, nil
			})
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.ListTools()
			_, _ = r.Execute("tool", map[string]any{})
		}()
	}

	wg.Wait()
}

type mockRuntime struct {
	evaluateFn func(expression string) (any, error)
}

func (m *mockRuntime) Navigate(ctx context.Context, url string) error { return nil }
func (m *mockRuntime) Click(ctx context.Context, selector string) error {
	return nil
}
func (m *mockRuntime) Fill(ctx context.Context, selector, value string) error {
	return nil
}
func (m *mockRuntime) WaitForSelector(ctx context.Context, selector string, options *browserruntime.WaitForOptions) error {
	return nil
}
func (m *mockRuntime) TextContent(ctx context.Context, selector string) (string, error) {
	return "", nil
}
func (m *mockRuntime) InnerHTML(ctx context.Context, selector string) (string, error) {
	return "", nil
}
func (m *mockRuntime) Evaluate(ctx context.Context, expression string) (any, error) {
	return m.evaluateFn(expression)
}
func (m *mockRuntime) Screenshot(ctx context.Context, options *browserruntime.ScreenshotOptions) ([]byte, error) {
	return nil, nil
}
func (m *mockRuntime) Page() playwright.Page { return nil }
func (m *mockRuntime) IsRunning() bool       { return true }

func TestObserveUIJS_UsesTreeWalker(t *testing.T) {
	checks := []struct {
		name   string
		needle string
	}{
		{"uses createTreeWalker", "createTreeWalker"},
		{"has isVisible check", "isVisible"},
		{"has isCapturable check", "isCapturable"},
		{"has buildSelector", "buildSelector"},
		{"includes interactive tags", "INPUT"},
		{"includes anchor tag", "'A'"},
		{"includes role check", "getAttribute('role')"},
		{"includes data-test check", "data-test"},
		{"handles class-based selectors", "getAttribute('class')"},
		{"capped at maxElements", "maxElements"},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(observeUIJS, c.needle) {
				t.Errorf("observeUIJS should contain %q", c.needle)
			}
		})
	}
}

func TestObserveUIJS_NoHasText(t *testing.T) {
	if strings.Contains(observeUIJS, "has-text") {
		t.Error("observeUIJS should NOT contain :has-text() selectors; they fail document.querySelector()")
	}
}

func TestObserveUI_404Detection_HeadingFirst(t *testing.T) {
	tests := []struct {
		name       string
		title      string
		heading    string
		wantWarning bool
	}{
		{
			name:        "heading contains 404",
			title:       "Some Page - Test Site",
			heading:     "404 - Page Not Found",
			wantWarning: true,
		},
		{
			name:        "title starts with 404 but heading is clean",
			title:       "404 Page Not Found - Site",
			heading:     "Welcome to our site",
			wantWarning: true,
		},
		{
			name:        "title contains 404 in middle, heading is clean",
			title:       "Learn about 404 errors - Tutorial",
			heading:     "HTTP Status Codes",
			wantWarning: false,
		},
		{
			name:        "no 404 anywhere",
			title:       "Practice Test Automation",
			heading:     "Welcome to Practice Test",
			wantWarning: false,
		},
		{
			name:        "heading contains not found",
			title:       "Test Site",
			heading:     "Page not found",
			wantWarning: true,
		},
		{
			name:        "title contains not found in middle",
			title:       "How to handle not found errors - Guide",
			heading:     "Error Handling Guide",
			wantWarning: false,
		},
		{
			name:        "title starts with not found",
			title:       "Not Found - The requested page does not exist",
			heading:     "Example heading",
			wantWarning: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ToolRegistry{
				tools: make(map[string]Tool),
				meta:  make(map[string]ToolInfo),
			}
			callCount := 0
			mock := &mockRuntime{
				evaluateFn: func(expression string) (any, error) {
					callCount++
					// First call is always observeUIJS → return basic page result
					if callCount == 1 {
						return `{"page_state":"loaded","interactive":[]}`, nil
					}
					// Second call is heading check
					if callCount == 2 {
						return tt.heading, nil
					}
					// Third call (if needed) is title check
					return tt.title, nil
				},
			}
			r.registerObserveUI(mock)

			result, err := r.Execute("observe_ui", nil)
			if err != nil {
				t.Fatalf("Execute(observe_ui) error: %v", err)
			}

			parsed, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("expected map result, got %T", result)
			}

			warning, hasWarning := parsed["warning"]
			if tt.wantWarning && !hasWarning {
				t.Error("expected warning but none was set")
			}
			if !tt.wantWarning && hasWarning {
				t.Errorf("unexpected warning: %q", warning)
			}
		})
	}
}

func TestCheckSelectorExists_JSInjectionPrevention(t *testing.T) {
	tests := []struct {
		name     string
		selector string
	}{
		{"simple id", "#username"},
		{"class selector", ".btn-primary"},
		{"double quote attack", `"); alert(1)//`},
		{"backtick injection", "`); eval(malicious)//"},
		{"parentheses attack", ");恶意代码("},
		{"mixed special chars", `#foo"bar'baz`},
		{"attribute selector with quotes", `input[name="test"]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedJS string
			r := &ToolRegistry{
				tools: make(map[string]Tool),
				meta:  make(map[string]ToolInfo),
			}
			mock := &mockRuntime{
				evaluateFn: func(expression string) (any, error) {
					capturedJS = expression
					return `{"exists": false, "error": "unit-test"}`, nil
				},
			}
			_ = r.checkSelectorExists(mock, tc.selector)
			if capturedJS == "" {
				t.Fatal("no JS was executed")
			}
			if !strings.Contains(capturedJS, `((selector) => {`) {
				t.Error("captured JS should contain the selectorExists function")
			}
		})
	}
}

func TestToolRegistryExecute_ValidatesRequiredAndType(t *testing.T) {
	r := &ToolRegistry{
		tools: make(map[string]Tool),
		meta:  make(map[string]ToolInfo),
	}
	r.Register("validate_me", map[string]ParameterInfo{
		"required_text": {Type: "string", Required: true},
		"flag":          {Type: "bool", Required: false},
	}, func(params map[string]any) (any, error) {
		return "ok", nil
	})

	if _, err := r.Execute("validate_me", map[string]any{}); err == nil {
		t.Fatal("expected required parameter validation error")
	}

	if _, err := r.Execute("validate_me", map[string]any{"required_text": 12}); err == nil {
		t.Fatal("expected type validation error")
	}

	if _, err := r.Execute("validate_me", map[string]any{"required_text": "x", "flag": true}); err != nil {
		t.Fatalf("expected successful execution, got %v", err)
	}
}
