package browserruntime

import (
	"context"
	"fmt"

	"github.com/playwright-community/playwright-go"
	"qa-orchestrator/packages/shared"
)

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
			if err != nil {
				return fmt.Errorf("navigation failed: %w", err)
			}
			return nil
		})
	})
}

func (r *BrowserRuntime) Click(ctx context.Context, selector string) error {
	timeout := float64(r.config.Timeout.Milliseconds())
	return r.withPageRecreation(ctx, func(page playwright.Page) error {
		return runWithContext(ctx, func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := page.Click(selector, playwright.PageClickOptions{
				Timeout: &timeout,
			}); err != nil {
				return fmt.Errorf("click failed on %s: %w", selector, err)
			}
			return nil
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
			if err != nil {
				return fmt.Errorf("fill failed on %s: %w", selector, err)
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

func (r *BrowserRuntime) SelectOption(ctx context.Context, selector, value, label string, index *int) error {
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
				return fmt.Errorf("select option evaluate failed: %w", err)
			}
			m, ok := result.(map[string]any)
			if !ok {
				return fmt.Errorf("select_option: unexpected result type %T", result)
			}
			if okVal, _ := m["ok"].(bool); !okVal {
				if em, ok := m["error"].(string); ok && em != "" {
					return fmt.Errorf("select_option failed: %s", em)
				}
				return fmt.Errorf("select_option failed")
			}
			waitTimeout := float64(500)
			_, _ = page.WaitForFunction(`([sel]) => {
				const el = document.querySelector(sel);
				return !!el && el instanceof HTMLSelectElement;
			}`, []any{selector}, playwright.PageWaitForFunctionOptions{
				Timeout: &waitTimeout,
			})
			return nil
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
			if _, err := page.WaitForSelector(selector, opts); err != nil {
				return fmt.Errorf("wait for selector %s failed: %w", selector, err)
			}
			return nil
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
			if err != nil {
				return fmt.Errorf("get text content for %s failed: %w", selector, err)
			}
			return nil
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
			if err != nil {
				return fmt.Errorf("get inner html for %s failed: %w", selector, err)
			}
			return nil
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
			if err != nil {
				return fmt.Errorf("evaluate script failed: %w", err)
			}
			return nil
		})
	})
	return result, err
}

// withPageRecreation wraps a browser operation, acquiring the page reference
// under lock and retrying once if the page is closed mid-operation.
func (r *BrowserRuntime) withPageRecreation(ctx context.Context, op func(page playwright.Page) error) error {
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
