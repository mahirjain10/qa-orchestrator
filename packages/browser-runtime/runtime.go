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
	if r.isRunning {
		return fmt.Errorf("browser already running")
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

	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
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

	page, err := context.NewPage()
	if err != nil {
		context.Close()
		browser.Close()
		pw.Stop()
		return fmt.Errorf("failed to create page: %w", err)
	}

	r.pw = pw
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
	if r.pw != nil {
		r.pw.Stop()
	}

	r.isRunning = false
	return lastErr
}

func (r *BrowserRuntime) Page() playwright.Page {
	return r.page
}

func (r *BrowserRuntime) IsRunning() bool {
	return r.isRunning
}

func (r *BrowserRuntime) Screenshot(options *ScreenshotOptions) ([]byte, error) {
	if err := r.ensurePage(); err != nil {
		return nil, err
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

func (r *BrowserRuntime) Navigate(ctx context.Context, url string) error {
	if err := r.ensurePage(); err != nil {
		return err
	}

	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled before navigate: %w", ctx.Err())
	}

	timeout := float64(r.config.Timeout.Milliseconds())
	_, err := r.page.Goto(url, playwright.PageGotoOptions{
		Timeout: &timeout,
	})
	if err != nil && r.page.IsClosed() {
		if recreateErr := r.ensurePage(); recreateErr != nil {
			return fmt.Errorf("page closed and recreate failed: %w", recreateErr)
		}
		_, err = r.page.Goto(url, playwright.PageGotoOptions{
			Timeout: &timeout,
		})
	}
	return err
}

func (r *BrowserRuntime) ensurePage() error {
	if r.page == nil {
		return fmt.Errorf("page not available")
	}
	if r.page.IsClosed() {
		if r.context != nil {
			newPage, err := r.context.NewPage()
			if err == nil {
				r.page = newPage
				return nil
			}
		}
		if r.browser != nil {
			newCtx, err := r.browser.NewContext(playwright.BrowserNewContextOptions{
				Viewport: &playwright.Size{
					Width:  r.config.ViewportWidth,
					Height: r.config.ViewportHeight,
				},
			})
			if err == nil {
				r.context = newCtx
				newPage, err := r.context.NewPage()
				if err == nil {
					r.page = newPage
					return nil
				}
				return fmt.Errorf("failed to create page after new context: %w", err)
			}
			return fmt.Errorf("failed to recreate browser context: %w", err)
		}
		return fmt.Errorf("browser not available to recreate context")
	}
	return nil
}

func (r *BrowserRuntime) Click(selector string) error {
	if err := r.ensurePage(); err != nil {
		return err
	}

	timeout := float64(r.config.Timeout.Milliseconds())
	err := r.page.Click(selector, playwright.PageClickOptions{
		Timeout: &timeout,
	})
	return err
}

func (r *BrowserRuntime) Fill(selector, value string) error {
	if err := r.ensurePage(); err != nil {
		return err
	}

	timeout := float64(r.config.Timeout.Milliseconds())
	err := r.page.Fill(selector, value, playwright.PageFillOptions{
		Timeout: &timeout,
	})
	return err
}

func (r *BrowserRuntime) WaitForSelector(selector string, options *WaitForOptions) error {
	if err := r.ensurePage(); err != nil {
		return err
	}

	timeout := float64(r.config.Timeout.Milliseconds())
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
	if err := r.ensurePage(); err != nil {
		return "", err
	}

	timeout := float64(r.config.Timeout.Milliseconds())
	return r.page.TextContent(selector, playwright.PageTextContentOptions{
		Timeout: &timeout,
	})
}

func (r *BrowserRuntime) InnerHTML(selector string) (string, error) {
	if err := r.ensurePage(); err != nil {
		return "", err
	}

	timeout := float64(r.config.Timeout.Milliseconds())
	return r.page.InnerHTML(selector, playwright.PageInnerHTMLOptions{
		Timeout: &timeout,
	})
}

func (r *BrowserRuntime) Evaluate(expression string) (any, error) {
	if err := r.ensurePage(); err != nil {
		return nil, err
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

type BrowserRuntimeInterface interface {
	Navigate(ctx context.Context, url string) error
	Click(selector string) error
	Fill(selector, value string) error
	WaitForSelector(selector string, options *WaitForOptions) error
	TextContent(selector string) (string, error)
	InnerHTML(selector string) (string, error)
	Evaluate(expression string) (any, error)
	Screenshot(options *ScreenshotOptions) ([]byte, error)
	Page() playwright.Page
	IsRunning() bool
}

type FlowBrowserRuntime struct {
	parent    *BrowserRuntime
	context   playwright.BrowserContext
	page      playwright.Page
	isRunning bool
}

func (r *BrowserRuntime) NewFlowRuntime() (*FlowBrowserRuntime, error) {
	if !r.isRunning {
		return nil, fmt.Errorf("browser not started")
	}

	ctx, err := r.browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  r.config.ViewportWidth,
			Height: r.config.ViewportHeight,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create flow context: %w", err)
	}

	page, err := ctx.NewPage()
	if err != nil {
		ctx.Close()
		return nil, fmt.Errorf("failed to create flow page: %w", err)
	}

	return &FlowBrowserRuntime{
		parent:    r,
		context:   ctx,
		page:      page,
		isRunning: true,
	}, nil
}

func (r *FlowBrowserRuntime) Close() error {
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

	r.isRunning = false
	return lastErr
}

func (r *FlowBrowserRuntime) IsRunning() bool {
	return r.isRunning
}

func (r *FlowBrowserRuntime) ensurePage() error {
	if r.page == nil || r.page.IsClosed() {
		return fmt.Errorf("flow page not available")
	}
	return nil
}

func (r *FlowBrowserRuntime) getTimeout() float64 {
	return float64(r.parent.config.Timeout.Milliseconds())
}

func (r *FlowBrowserRuntime) Page() playwright.Page {
	return r.page
}

func (r *FlowBrowserRuntime) Navigate(ctx context.Context, url string) error {
	if err := r.ensurePage(); err != nil {
		return err
	}

	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled before navigate: %w", ctx.Err())
	}

	timeout := r.getTimeout()
	_, err := r.page.Goto(url, playwright.PageGotoOptions{
		Timeout: &timeout,
	})
	return err
}

func (r *FlowBrowserRuntime) Click(selector string) error {
	if err := r.ensurePage(); err != nil {
		return err
	}

	timeout := r.getTimeout()
	err := r.page.Click(selector, playwright.PageClickOptions{
		Timeout: &timeout,
	})
	return err
}

func (r *FlowBrowserRuntime) Fill(selector, value string) error {
	if err := r.ensurePage(); err != nil {
		return err
	}

	timeout := r.getTimeout()
	err := r.page.Fill(selector, value, playwright.PageFillOptions{
		Timeout: &timeout,
	})
	return err
}

func (r *FlowBrowserRuntime) WaitForSelector(selector string, options *WaitForOptions) error {
	if err := r.ensurePage(); err != nil {
		return err
	}

	timeout := r.getTimeout()
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

func (r *FlowBrowserRuntime) TextContent(selector string) (string, error) {
	if err := r.ensurePage(); err != nil {
		return "", err
	}

	timeout := r.getTimeout()
	return r.page.TextContent(selector, playwright.PageTextContentOptions{
		Timeout: &timeout,
	})
}

func (r *FlowBrowserRuntime) InnerHTML(selector string) (string, error) {
	if err := r.ensurePage(); err != nil {
		return "", err
	}

	timeout := r.getTimeout()
	return r.page.InnerHTML(selector, playwright.PageInnerHTMLOptions{
		Timeout: &timeout,
	})
}

func (r *FlowBrowserRuntime) Evaluate(expression string) (any, error) {
	if err := r.ensurePage(); err != nil {
		return nil, err
	}

	return r.page.Evaluate(expression)
}

func (r *FlowBrowserRuntime) Screenshot(options *ScreenshotOptions) ([]byte, error) {
	if err := r.ensurePage(); err != nil {
		return nil, err
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

var _ BrowserRuntimeInterface = (*BrowserRuntime)(nil)
var _ BrowserRuntimeInterface = (*FlowBrowserRuntime)(nil)
