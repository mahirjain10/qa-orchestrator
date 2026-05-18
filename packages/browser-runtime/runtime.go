package browserruntime

import (
	"context"
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
)

type BrowserType string

const (
	BrowserTypeChromium BrowserType = "chromium"
	BrowserTypeFirefox BrowserType = "firefox"
	BrowserTypeWebkit  BrowserType = "webkit"
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
	config    *Config
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
	if r.isRunning {
		return fmt.Errorf("browser already running")
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
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  r.config.ViewportWidth,
			Height: r.config.ViewportHeight,
		},
	})
	if err != nil {
		browser.Close()
		return fmt.Errorf("failed to create context: %w", err)
	}

	page, err := context.NewPage()
	if err != nil {
		context.Close()
		browser.Close()
		return fmt.Errorf("failed to create page: %w", err)
	}

	r.browser = browser
	r.context = context
	r.page = page
	r.isRunning = true

	return nil
}

func (r *BrowserRuntime) Stop() error {
	if !r.isRunning {
		return nil
	}

	var lastErr error
	if r.page != nil {
		if err := r.page.Close(); err != nil {
			lastErr = err
		}
	}
	if r.context != nil {
		if err := r.context.Close(); err != nil {
			lastErr = err
		}
	}
	if r.browser != nil {
		if err := r.browser.Close(); err != nil {
			lastErr = err
		}
	}

	r.isRunning = false
	return lastErr
}

func (r *BrowserRuntime) Page() playwright.Page {
	return r.page
}

func (r *BrowserRuntime) Context() playwright.BrowserContext {
	return r.context
}

func (r *BrowserRuntime) IsRunning() bool {
	return r.isRunning
}

func (r *BrowserRuntime) Screenshot(options *ScreenshotOptions) ([]byte, error) {
	if r.page == nil {
		return nil, fmt.Errorf("page not available")
	}

	opts := playwright.PageScreenshotOptions{}
	if options != nil {
		if options.Path != "" {
			opts.Path = &options.Path
		}
		if options.FullPage {
			opts.FullPage = &options.FullPage
		}
	}

	return r.page.Screenshot(opts)
}

func (r *BrowserRuntime) Navigate(url string) error {
	if r.page == nil {
		return fmt.Errorf("page not available")
	}

	timeout := float64(r.config.Timeout.Seconds())
	_, err := r.page.Goto(url, playwright.PageGotoOptions{
		Timeout: &timeout,
	})
	return err
}

func (r *BrowserRuntime) Click(selector string) error {
	if r.page == nil {
		return fmt.Errorf("page not available")
	}

	timeout := float64(r.config.Timeout.Seconds())
	err := r.page.Click(selector, playwright.PageClickOptions{
		Timeout: &timeout,
	})
	return err
}

func (r *BrowserRuntime) Fill(selector, value string) error {
	if r.page == nil {
		return fmt.Errorf("page not available")
	}

	timeout := float64(r.config.Timeout.Seconds())
	err := r.page.Fill(selector, value, playwright.PageFillOptions{
		Timeout: &timeout,
	})
	return err
}

func (r *BrowserRuntime) WaitForSelector(selector string, options *WaitForOptions) error {
	if r.page == nil {
		return fmt.Errorf("page not available")
	}

	timeout := float64(r.config.Timeout.Seconds())
	opts := playwright.PageWaitForSelectorOptions{
		Timeout: &timeout,
	}

	if options != nil {
		if options.State == "visible" {
			opts.State = playwright.WaitForSelectorStateVisible
		} else if options.State == "hidden" {
			opts.State = playwright.WaitForSelectorStateHidden
		} else if options.State == "attached" {
			opts.State = playwright.WaitForSelectorStateAttached
		}
	}

	_, err := r.page.WaitForSelector(selector, opts)
	return err
}

func (r *BrowserRuntime) TextContent(selector string) (string, error) {
	if r.page == nil {
		return "", fmt.Errorf("page not available")
	}

	timeout := float64(r.config.Timeout.Seconds())
	return r.page.TextContent(selector, playwright.PageTextContentOptions{
		Timeout: &timeout,
	})
}

func (r *BrowserRuntime) InnerHTML(selector string) (string, error) {
	if r.page == nil {
		return "", fmt.Errorf("page not available")
	}

	timeout := float64(r.config.Timeout.Seconds())
	return r.page.InnerHTML(selector, playwright.PageInnerHTMLOptions{
		Timeout: &timeout,
	})
}

func (r *BrowserRuntime) Evaluate(expression string) (any, error) {
	if r.page == nil {
		return nil, fmt.Errorf("page not available")
	}

	return r.page.Evaluate(expression)
}

type ScreenshotOptions struct {
	Path     string
	FullPage bool
}

type WaitForOptions struct {
	State string
}
