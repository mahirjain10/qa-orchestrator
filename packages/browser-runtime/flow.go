package browserruntime

import (
	"context"
	"fmt"
	"sync"

	"github.com/playwright-community/playwright-go"
	"qa-orchestrator/packages/shared"
)

type FlowBrowserRuntime struct {
	mu        sync.RWMutex
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
	r.mu.RLock()
	bctx := r.context
	defer r.mu.RUnlock()

	if bctx == nil {
		return nil, fmt.Errorf("context not available")
	}
	state, err := bctx.StorageState()
	if err != nil {
		return nil, fmt.Errorf("failed to get storage state: %w", err)
	}
	return state, nil
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
			lastErr = fmt.Errorf("failed to close flow page: %w", err)
		}
	}
	if r.context != nil {
		if err := r.context.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close flow context: %w", err)
		}
	}

	r.isRunning = false
	r.page = nil
	r.context = nil
	return lastErr
}

func (r *FlowBrowserRuntime) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isRunning
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.page
}

// withPageRecreation wraps a browser operation, acquiring the page reference
// under lock and retrying once if the page is closed mid-operation.
func (r *FlowBrowserRuntime) withPageRecreation(ctx context.Context, op func(page playwright.Page) error) error {
	getPage := func() (playwright.Page, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		if err := r.ensurePage(); err != nil {
			return nil, err
		}
		return r.page, nil
	}

	page, err := getPage()
	if err != nil {
		return err
	}

	err = op(page)
	if err != nil && page.IsClosed() {
		page, err = getPage()
		if err != nil {
			return fmt.Errorf("page closed and recreate failed: %w", err)
		}
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
			if err != nil {
				return fmt.Errorf("flow navigation failed: %w", err)
			}
			return nil
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
			if err := page.Click(selector, playwright.PageClickOptions{
				Timeout: &timeout,
			}); err != nil {
				return fmt.Errorf("flow click failed on %s: %w", selector, err)
			}
			return nil
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
			if err != nil {
				return fmt.Errorf("flow fill failed on %s: %w", selector, err)
			}

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
			return nil
		})
	})
}

func (r *FlowBrowserRuntime) SelectOption(ctx context.Context, selector, value, label string, index *int) error {
	return r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			result, err := page.Evaluate(`([selector, value, label, index]) => {
				const el = document.querySelector(selector);
				if (!el) return { ok: false, error: "selector not found" };
				if (!(el instanceof HTMLSelectElement)) return { ok: false, error: "element is not a <select>" };
				let selected = false;
				if (typeof value === "string" && value.length > 0) {
					el.value = value;
					selected = el.value === value;
				} else if (typeof label === "string" && label.length > 0) {
					const opt = Array.from(el.options).find(o => o.label === label || o.text === label);
					if (opt) {
						el.value = opt.value;
						selected = true;
					}
				} else if (typeof index === "number") {
					const i = Math.trunc(index);
					if (i >= 0 && i < el.options.length) {
						el.selectedIndex = i;
						selected = true;
					}
				}
				if (!selected) return { ok: false, error: "option not found" };
				el.dispatchEvent(new Event("input", { bubbles: true }));
				el.dispatchEvent(new Event("change", { bubbles: true }));
				return { ok: true, value: el.value };
			}`, []any{selector, value, label, index})
			if err != nil {
				return fmt.Errorf("flow select option evaluate failed: %w", err)
			}
			m, ok := result.(map[string]any)
			if !ok {
				return fmt.Errorf("flow select_option: unexpected result type %T", result)
			}
			if okVal, _ := m["ok"].(bool); !okVal {
				if em, ok := m["error"].(string); ok && em != "" {
					return fmt.Errorf("flow select_option failed: %s", em)
				}
				return fmt.Errorf("flow select_option failed")
			}
			return nil
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
			if _, err := page.WaitForSelector(selector, opts); err != nil {
				return fmt.Errorf("flow wait for selector %s failed: %w", selector, err)
			}
			return nil
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
			if err != nil {
				return fmt.Errorf("flow get text content for %s failed: %w", selector, err)
			}
			return nil
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
			if err != nil {
				return fmt.Errorf("flow get inner html for %s failed: %w", selector, err)
			}
			return nil
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
			if err != nil {
				return fmt.Errorf("flow evaluate script failed: %w", err)
			}
			return nil
		})
	})
	return result, err
}
