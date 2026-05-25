package browserruntime

import (
	"context"
	"fmt"

	"github.com/playwright-community/playwright-go"
)

type ScreenshotOptions struct {
	Path     string
	FullPage bool
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
			if err != nil {
				return fmt.Errorf("screenshot failed: %w", err)
			}
			return nil
		})
	})
	return data, err
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
			if err != nil {
				return fmt.Errorf("flow screenshot failed: %w", err)
			}
			return nil
		})
	})
	return data, err
}
