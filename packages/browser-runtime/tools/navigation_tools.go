package tools

import (
	"encoding/json"
	"fmt"

	"qa-orchestrator/packages/browser-runtime"
)

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
			return nil, fmt.Errorf("register click: %w", err)
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
			return nil, fmt.Errorf("register type_text: %w", err)
		}
		if err := runtime.Fill(r.ctx, selector, value); err != nil {
			return nil, fmt.Errorf("type_text failed: %w", err)
		}
		return fmt.Sprintf("typed '%s' into %s", value, selector), nil
	})
}

func (r *ToolRegistry) registerSelectOption(runtime browserruntime.BrowserRuntimeInterface) {
	r.Register("select_option", map[string]ParameterInfo{
		"selector": {Type: "string", Description: "CSS selector for the <select> element", Required: true},
		"value":    {Type: "string", Description: "Option value to select", Required: false},
		"label":    {Type: "string", Description: "Option label/text to select", Required: false},
		"index":    {Type: "number", Description: "Option index to select (0-based)", Required: false},
	}, func(params map[string]any) (any, error) {
		selector, ok := params["selector"].(string)
		if !ok {
			return nil, fmt.Errorf("selector parameter required")
		}
		if err := r.checkSelectorExists(runtime, selector); err != nil {
			return nil, fmt.Errorf("register select_option: %w", err)
		}

		value, _ := params["value"].(string)
		label, _ := params["label"].(string)
		var index *int
		if raw, exists := params["index"]; exists {
			switch v := raw.(type) {
			case int:
				index = &v
			case int64:
				i := int(v)
				index = &i
			case float64:
				i := int(v)
				index = &i
			case json.Number:
				n, err := v.Int64()
				if err != nil {
					return nil, fmt.Errorf("index must be an integer")
				}
				i := int(n)
				index = &i
			default:
				return nil, fmt.Errorf("index must be a number")
			}
		}

		criteriaCount := 0
		if value != "" {
			criteriaCount++
		}
		if label != "" {
			criteriaCount++
		}
		if index != nil {
			criteriaCount++
		}
		if criteriaCount == 0 {
			return nil, fmt.Errorf("one of value, label, or index is required")
		}
		if criteriaCount > 1 {
			return nil, fmt.Errorf("provide only one of value, label, or index")
		}

		if err := runtime.SelectOption(r.ctx, selector, value, label, index); err != nil {
			return nil, fmt.Errorf("select_option failed: %w", err)
		}
		if value != "" {
			return fmt.Sprintf("selected option value '%s' on %s", value, selector), nil
		}
		if label != "" {
			return fmt.Sprintf("selected option label '%s' on %s", label, selector), nil
		}
		return fmt.Sprintf("selected option index %d on %s", *index, selector), nil
	})
}
