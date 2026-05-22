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
	provider    Provider
	httpClient  *http.Client
	retryConfig *RetryConfig
}

func NewClient(cfg *Config) (*HTTPClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	provider, err := cfg.GetProvider()
	if err != nil {
		return nil, fmt.Errorf("getting provider: %w", err)
	}

	if err := provider.ValidateModel(cfg.Model); err != nil {
		return nil, fmt.Errorf("validating model for %s: %w", provider.Name(), err)
	}

	return &HTTPClient{
		config:   cfg,
		provider: provider,
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

	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		resp, err := c.doRequest(ctx, req)
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

func (c *HTTPClient) doRequest(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	messages := req.Messages
	systemPrompt := ""

	if len(messages) > 0 && messages[0].Role == RoleSystem {
		systemPrompt = messages[0].Content
		messages = messages[1:]
	}

	body, err := c.provider.BuildRequest(messages, systemPrompt, req.Model, req.Temperature, req.MaxTokens)
	if err != nil {
		return nil, NewRetryableError(fmt.Errorf("building request: %w", err), true)
	}

	endpoint := c.provider.Endpoint(req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, NewRetryableError(fmt.Errorf("creating request: %w", err), true)
	}

	authHeaders := c.provider.AuthHeaders(c.config.GetAPIKey())
	for key, value := range authHeaders {
		httpReq.Header.Set(key, value)
	}

	log.Info().
		Str("provider", c.provider.Name()).
		Str("model", req.Model).
		Str("endpoint", endpoint).
		Msg("LLM request")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewRetryableError(NetworkError(err.Error()), true)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		log.Warn().
			Str("provider", c.provider.Name()).
			Str("model", req.Model).
			Str("retry_after", retryAfter.String()).
			Msg("LLM rate limited")
		return nil, NewRetryableError(
			fmt.Errorf("rate limited by %s (model: %s)", c.provider.Name(), req.Model),
			true,
			retryAfter,
		)
	}

	if !isSuccess(resp.StatusCode) {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, NewRetryableError(
				fmt.Errorf("request failed with status %d (provider: %s, model: %s)", resp.StatusCode, c.provider.Name(), req.Model),
				IsRetryableStatusCode(resp.StatusCode),
			)
		}
		log.Error().
			Str("provider", c.provider.Name()).
			Str("model", req.Model).
			Int("status", resp.StatusCode).
			Msg("LLM error response")
		return nil, c.provider.ParseError(resp.StatusCode, bodyBytes)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	generateResp, err := c.provider.ParseResponse(bodyBytes)
	if err != nil {
		return nil, err
	}

	log.Info().
		Str("provider", c.provider.Name()).
		Str("model_requested", req.Model).
		Str("model_used", generateResp.Model).
		Msg("LLM response received")

	if len(generateResp.Content) == 0 {
		return nil, NewNonRetryableError(fmt.Errorf("empty response from %s (model: %s)", c.provider.Name(), req.Model))
	}

	if len(generateResp.Choices) > 0 {
		finishReason := generateResp.Choices[0].FinishReason
		if finishReason != "" {
			log.Debug().Str("finish_reason", finishReason).Msg("LLM finish reason")
		}
	}

	return generateResp, nil
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

// SimpleClient adapts the full Client interface to a simpler prompt-based API.
//
// The planner and engine expect an LLMClient interface with a Generate(ctx, prompt) signature,
// but the underlying Client interface uses Generate(ctx, *GenerateRequest) with a structured
// request object. SimpleClient bridges this gap by wrapping Client and accepting plain strings,
// converting them internally to Message slices before delegating to Client.
//
// This adapter exists to keep the LLM package's public interface clean (Client + GenerateRequest)
// while providing a convenience wrapper for callers that only need simple prompt/response semantics.
// TODO: Consider consolidating LLMClient and Client into a single interface in a future refactor.
type SimpleClient struct {
	client Client
}

func NewSimpleClientWithClient(client Client) *SimpleClient {
	return &SimpleClient{client: client}
}

func NewSimpleClient(apiKey string) (*SimpleClient, error) {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = &Config{
			APIKey:     apiKey,
			BaseURL:    defaultBaseURL,
			Model:      defaultModel,
			Timeout:    defaultTimeout * time.Second,
			MaxRetries: defaultMaxRetries,
		}
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
