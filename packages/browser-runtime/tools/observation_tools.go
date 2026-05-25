package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"qa-orchestrator/packages/browser-runtime"
)

const observeUIJS = `(() => {
	const maxElements = 50;
	const priorityOrder = { 'INPUT': 0, 'BUTTON': 1, 'TEXTAREA': 2, 'SELECT': 3, 'A': 4 };
	const interactiveTags = { 'INPUT': true, 'BUTTON': true, 'TEXTAREA': true, 'SELECT': true, 'A': true, 'FORM': true };

	function isVisible(el) {
		const rect = el.getBoundingClientRect();
		if (rect.width === 0 || rect.height === 0) return false;
		const style = window.getComputedStyle(el);
		return style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0';
	}

	function isCapturable(el) {
		const tag = el.tagName;
		if (interactiveTags[tag]) return true;
		const role = el.getAttribute('role');
		if (role) return true;
		if (el.getAttribute('tabindex') !== null) return true;
		const text = (el.textContent || '').trim();
		if (text.length > 0) return true;
		if (el.getAttribute('data-test')) return true;
		return false;
	}

	function cssEscapeClass(name) {
		return name.replace(/[!"#$%&'()*+,.\/:;<=>?@[\]^{|}~ ]/g, '\\$&');
	}

	function buildSelector(el) {
		const tag = el.tagName.toLowerCase();
		if (el.id) return '#' + CSS.escape(el.id);
		const name = el.getAttribute('name');
		if (name) return tag + '[name="' + name.replace(/"/g, '\\"') + '"]';
		const dataTest = el.getAttribute('data-test');
		if (dataTest) return tag + '[data-test="' + dataTest.replace(/"/g, '\\"') + '"]';
		const cls = el.getAttribute('class');
		if (cls) {
			const parts = cls.trim().split(/\s+/).filter(function(c) { return c.length > 0; });
			if (parts.length > 0) {
				const safeParts = [];
				for (let p = 0; p < parts.length; p++) {
					if (typeof CSS !== 'undefined' && CSS.escape) {
						safeParts.push(CSS.escape(parts[p]));
					} else {
						safeParts.push(cssEscapeClass(parts[p]));
					}
				}
				return tag + '.' + safeParts.join('.');
			}
		}
		let idx = 1;
		let sib = el.previousElementSibling;
		while (sib) { if (sib.tagName === el.tagName) idx++; sib = sib.previousElementSibling; }
		return tag + ':nth-of-type(' + idx + ')';
	}

	const collected = [];
	if (!document.body) return JSON.stringify({ page_state: 'empty', interactive: [] });
	const walker = document.createTreeWalker(
		document.body,
		NodeFilter.SHOW_ELEMENT,
		{
			acceptNode: function(node) {
				if (isVisible(node) && isCapturable(node)) return NodeFilter.FILTER_ACCEPT;
				return NodeFilter.FILTER_SKIP;
			}
		},
		false
	);
	while (walker.nextNode() && collected.length < maxElements) {
		collected.push(walker.currentNode);
	}

	// Tag priority sort removed to preserve visual DOM order
	// Filter noisy attributes that destroy token efficiency (inline styles, iframe srcdoc, etc.)
	const SKIP_ATTRS = new Set(["style", "srcdoc"]);

	const elements = [];
	for (let i = 0; i < collected.length && i < maxElements; i++) {
		const el = collected[i];
		const elem = { tag: el.tagName.toLowerCase() };

		// Dynamically collect all HTML attributes — no schema coupling between DOM and planner
		for (const attr of el.attributes) {
			if (SKIP_ATTRS.has(attr.name)) continue;
			let val = attr.value;
			if (val === "") continue;
			if (attr.name === "class") {
				// Keep only first 5 classes — large Tailwind/etc strings destroy token budget
				val = val.split(/\s+/).slice(0, 5).join(" ");
			}
			if (val.length > 300) val = val.substring(0, 300);
			elem[attr.name] = val;
		}

		// Runtime state intentionally overrides static HTML attributes
		// (e.g. el.value reflects current input, not the HTML value="..." default)
		if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT') {
			elem.value = el.type === 'password' ? (el.value.length > 0 ? '********' : '') : (el.value || '');
		}
		// Boolean runtime properties — only included when true by planner rendering
		elem.checked = !!el.checked;
		elem.disabled = !!el.disabled;
		elem.selected = !!el.selected;

		// Computed fields (not native HTML attributes)
		elem.text = (el.textContent || '').trim().substring(0, 100);
		elem.selector = buildSelector(el);
		elements.push(elem);
	}
	return JSON.stringify({
		page_state: elements.length > 0 ? 'loaded' : 'empty',
		interactive: elements
	});
})()`

func (r *ToolRegistry) registerWaitFor(runtime browserruntime.BrowserRuntimeInterface) {
	r.Register("wait_for", map[string]ParameterInfo{
		"selector": {Type: "string", Description: "CSS selector for the element to wait for", Required: true},
		"state":    {Type: "string", Description: "Wait state: visible, hidden, attached (default: visible)", Required: false},
	}, func(params map[string]any) (any, error) {
		selector, ok := params["selector"].(string)
		if !ok {
			return nil, fmt.Errorf("selector parameter required")
		}
		state := "visible"
		if s, ok := params["state"].(string); ok {
			state = s
		}
		if err := r.checkSelectorExists(runtime, selector); err != nil {
			return nil, fmt.Errorf("register wait_for: %w", err)
		}
		err := runtime.WaitForSelector(r.ctx, selector, &browserruntime.WaitForOptions{State: state})
		if err != nil {
			return nil, fmt.Errorf("wait_for failed: %w", err)
		}
		return fmt.Sprintf("waited for %s (%s)", selector, state), nil
	})
}

func (r *ToolRegistry) registerGetText(runtime browserruntime.BrowserRuntimeInterface) {
	r.Register("get_text", map[string]ParameterInfo{
		"selector": {Type: "string", Description: "CSS selector for the element", Required: true},
	}, func(params map[string]any) (any, error) {
		selector, ok := params["selector"].(string)
		if !ok {
			return nil, fmt.Errorf("selector parameter required")
		}
		text, err := runtime.TextContent(r.ctx, selector)
		if err != nil {
			return nil, fmt.Errorf("get_text failed: %w", err)
		}
		return text, nil
	})
}

func (r *ToolRegistry) registerGetHTML(runtime browserruntime.BrowserRuntimeInterface) {
	r.Register("get_html", map[string]ParameterInfo{
		"selector": {Type: "string", Description: "CSS selector for the element", Required: true},
	}, func(params map[string]any) (any, error) {
		selector, ok := params["selector"].(string)
		if !ok {
			return nil, fmt.Errorf("selector parameter required")
		}
		html, err := runtime.InnerHTML(r.ctx, selector)
		if err != nil {
			return nil, fmt.Errorf("get_html failed: %w", err)
		}
		return html, nil
	})
}

func (r *ToolRegistry) registerEvaluate(runtime browserruntime.BrowserRuntimeInterface) {
	r.Register("evaluate", map[string]ParameterInfo{
		"expression": {Type: "string", Description: "JavaScript expression to evaluate", Required: true},
	}, func(params map[string]any) (any, error) {
		expr, ok := params["expression"].(string)
		if !ok {
			return nil, fmt.Errorf("expression parameter required")
		}
		result, err := runtime.Evaluate(r.ctx, expr)
		if err != nil {
			return nil, fmt.Errorf("evaluate failed: %w", err)
		}
		return result, nil
	})
}

func (r *ToolRegistry) registerScreenshot(runtime browserruntime.BrowserRuntimeInterface) {
	r.Register("screenshot", map[string]ParameterInfo{
		"path":      {Type: "string", Description: "File path to save the screenshot", Required: false},
		"full_page": {Type: "bool", Description: "Capture full page if true", Required: false},
	}, func(params map[string]any) (any, error) {
		path := ""
		if p, ok := params["path"].(string); ok {
			path = p
		}
		fullPage := false
		if fp, ok := params["full_page"].(bool); ok {
			fullPage = fp
		}
		screenshot, err := runtime.Screenshot(r.ctx, &browserruntime.ScreenshotOptions{
			Path:     path,
			FullPage: fullPage,
		})
		if err != nil {
			return nil, fmt.Errorf("screenshot failed: %w", err)
		}
		return fmt.Sprintf("screenshot captured (%d bytes)", len(screenshot)), nil
	})
}

func (r *ToolRegistry) registerObserveUI(runtime browserruntime.BrowserRuntimeInterface) {
	r.Register("observe_ui", map[string]ParameterInfo{}, func(params map[string]any) (any, error) {
		result, err := runtime.Evaluate(r.ctx, observeUIJS)
		if err != nil {
			return nil, fmt.Errorf("observe_ui failed: %w", err)
		}
		str, ok := result.(string)
		if !ok {
			return nil, fmt.Errorf("observe_ui: expected string result, got %T", result)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(str), &parsed); err != nil {
			return nil, fmt.Errorf("observe_ui: failed to parse result: %w", err)
		}

		// Check heading first (more targeted — h1/h2 with 404 content)
		headingResult, headingErr := runtime.Evaluate(r.ctx, `
			(() => {
				const el = document.querySelector('h1, h2, .error, .not-found, .page-not-found, .error-page');
				return el ? el.textContent.substring(0, 100) : '';
			})()
		`)
		if headingErr == nil {
			if headingStr, ok := headingResult.(string); ok {
				headingLower := strings.ToLower(headingStr)
				if strings.Contains(headingLower, "404") || strings.Contains(headingLower, "not found") {
					parsed["warning"] = "⚠️ WARNING: Page appears to be a 404 or error page."
				}
			}
		}

		// Only check title if heading didn't trigger — require prefix match to avoid
		// flagging legitimate pages whose titles mention "404" or "not found" in passing.
		if _, exists := parsed["warning"]; !exists {
			titleResult, titleErr := runtime.Evaluate(r.ctx, "document.title")
			if titleErr == nil {
				title, _ := titleResult.(string)
				titleLower := strings.ToLower(title)
				if strings.HasPrefix(titleLower, "404") || strings.HasPrefix(titleLower, "not found") {
					parsed["warning"] = "⚠️ WARNING: Page appears to be a 404 or error page."
				}
			}
		}

		return parsed, nil
	})
}
