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
func (m *mockRuntime) Click(selector string) error                    { return nil }
func (m *mockRuntime) Fill(selector, value string) error              { return nil }
func (m *mockRuntime) WaitForSelector(selector string, options *browserruntime.WaitForOptions) error {
	return nil
}
func (m *mockRuntime) TextContent(selector string) (string, error) { return "", nil }
func (m *mockRuntime) InnerHTML(selector string) (string, error)   { return "", nil }
func (m *mockRuntime) Evaluate(expression string) (any, error)     { return m.evaluateFn(expression) }
func (m *mockRuntime) Screenshot(options *browserruntime.ScreenshotOptions) ([]byte, error) {
	return nil, nil
}
func (m *mockRuntime) Page() playwright.Page { return nil }
func (m *mockRuntime) IsRunning() bool       { return true }

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
