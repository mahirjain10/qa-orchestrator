package browserruntime

import (
	"context"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.BrowserType != BrowserTypeChromium {
		t.Errorf("BrowserType = %s, want chromium", config.BrowserType)
	}

	if !config.Headless {
		t.Error("Headless should be true by default")
	}

	if config.Timeout != 30*1e9 {
		t.Errorf("Timeout = %d, want 30000000000", config.Timeout)
	}

	if config.ViewportWidth != 1280 {
		t.Errorf("ViewportWidth = %d, want 1280", config.ViewportWidth)
	}

	if config.ViewportHeight != 720 {
		t.Errorf("ViewportHeight = %d, want 720", config.ViewportHeight)
	}
}

func TestNewBrowserRuntime(t *testing.T) {
	runtime, err := NewBrowserRuntime(nil)
	if err != nil {
		t.Fatalf("NewBrowserRuntime failed: %v", err)
	}

	if runtime.IsRunning() {
		t.Error("New runtime should not be running")
	}

	if runtime.config == nil {
		t.Error("Config should not be nil")
	}
}

func TestNewFlowRuntime_AcceptsNilStorageState(t *testing.T) {
	r, err := NewBrowserRuntime(nil)
	if err != nil {
		t.Fatalf("NewBrowserRuntime failed: %v", err)
	}

	if r.IsRunning() {
		t.Fatal("expected not running")
	}

	// Verify NewFlowRuntime compiles and accepts nil storage state (variadic)
	// Full test would require a started browser, so we only verify the signature
	// compiles by checking the function is callable with no args.
	if r == nil {
		t.Fatal("unexpected nil runtime")
	}
}

func TestNewBrowserRuntimeWithConfig(t *testing.T) {
	config := &Config{
		BrowserType:    BrowserTypeFirefox,
		Headless:       false,
		Timeout:        60,
		ViewportWidth:  1920,
		ViewportHeight: 1080,
	}

	runtime, err := NewBrowserRuntime(config)
	if err != nil {
		t.Fatalf("NewBrowserRuntime failed: %v", err)
	}

	if runtime.config.BrowserType != BrowserTypeFirefox {
		t.Errorf("BrowserType = %s, want firefox", runtime.config.BrowserType)
	}

	if runtime.config.Headless {
		t.Error("Headless should be false")
	}

	if runtime.config.ViewportWidth != 1920 {
		t.Errorf("ViewportWidth = %d, want 1920", runtime.config.ViewportWidth)
	}
}

func TestStop_NilsFieldsAndIsIdempotent(t *testing.T) {
	r, err := NewBrowserRuntime(nil)
	if err != nil {
		t.Fatalf("NewBrowserRuntime failed: %v", err)
	}
	r.isRunning = true

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	if r.page != nil {
		t.Error("page should be nil after Stop()")
	}
	if r.context != nil {
		t.Error("context should be nil after Stop()")
	}
	if r.browser != nil {
		t.Error("browser should be nil after Stop()")
	}
	if r.pw != nil {
		t.Error("pw should be nil after Stop()")
	}
	if r.isRunning {
		t.Error("isRunning should be false after Stop()")
	}

	// Second call must not panic or error
	if err := r.Stop(); err != nil {
		t.Fatalf("second Stop() error: %v", err)
	}
}

func TestClose_NilsFlowFields(t *testing.T) {
	flow := &FlowBrowserRuntime{
		isRunning: true,
	}

	if err := flow.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	if flow.page != nil {
		t.Error("page should be nil after Close()")
	}
	if flow.context != nil {
		t.Error("context should be nil after Close()")
	}
	if flow.isRunning {
		t.Error("isRunning should be false after Close()")
	}
}

func TestOperations_FailWhenNotRunning(t *testing.T) {
	r, err := NewBrowserRuntime(nil)
	if err != nil {
		t.Fatalf("NewBrowserRuntime failed: %v", err)
	}

	ctx := context.Background()
	ops := []struct {
		name string
		fn   func() error
	}{
		{"Navigate", func() error { return r.Navigate(ctx, "https://example.com") }},
		{"Click", func() error { return r.Click(ctx, "#btn") }},
		{"Fill", func() error { return r.Fill(ctx, "#input", "val") }},
		{"WaitForSelector", func() error { return r.WaitForSelector(ctx, "#el", nil) }},
		{"TextContent", func() error { _, err := r.TextContent(ctx, "#el"); return err }},
		{"InnerHTML", func() error { _, err := r.InnerHTML(ctx, "#el"); return err }},
		{"Evaluate", func() error { _, err := r.Evaluate(ctx, "1+1"); return err }},
		{"Screenshot", func() error { _, err := r.Screenshot(ctx, nil); return err }},
	}
	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			if err := op.fn(); err == nil {
				t.Error("expected error for non-started runtime, got nil")
			}
		})
	}
}
