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
	mu        sync.Mutex
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
	r.page = nil
	r.context = nil
	r.browser = nil
	r.pw = nil
	return lastErr
}

func (r *BrowserRuntime) Page() playwright.Page {
	r.mu.Lock()
	p := r.page
	r.mu.Unlock()
	return p
}

func (r *BrowserRuntime) IsRunning() bool {
	r.mu.Lock()
	v := r.isRunning
	r.mu.Unlock()
	return v
}

func (r *BrowserRuntime) Navigate(ctx context.Context, url string) error {
	timeout := float64(r.config.Timeout.Milliseconds())
	return r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			_, err := page.Goto(url, playwright.PageGotoOptions{
				Timeout: &timeout,
			})
			return err
		})
	})
}

// withPageRecreation wraps a browser operation, acquiring the page reference
// under lock and retrying once if the page is closed mid-operation.
func (r *BrowserRuntime) withPageRecreation(ctx context.Context, op func(page playwright.Page) error) error {
	r.mu.Lock()
	if err := r.ensurePage(); err != nil {
		r.mu.Unlock()
		return err
	}
	page := r.page
	r.mu.Unlock()

	err := op(page)
	if err != nil && page.IsClosed() {
		r.mu.Lock()
		if recreateErr := r.ensurePage(); recreateErr != nil {
			r.mu.Unlock()
			return fmt.Errorf("page closed and recreate failed: %w", recreateErr)
		}
		page = r.page
		r.mu.Unlock()
		err = op(page)
	}
	return err
}

// ensurePage must be called with r.mu held.
func (r *BrowserRuntime) ensurePage() error {
	if !r.isRunning {
		return fmt.Errorf("%w", shared.ErrNotRunning)
	}
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
			if r.context != nil {
				r.context.Close()
			}
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

func (r *BrowserRuntime) Click(ctx context.Context, selector string) error {
	timeout := float64(r.config.Timeout.Milliseconds())
	return r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return page.Click(selector, playwright.PageClickOptions{
				Timeout: &timeout,
			})
		})
	})
}

func (r *BrowserRuntime) Fill(ctx context.Context, selector, value string) error {
	timeout := float64(r.config.Timeout.Milliseconds())
	return r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			err := page.Fill(selector, value, playwright.PageFillOptions{
				Timeout: &timeout,
			})
			if err == nil {
				// Dynamically wait for React/Vue/Angular state to reconcile and reflect the value
				waitTimeout := float64(500)
				_, _ = page.WaitForFunction(`([sel, expected]) => {
					const el = document.querySelector(sel);
					if (!el) return false;
					if (el.type === 'checkbox' || el.type === 'radio') return true;
					return el.value === expected;
				}`, []any{selector, value}, playwright.PageWaitForFunctionOptions{
					Timeout: &waitTimeout,
				})
			}
			return err
		})
	})
}

func (r *BrowserRuntime) WaitForSelector(ctx context.Context, selector string, options *WaitForOptions) error {
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

	return r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			_, err := page.WaitForSelector(selector, opts)
			return err
		})
	})
}

func (r *BrowserRuntime) TextContent(ctx context.Context, selector string) (string, error) {
	timeout := float64(r.config.Timeout.Milliseconds())
	var content string
	err := r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var err error
			content, err = page.TextContent(selector, playwright.PageTextContentOptions{
				Timeout: &timeout,
			})
			return err
		})
	})
	return content, err
}

func (r *BrowserRuntime) InnerHTML(ctx context.Context, selector string) (string, error) {
	timeout := float64(r.config.Timeout.Milliseconds())
	var html string
	err := r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var err error
			html, err = page.InnerHTML(selector, playwright.PageInnerHTMLOptions{
				Timeout: &timeout,
			})
			return err
		})
	})
	return html, err
}

func (r *BrowserRuntime) Evaluate(ctx context.Context, expression string) (any, error) {
	var result any
	err := r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var err error
			result, err = page.Evaluate(expression)
			return err
		})
	})
	return result, err
}

func (r *BrowserRuntime) Screenshot(ctx context.Context, options *ScreenshotOptions) ([]byte, error) {
	opts := playwright.PageScreenshotOptions{}
	if options != nil {
		if options.Path != "" {
			opts.Path = &options.Path
		}
		if options.FullPage {
			opts.FullPage = &options.FullPage
		}
	}

	var data []byte
	err := r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var err error
			data, err = page.Screenshot(opts)
			return err
		})
	})
	return data, err
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
	Click(ctx context.Context, selector string) error
	Fill(ctx context.Context, selector, value string) error
	WaitForSelector(ctx context.Context, selector string, options *WaitForOptions) error
	TextContent(ctx context.Context, selector string) (string, error)
	InnerHTML(ctx context.Context, selector string) (string, error)
	Evaluate(ctx context.Context, expression string) (any, error)
	Screenshot(ctx context.Context, options *ScreenshotOptions) ([]byte, error)
	Page() playwright.Page
	IsRunning() bool
}

type FlowBrowserRuntime struct {
	mu        sync.Mutex
	parent    *BrowserRuntime
	context   playwright.BrowserContext
	page      playwright.Page
	isRunning bool
}

func (r *BrowserRuntime) NewFlowRuntime(storageState ...*playwright.StorageState) (*FlowBrowserRuntime, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isRunning {
		return nil, fmt.Errorf("browser not started")
	}

	opts := playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  r.config.ViewportWidth,
			Height: r.config.ViewportHeight,
		},
	}
	if len(storageState) > 0 && storageState[0] != nil {
		opts.StorageState = storageState[0].ToOptionalStorageState()
	}
	bctx, err := r.browser.NewContext(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create flow context: %w", err)
	}

	page, err := bctx.NewPage()
	if err != nil {
		bctx.Close()
		return nil, fmt.Errorf("failed to create flow page: %w", err)
	}

	return &FlowBrowserRuntime{
		parent:    r,
		context:   bctx,
		page:      page,
		isRunning: true,
	}, nil
}

func (r *FlowBrowserRuntime) StorageState() (*playwright.StorageState, error) {
	r.mu.Lock()
	bctx := r.context
	r.mu.Unlock()

	if bctx == nil {
		return nil, fmt.Errorf("context not available")
	}
	return bctx.StorageState()
}

func (r *FlowBrowserRuntime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

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
	r.page = nil
	r.context = nil
	return lastErr
}

func (r *FlowBrowserRuntime) IsRunning() bool {
	r.mu.Lock()
	v := r.isRunning
	r.mu.Unlock()
	return v
}

// ensurePage must be called with r.mu held.
func (r *FlowBrowserRuntime) ensurePage() error {
	if !r.isRunning {
		return fmt.Errorf("%w", shared.ErrNotRunning)
	}
	if r.page == nil || r.page.IsClosed() {
		if r.context != nil {
			newPage, err := r.context.NewPage()
			if err == nil {
				r.page = newPage
				return nil
			}
		}
		if r.parent != nil && r.parent.browser != nil {
			if r.context != nil {
				r.context.Close()
			}
			newCtx, err := r.parent.browser.NewContext(playwright.BrowserNewContextOptions{
				Viewport: &playwright.Size{
					Width:  r.parent.config.ViewportWidth,
					Height: r.parent.config.ViewportHeight,
				},
			})
			if err == nil {
				r.context = newCtx
				newPage, err := r.context.NewPage()
				if err == nil {
					r.page = newPage
					return nil
				}
				return fmt.Errorf("failed to create flow page after new context: %w", err)
			}
			return fmt.Errorf("failed to recreate flow context: %w", err)
		}
		return fmt.Errorf("flow page not available")
	}
	return nil
}

func (r *FlowBrowserRuntime) getTimeout() float64 {
	return float64(r.parent.config.Timeout.Milliseconds())
}

func (r *FlowBrowserRuntime) Page() playwright.Page {
	r.mu.Lock()
	p := r.page
	r.mu.Unlock()
	return p
}

// withPageRecreation wraps a browser operation, acquiring the page reference
// under lock and retrying once if the page is closed mid-operation.
func (r *FlowBrowserRuntime) withPageRecreation(ctx context.Context, op func(page playwright.Page) error) error {
	r.mu.Lock()
	if err := r.ensurePage(); err != nil {
		r.mu.Unlock()
		return err
	}
	page := r.page
	r.mu.Unlock()

	err := op(page)
	if err != nil && page.IsClosed() {
		r.mu.Lock()
		if recreateErr := r.ensurePage(); recreateErr != nil {
			r.mu.Unlock()
			return fmt.Errorf("page closed and recreate failed: %w", recreateErr)
		}
		page = r.page
		r.mu.Unlock()
		err = op(page)
	}
	return err
}

func (r *FlowBrowserRuntime) Navigate(ctx context.Context, url string) error {
	timeout := r.getTimeout()
	return r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			_, err := page.Goto(url, playwright.PageGotoOptions{
				Timeout: &timeout,
			})
			return err
		})
	})
}

func (r *FlowBrowserRuntime) Click(ctx context.Context, selector string) error {
	timeout := r.getTimeout()
	return r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return page.Click(selector, playwright.PageClickOptions{
				Timeout: &timeout,
			})
		})
	})
}

func (r *FlowBrowserRuntime) Fill(ctx context.Context, selector, value string) error {
	timeout := r.getTimeout()
	return r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			err := page.Fill(selector, value, playwright.PageFillOptions{
				Timeout: &timeout,
			})
			if err == nil {
				// Dynamically wait for React/Vue/Angular state to reconcile and reflect the value
				waitTimeout := float64(500)
				_, _ = page.WaitForFunction(`([sel, expected]) => {
					const el = document.querySelector(sel);
					if (!el) return false;
					if (el.type === 'checkbox' || el.type === 'radio') return true;
					return el.value === expected;
				}`, []any{selector, value}, playwright.PageWaitForFunctionOptions{
					Timeout: &waitTimeout,
				})
			}
			return err
		})
	})
}

func (r *FlowBrowserRuntime) WaitForSelector(ctx context.Context, selector string, options *WaitForOptions) error {
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

	return r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			_, err := page.WaitForSelector(selector, opts)
			return err
		})
	})
}

func (r *FlowBrowserRuntime) TextContent(ctx context.Context, selector string) (string, error) {
	timeout := r.getTimeout()
	var content string
	err := r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var err error
			content, err = page.TextContent(selector, playwright.PageTextContentOptions{
				Timeout: &timeout,
			})
			return err
		})
	})
	return content, err
}

func (r *FlowBrowserRuntime) InnerHTML(ctx context.Context, selector string) (string, error) {
	timeout := r.getTimeout()
	var html string
	err := r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var err error
			html, err = page.InnerHTML(selector, playwright.PageInnerHTMLOptions{
				Timeout: &timeout,
			})
			return err
		})
	})
	return html, err
}

func (r *FlowBrowserRuntime) Evaluate(ctx context.Context, expression string) (any, error) {
	var result any
	err := r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var err error
			result, err = page.Evaluate(expression)
			return err
		})
	})
	return result, err
}

func (r *FlowBrowserRuntime) Screenshot(ctx context.Context, options *ScreenshotOptions) ([]byte, error) {
	opts := playwright.PageScreenshotOptions{}
	if options != nil {
		if options.Path != "" {
			opts.Path = &options.Path
		}
		if options.FullPage {
			opts.FullPage = &options.FullPage
		}
	}

	var data []byte
	err := r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var err error
			data, err = page.Screenshot(opts)
			return err
		})
	})
	return data, err
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
		return ctx.Err()
	}
}

var _ BrowserRuntimeInterface = (*BrowserRuntime)(nil)
var _ BrowserRuntimeInterface = (*FlowBrowserRuntime)(nil)
