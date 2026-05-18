package tools

import (
	"sync"
	"testing"
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
