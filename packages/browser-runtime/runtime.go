package browserruntime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
	"qa-orchestrator/packages/shared"
)

type BrowserType string

const (
	BrowserTypeChromium BrowserType = "chromium"
	BrowserTypeFirefox  BrowserType = "firefox"
	BrowserTypeWebkit   BrowserType = "webkit"
)

type Config struct {
	BrowserType    BrowserType
	Headless       bool
	Timeout        time.Duration
	SlowMo         time.Duration
	ViewportWidth  int
	ViewportHeight int
}

func DefaultConfig() *Config {
	return &Config{
		BrowserType:    BrowserTypeChromium,
		Headless:       true,
		Timeout:        30 * time.Second,
		SlowMo:         0,
		ViewportWidth:  1280,
		ViewportHeight: 720,
	}
}

type BrowserRuntime struct {
	mu        sync.RWMutex
	config    *Config
	pw        *playwright.Playwright
	browser   playwright.Browser
	context   playwright.BrowserContext
	page      playwright.Page
	isRunning bool
}

func NewBrowserRuntime(config *Config) (*BrowserRuntime, error) {
	if config == nil {
		config = DefaultConfig()
	}

	return &BrowserRuntime{
		config:    config,
		isRunning: false,
	}, nil
}

func (r *BrowserRuntime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isRunning {
		return fmt.Errorf("%w", shared.ErrAlreadyRunning)
	}

	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled before start: %w", ctx.Err())
	}

	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("failed to start playwright: %w", err)
	}

	var browser playwright.Browser
	slowMo := float64(r.config.SlowMo.Milliseconds())

	switch r.config.BrowserType {
	case BrowserTypeFirefox:
		browser, err = pw.Firefox.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(r.config.Headless),
			SlowMo:   &slowMo,
		})
	case BrowserTypeWebkit:
		browser, err = pw.WebKit.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(r.config.Headless),
			SlowMo:   &slowMo,
		})
	default:
		browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(r.config.Headless),
			SlowMo:   &slowMo,
		})
	}

	if err != nil {
		pw.Stop()
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	bctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  r.config.ViewportWidth,
			Height: r.config.ViewportHeight,
		},
	})
	if err != nil {
		browser.Close()
		pw.Stop()
		return fmt.Errorf("failed to create context: %w", err)
	}

	page, err := bctx.NewPage()
	if err != nil {
		bctx.Close()
		browser.Close()
		pw.Stop()
		return fmt.Errorf("failed to create page: %w", err)
	}

	r.pw = pw
	r.browser = browser
	r.context = bctx
	r.page = page
	r.isRunning = true

	return nil
}

func (r *BrowserRuntime) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isRunning {
		return nil
	}

	var lastErr error
	if r.page != nil {
		if err := r.page.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close page: %w", err)
		}
	}
	if r.context != nil {
		if err := r.context.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close context: %w", err)
		}
	}
	if r.browser != nil {
		if err := r.browser.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close browser: %w", err)
		}
	}
	if r.pw != nil {
		r.pw.Stop()
	}

	r.isRunning = false
	r.page = nil
	r.context = nil
	r.browser = nil
	r.pw = nil
	return lastErr
}

func (r *BrowserRuntime) Page() playwright.Page {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.page
}

func (r *BrowserRuntime) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isRunning
}

type WaitForOptions struct {
	State string
}

type BrowserRuntimeInterface interface {
	Navigate(ctx context.Context, url string) error
	Click(ctx context.Context, selector string) error
	Fill(ctx context.Context, selector, value string) error
	SelectOption(ctx context.Context, selector, value, label string, index *int) error
	WaitForSelector(ctx context.Context, selector string, options *WaitForOptions) error
	TextContent(ctx context.Context, selector string) (string, error)
	InnerHTML(ctx context.Context, selector string) (string, error)
	Evaluate(ctx context.Context, expression string) (any, error)
	Screenshot(ctx context.Context, options *ScreenshotOptions) ([]byte, error)
	Page() playwright.Page
	IsRunning() bool
}

func runWithContext(ctx context.Context, fn func(context.Context) error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	ch := make(chan error, 1)
	go func() {
		ch <- fn(ctx)
	}()
	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		go func() { <-ch }()
		return ctx.Err()
	}
}

var _ BrowserRuntimeInterface = (*BrowserRuntime)(nil)
var _ BrowserRuntimeInterface = (*FlowBrowserRuntime)(nil)
