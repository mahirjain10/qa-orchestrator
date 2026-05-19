package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type Client interface {
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
	GenerateWithMessages(ctx context.Context, messages []Message) (*GenerateResponse, error)
	Close() error
}

type HTTPClient struct {
	config      *Config
	httpClient  *http.Client
	retryConfig *RetryConfig
}

func NewClient(cfg *Config) (*HTTPClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &HTTPClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		retryConfig: &DefaultRetryConfig,
	}, nil
}

func (c *HTTPClient) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	if req.Model == "" {
		req.Model = c.config.Model
	}

	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	if req.MaxTokens == 0 {
		req.MaxTokens = 1024
	}

	// Apply provider settings if not already set in request
	if req.Provider == nil && (c.config.ProviderPriority != "" || c.config.ProviderAllow != "") {
		req.Provider = &ProviderSettings{}
		if c.config.ProviderPriority != "" {
			req.Provider.Order = strings.Split(c.config.ProviderPriority, ",")
			for i := range req.Provider.Order {
				req.Provider.Order[i] = strings.TrimSpace(req.Provider.Order[i])
			}
		}
		if c.config.ProviderAllow != "" {
			allow := c.config.ProviderAllow == "true"
			req.Provider.AllowFallbacks = &allow
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		resp, err := c.doRequest(ctx, body)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		if !IsRetryable(err) {
			return nil, err
		}

		retry, delay := ShouldRetry(err, attempt, c.config.MaxRetries)
		if !retry {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *HTTPClient) doRequest(ctx context.Context, body []byte) (*GenerateResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.config.Endpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, NewRetryableError(fmt.Errorf("creating request: %w", err), true)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	if c.config.HTTPReferer != "" {
		httpReq.Header.Set("HTTP-Referer", c.config.HTTPReferer)
	}
	if c.config.AppTitle != "" {
		httpReq.Header.Set("X-Title", c.config.AppTitle)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewRetryableError(NetworkError(err.Error()), true)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, NewRetryableError(
			fmt.Errorf("rate limited"),
			true,
			retryAfter,
		)
	}

	if !isSuccess(resp.StatusCode) {
		errResp, err := parseErrorResponse(resp.Body)
		if err != nil {
			return nil, NewRetryableError(
				fmt.Errorf("request failed with status %d", resp.StatusCode),
				IsRetryableStatusCode(resp.StatusCode),
			)
		}
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Message:    errResp.Error.Message,
			Type:       errResp.Error.Type,
			Code:       errResp.Error.Code,
		}
		return nil, NewRetryableError(apiErr, IsRetryableStatusCode(resp.StatusCode))
	}

	var generateResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&generateResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Log the model actually used (OpenRouter may route differently)
	log.Debug().Str("model_used", generateResp.Model).Msg("LLM response received")

	// Extract content from choices (OpenAI format) or direct content
	content := generateResp.Content
	if content == "" && len(generateResp.Choices) > 0 {
		content = generateResp.Choices[0].Message.Content
		finishReason := generateResp.Choices[0].FinishReason
		if finishReason != "" {
			log.Debug().Str("finish_reason", finishReason).Msg("LLM finish reason")
		}
	}

	if len(content) == 0 {
		return nil, NewNonRetryableError(fmt.Errorf("empty response from LLM"))
	}

	generateResp.Content = content
	return &generateResp, nil
}

func (c *HTTPClient) GenerateWithMessages(ctx context.Context, messages []Message) (*GenerateResponse, error) {
	req := &GenerateRequest{
		Model:    c.config.Model,
		Messages: messages,
	}
	return c.Generate(ctx, req)
}

func (c *HTTPClient) Close() error {
	return nil
}

func isSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

func parseErrorResponse(body io.Reader) (*ErrorResponse, error) {
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("reading error body: %w", err)
	}

	// Try standard OpenAI/OpenRouter error format
	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err == nil {
		if errResp.Error.Message != "" {
			return &errResp, nil
		}
	}

	// Try nested error format (OpenRouter may wrap provider errors)
	var nested struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code,omitempty"`
			Inner   struct {
				Message string `json:"message"`
			} `json:"inner_error,omitempty"`
		} `json:"error"`
	}
	if err := json.Unmarshal(bodyBytes, &nested); err == nil {
		if nested.Error.Message != "" {
			return &ErrorResponse{
				Error: ErrorDetail{
					Message: nested.Error.Message,
					Type:    nested.Error.Type,
					Code:    nested.Error.Code,
				},
			}, nil
		}
		if nested.Error.Inner.Message != "" {
			return &ErrorResponse{
				Error: ErrorDetail{
					Message: nested.Error.Inner.Message,
					Type:    "provider_error",
				},
			}, nil
		}
	}

	// Try raw error object
	var raw map[string]any
	if err := json.Unmarshal(bodyBytes, &raw); err == nil {
		if msg, ok := raw["error"].(string); ok {
			return &ErrorResponse{
				Error: ErrorDetail{Message: msg, Type: "unknown"},
			}, nil
		}
	}

	return nil, fmt.Errorf("could not parse error response: %s", string(bodyBytes))
}

func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	seconds := 0
	_, err := fmt.Sscanf(value, "%d", &seconds)
	if err != nil {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

type SimpleClient struct {
	client Client
}

func NewSimpleClientWithClient(client Client) *SimpleClient {
	return &SimpleClient{client: client}
}

func NewSimpleClient(apiKey string) (*SimpleClient, error) {
	cfg := &Config{
		APIKey:     apiKey,
		BaseURL:    defaultBaseURL,
		Model:      defaultModel,
		Timeout:    defaultTimeout * time.Second,
		MaxRetries: defaultMaxRetries,
	}
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &SimpleClient{client: client}, nil
}

func (s *SimpleClient) Generate(ctx context.Context, prompt string) (string, error) {
	messages := []Message{
		{Role: RoleUser, Content: prompt},
	}
	resp, err := s.client.GenerateWithMessages(ctx, messages)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (s *SimpleClient) GenerateWithSystem(ctx context.Context, system, user string) (string, error) {
	messages := []Message{
		{Role: RoleSystem, Content: system},
		{Role: RoleUser, Content: user},
	}
	resp, err := s.client.GenerateWithMessages(ctx, messages)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (s *SimpleClient) Close() error {
	return s.client.Close()
}

func ParseStepsFromResponse(response string) ([]map[string]any, error) {
	response = strings.TrimSpace(response)

	jsonStr, err := extractJSONArray(response)
	if err != nil {
		return nil, err
	}

	var steps []map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &steps); err != nil {
		return nil, fmt.Errorf("parsing steps JSON: %w", err)
	}

	return steps, nil
}

func extractJSONArray(s string) (string, error) {
	for i := 0; i < len(s); i++ {
		if s[i] != '[' {
			continue
		}
		depth := 0
		inString := false
		escaped := false
		for j := i; j < len(s); j++ {
			ch := s[j]
			if inString {
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
					continue
				}
				if ch == '"' {
					inString = false
				}
				continue
			}
			if ch == '"' {
				inString = true
				continue
			}
			if ch == '[' {
				depth++
			} else if ch == ']' {
				depth--
				if depth == 0 {
					candidate := s[i : j+1]
					var probe any
					if err := json.Unmarshal([]byte(candidate), &probe); err == nil {
						if _, ok := probe.([]any); ok {
							return candidate, nil
						}
					}
					break
				}
			}
		}
	}
	return "", fmt.Errorf("no JSON array found in response")
}
