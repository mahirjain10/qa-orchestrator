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
	httpReq.Header.Set("HTTP-Referer", "https://github.com/zenact")
	httpReq.Header.Set("X-Title", "Zenact QA Orchestrator")

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

	if len(generateResp.Content) == 0 {
		return nil, NewNonRetryableError(fmt.Errorf("empty response from LLM"))
	}

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
	var errResp ErrorResponse
	if err := json.NewDecoder(body).Decode(&errResp); err != nil {
		return nil, fmt.Errorf("parsing error response: %w", err)
	}
	return &errResp, nil
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

	start := strings.Index(response, "[")
	if start == -1 {
		return nil, fmt.Errorf("no JSON array found in response")
	}
	end := strings.LastIndex(response, "]")
	if end == -1 {
		return nil, fmt.Errorf("invalid JSON array format")
	}

	jsonStr := response[start : end+1]

	var steps []map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &steps); err != nil {
		return nil, fmt.Errorf("parsing steps JSON: %w", err)
	}

	return steps, nil
}