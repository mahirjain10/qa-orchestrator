package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"qa-orchestrator/packages/browser-runtime"
	sharedtypes "qa-orchestrator/packages/shared/types"
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

	function buildSelector(el) {
		const tag = el.tagName.toLowerCase();
		if (el.id) return '#' + el.id;
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
						safeParts.push(parts[p]);
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

const selectorExistsJS = `((selector) => {
	try {
		const el = document.querySelector(selector);
		if (!el) return JSON.stringify({exists: false});
		const rect = el.getBoundingClientRect();
		return JSON.stringify({exists: true, visible: rect.width > 0 && rect.height > 0});
	} catch(e) {
		return JSON.stringify({exists: false, error: e.message});
	}
})`

type Tool func(params map[string]any) (any, error)

type ParameterInfo = sharedtypes.ParameterInfo
type ToolInfo = sharedtypes.ToolInfo

type ToolRegistry struct {
	mu       sync.RWMutex
	tools    map[string]Tool
	meta     map[string]ToolInfo
	ctx      context.Context
	cancelFn context.CancelFunc
}

func NewToolRegistry(runtime browserruntime.BrowserRuntimeInterface) *ToolRegistry {
	return NewToolRegistryWithContext(runtime, context.Background())
}

func NewToolRegistryWithContext(runtime browserruntime.BrowserRuntimeInterface, ctx context.Context) *ToolRegistry {
	ctx, cancel := context.WithCancel(ctx)
	registry := &ToolRegistry{
		tools:    make(map[string]Tool),
		meta:     make(map[string]ToolInfo),
		ctx:      ctx,
		cancelFn: cancel,
	}
	registry.registerDefaultTools(runtime)
	return registry
}

func (r *ToolRegistry) Cancel() {
	if r.cancelFn != nil {
		r.cancelFn()
	}
}

func (r *ToolRegistry) checkSelectorExists(runtime browserruntime.BrowserRuntimeInterface, selector string) error {
	selectorJSON, _ := json.Marshal(selector)
	js := selectorExistsJS + `(` + string(selectorJSON) + `)`
	result, err := runtime.Evaluate(r.ctx, js)
	if err != nil {
		return fmt.Errorf("selector check eval failed: %w", err)
	}
	str, ok := result.(string)
	if !ok {
		return fmt.Errorf("selector check: unexpected result type %T", result)
	}
	var check struct {
		Exists  bool   `json:"exists"`
		Visible bool   `json:"visible"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(str), &check); err != nil {
		return fmt.Errorf("selector check: failed to parse result: %w", err)
	}
	if !check.Exists {
		if check.Error != "" {
			return fmt.Errorf("invalid selector syntax for '%s': %s", selector, check.Error)
		}
		return fmt.Errorf("selector '%s' not found on page — selector does not exist in current DOM", selector)
	}
	return nil
}

func (r *ToolRegistry) registerDefaultTools(runtime browserruntime.BrowserRuntimeInterface) {
	r.registerNavigate(runtime)
	r.registerClick(runtime)
	r.registerTypeText(runtime)
	r.registerWaitFor(runtime)
	r.registerGetText(runtime)
	r.registerGetHTML(runtime)
	r.registerEvaluate(runtime)
	r.registerScreenshot(runtime)
	r.registerFinish(runtime)
	r.registerObserveUI(runtime)
	r.registerEcho()
}

func (r *ToolRegistry) registerNavigate(runtime browserruntime.BrowserRuntimeInterface) {
	r.Register("navigate", map[string]ParameterInfo{
		"url": {Type: "string", Description: "The URL to navigate to", Required: true},
	}, func(params map[string]any) (any, error) {
		url, ok := params["url"].(string)
		if !ok {
			return nil, fmt.Errorf("url parameter required")
		}
		if err := runtime.Navigate(r.ctx, url); err != nil {
			return nil, fmt.Errorf("navigate failed: %w", err)
		}
		return fmt.Sprintf("navigated to %s", url), nil
	})
}

func (r *ToolRegistry) registerClick(runtime browserruntime.BrowserRuntimeInterface) {
	r.Register("click", map[string]ParameterInfo{
		"selector": {Type: "string", Description: "CSS selector for the element to click", Required: true},
	}, func(params map[string]any) (any, error) {
		selector, ok := params["selector"].(string)
		if !ok {
			return nil, fmt.Errorf("selector parameter required")
		}
		if err := r.checkSelectorExists(runtime, selector); err != nil {
			return nil, err
		}
		if err := runtime.Click(r.ctx, selector); err != nil {
			return nil, fmt.Errorf("click failed: %w", err)
		}
		return fmt.Sprintf("clicked %s", selector), nil
	})
}

func (r *ToolRegistry) registerTypeText(runtime browserruntime.BrowserRuntimeInterface) {
	r.Register("type_text", map[string]ParameterInfo{
		"selector": {Type: "string", Description: "CSS selector for the input field", Required: true},
		"value":    {Type: "string", Description: "Text to type into the field", Required: true},
	}, func(params map[string]any) (any, error) {
		selector, ok := params["selector"].(string)
		if !ok {
			return nil, fmt.Errorf("selector parameter required")
		}
		value, ok := params["value"].(string)
		if !ok {
			return nil, fmt.Errorf("value parameter required")
		}
		if err := r.checkSelectorExists(runtime, selector); err != nil {
			return nil, err
		}
		if err := runtime.Fill(r.ctx, selector, value); err != nil {
			return nil, fmt.Errorf("type_text failed: %w", err)
		}
		return fmt.Sprintf("typed '%s' into %s", value, selector), nil
	})
}

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

func (r *ToolRegistry) registerFinish(runtime browserruntime.BrowserRuntimeInterface) {
	r.Register("finish", map[string]ParameterInfo{
		"status": {Type: "string", Description: "Set to 'success' if goal is achieved, or 'fail' if the goal is unachievable (e.g. elements not found).", Required: false},
	}, func(params map[string]any) (any, error) {
		status, ok := params["status"].(string)
		if ok && status == "fail" {
			return nil, fmt.Errorf("goal unachievable: execution complete")
		}
		return "goal achieved, execution complete", nil
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

func (r *ToolRegistry) registerEcho() {
	r.Register("echo", map[string]ParameterInfo{
		"value": {Type: "string", Description: "The value to echo back", Required: true},
	}, func(params map[string]any) (any, error) {
		return params["value"], nil
	})
}

func (r *ToolRegistry) Register(name string, params map[string]ParameterInfo, fn Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[name] = fn
	r.meta[name] = ToolInfo{
		Name:        name,
		Description: getToolDescription(name),
		Parameters:  params,
	}
}

func getToolDescription(name string) string {
	descriptions := map[string]string{
		"navigate":   "Navigate to a URL in the browser",
		"click":      "Click on an element identified by CSS selector",
		"type_text":  "Type text into an input field",
		"wait_for":   "Wait for an element to reach a specific state",
		"get_text":   "Get the text content of an element",
		"get_html":   "Get the inner HTML of an element",
		"evaluate":   "Evaluate a JavaScript expression in the browser context",
		"screenshot": "Take a screenshot of the page",
		"finish":     "Signal that the goal has been achieved (or is unachievable) and no more steps are needed",
		"observe_ui": "Inspect the current page and return a list of visible interactive elements with their selectors",
		"echo":       "Return the provided value as-is. Useful for testing tool integration and guided flow verification.",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return fmt.Sprintf("Tool: %s", name)
}

func (r *ToolRegistry) Execute(name string, params map[string]any) (any, error) {
	r.mu.RLock()
	tool, exists := r.tools[name]
	meta, metaExists := r.meta[name]
	r.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	if !metaExists {
		return nil, fmt.Errorf("missing metadata for tool: %s", name)
	}
	if err := validateToolParams(meta, params); err != nil {
		return nil, err
	}
	return tool(params)
}

func (r *ToolRegistry) ListTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]string, 0, len(r.tools))
	for name := range r.tools {
		tools = append(tools, name)
	}
	return tools
}

func (r *ToolRegistry) HasTool(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.tools[name]
	return exists
}

func (r *ToolRegistry) ListToolsWithDocs() []ToolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]ToolInfo, 0, len(r.meta))
	for _, info := range r.meta {
		tools = append(tools, info)
	}
	return tools
}

func (r *ToolRegistry) GetToolInfo(name string) (ToolInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, exists := r.meta[name]
	return info, exists
}

func (r *ToolRegistry) ToLLMTools() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]map[string]any, 0, len(r.meta))
	for _, info := range r.meta {
		params := make(map[string]map[string]any)
		for name, p := range info.Parameters {
			params[name] = map[string]any{
				"type":        p.Type,
				"description": p.Description,
				"required":    p.Required,
			}
		}
		result = append(result, map[string]any{
			"name":        info.Name,
			"description": info.Description,
			"parameters":  params,
		})
	}
	return result
}

func validateToolParams(info ToolInfo, params map[string]any) error {
	if params == nil {
		params = map[string]any{}
	}
	for name, spec := range info.Parameters {
		value, exists := params[name]
		if spec.Required && !exists {
			return fmt.Errorf("%s parameter required", name)
		}
		if !exists {
			continue
		}
		if !matchesType(spec.Type, value) {
			return fmt.Errorf("%s must be %s", name, spec.Type)
		}
	}
	for name := range params {
		if _, known := info.Parameters[name]; !known {
			return fmt.Errorf("unknown parameter: %s", name)
		}
	}
	return nil
}

func matchesType(expected string, value any) bool {
	switch expected {
	case "string":
		_, ok := value.(string)
		return ok
	case "bool":
		_, ok := value.(bool)
		return ok
	case "number":
		switch value.(type) {
		case int, int8, int16, int32, int64, float32, float64, json.Number:
			return true
		default:
			return false
		}
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	default:
		return true
	}
}
