package tools

import (
	"context"
	"encoding/json"
	"fmt"
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
func (m *mockRuntime) SelectOption(ctx context.Context, selector, value, label string, index *int) error {
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
		name        string
		title       string
		heading     string
		headingErr  error
		titleErr    error
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
		{
			name:        "heading evaluate error skips check",
			title:       "Normal Page",
			heading:     "Welcome",
			headingErr:  fmt.Errorf("page closed"),
			wantWarning: false,
		},
		{
			name:        "title evaluate error skips check",
			title:       "Page Not Found",
			headingErr:  nil,
			titleErr:    fmt.Errorf("page closed"),
			wantWarning: false,
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
						return tt.heading, tt.headingErr
					}
					// Third call (if needed) is title check
					return tt.title, tt.titleErr
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

func TestCheckSelectorExists_ErrorPaths(t *testing.T) {
	tests := []struct {
		name       string
		evaluateFn func(expression string) (any, error)
		wantErr    string
	}{
		{
			name: "js eval failure",
			evaluateFn: func(expression string) (any, error) {
				return nil, fmt.Errorf("eval error")
			},
			wantErr: "selector check eval failed",
		},
		{
			name: "type assertion failure",
			evaluateFn: func(expression string) (any, error) {
				return 42, nil
			},
			wantErr: "unexpected result type",
		},
		{
			name: "json parse failure",
			evaluateFn: func(expression string) (any, error) {
				return `not json`, nil
			},
			wantErr: "failed to parse result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ToolRegistry{
				tools: make(map[string]Tool),
				meta:  make(map[string]ToolInfo),
			}
			mock := &mockRuntime{
				evaluateFn: tt.evaluateFn,
			}
			err := r.checkSelectorExists(mock, "#test")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
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

func TestSelectOption_RequiresSingleSelectionCriterion(t *testing.T) {
	r := &ToolRegistry{
		tools: make(map[string]Tool),
		meta:  make(map[string]ToolInfo),
	}
	mock := &mockRuntime{
		evaluateFn: func(expression string) (any, error) {
			return `{"exists": true, "visible": true}`, nil
		},
	}
	r.registerSelectOption(mock)

	if _, err := r.Execute("select_option", map[string]any{"selector": "#sort"}); err == nil {
		t.Fatal("expected error when no selection criterion is provided")
	}
	if _, err := r.Execute("select_option", map[string]any{"selector": "#sort", "value": "name", "label": "Name"}); err == nil {
		t.Fatal("expected error when multiple selection criteria are provided")
	}
	if _, err := r.Execute("select_option", map[string]any{"selector": "#sort", "value": "lohi"}); err != nil {
		t.Fatalf("expected success when one criterion is provided, got %v", err)
	}
}

func TestMatchesType_AcceptsJSONNumberAsNumber(t *testing.T) {
	tests := []struct {
		name  string
		typ   string
		value any
		want  bool
	}{
		{"json.Number", "number", json.Number("42"), true},
		{"json.Number float", "number", json.Number("3.14"), true},
		{"json.Number zero", "number", json.Number("0"), true},
		{"json.Number negative", "number", json.Number("-5"), true},
		{"int", "number", 42, true},
		{"float64", "number", 3.14, true},
		{"string is not number", "number", "42", false},
		{"json.Number not a string", "string", json.Number("hello"), false},
		{"json.Number not a bool", "bool", json.Number("1"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesType(tt.typ, tt.value); got != tt.want {
				t.Errorf("matchesType(%q, %T(%v)) = %v, want %v", tt.typ, tt.value, tt.value, got, tt.want)
			}
		})
	}
}

func TestObserveUIJS_DynamicAttrIteration(t *testing.T) {
	if !strings.Contains(observeUIJS, "el.attributes") {
		t.Error("observeUIJS should iterate el.attributes for dynamic attribute support")
	}
	if !strings.Contains(observeUIJS, "for (const attr of") {
		t.Error("observeUIJS should use for...of for attribute iteration")
	}
}

func TestObserveUIJS_SkipAttrs(t *testing.T) {
	if !strings.Contains(observeUIJS, `"style"`) {
		t.Error("observeUIJS should skip style attr for token efficiency")
	}
	if !strings.Contains(observeUIJS, `"srcdoc"`) {
		t.Error("observeUIJS should skip srcdoc attr for token efficiency")
	}
}

func TestObserveUIJS_ClassTruncation(t *testing.T) {
	if !strings.Contains(observeUIJS, "slice(0, 5)") {
		t.Error("observeUIJS should truncate class to first 5 tokens")
	}
}

func TestObserveUIJS_LargeValueTruncation(t *testing.T) {
	if !strings.Contains(observeUIJS, "300") {
		t.Error("observeUIJS should truncate values >300 chars")
	}
}

func TestObserveUIJS_RuntimeValueExtraction(t *testing.T) {
	if !strings.Contains(observeUIJS, "el.value") {
		t.Error("observeUIJS should extract runtime el.value for inputs")
	}
}

func TestObserveUIJS_PasswordRedaction(t *testing.T) {
	if !strings.Contains(observeUIJS, "********") {
		t.Error("observeUIJS should redact password values")
	}
	if !strings.Contains(observeUIJS, "type === 'password'") {
		t.Error("observeUIJS should check for password type")
	}
}

func TestObserveUIJS_BooleanRuntimeProperties(t *testing.T) {
	if !strings.Contains(observeUIJS, "el.checked") {
		t.Error("observeUIJS should capture checked state")
	}
	if !strings.Contains(observeUIJS, "el.disabled") {
		t.Error("observeUIJS should capture disabled state")
	}
	if !strings.Contains(observeUIJS, "el.selected") {
		t.Error("observeUIJS should capture selected state")
	}
}
