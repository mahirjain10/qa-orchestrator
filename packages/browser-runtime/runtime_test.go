package browserruntime

import (
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
