package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ai_errors "github.com/tutu-hack/openworld/internal/errors/ai"
)

const (
	searchAttempts          = 2
	searchRetryBackoff      = 2 * time.Second
	searchResponseBodyLimit = 8 << 20
	searchHeaderTimeout     = 3 * time.Minute
)

type searchClient struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

type searchRequest struct {
	Model           string       `json:"model"`
	Instructions    string       `json:"instructions"`
	Input           string       `json:"input"`
	Tools           []searchTool `json:"tools"`
	MaxOutputTokens int          `json:"max_output_tokens,omitempty"`
}

type searchTool struct {
	Type string `json:"type"`
}

type searchResponse struct {
	Status            string `json:"status"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func newSearchClient(apiKey string, baseURL string, timeout time.Duration) searchClient {
	return searchClient{
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:          idleConnections,
				MaxIdleConnsPerHost:   idleConnections,
				IdleConnTimeout:       idleConnTimeout,
				ResponseHeaderTimeout: searchHeaderTimeout,
				ForceAttemptHTTP2:     true,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return ai_errors.ErrRedirectNotAllowed
			},
		},
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
	}
}

func (c searchClient) search(ctx context.Context, payload searchRequest) (string, error) {
	payload.Tools = []searchTool{{Type: "web_search"}}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode DeepSeek search request: %w", err)
	}

	var lastError error

	for attempt := 1; attempt <= searchAttempts; attempt++ {
		content, err := c.attemptSearch(ctx, requestBody)
		if err == nil {
			return content, nil
		}

		lastError = err

		if !retryableError(err) || attempt == searchAttempts {
			return "", lastError
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(searchRetryBackoff):
		}
	}

	return "", lastError
}

func (c searchClient) attemptSearch(ctx context.Context, requestBody []byte) (string, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/responses",
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return "", fmt.Errorf("create DeepSeek search request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request) //nolint:gosec // Base URL is operator-controlled and validated at startup.
	if err != nil {
		return "", fmt.Errorf("%w: %w", ai_errors.ErrTemporaryFailure, err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, searchResponseBodyLimit))
	if err != nil {
		return "", fmt.Errorf("%w: read DeepSeek search response: %w", ai_errors.ErrTemporaryFailure, err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		statusError := fmt.Errorf("%w: %d", ai_errors.ErrUnexpectedStatus, response.StatusCode)
		if retryableStatus(response.StatusCode) {
			return "", fmt.Errorf("%w: %w", ai_errors.ErrTemporaryFailure, statusError)
		}

		return "", statusError
	}

	var result searchResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("decode DeepSeek search response: %w", err)
	}

	if result.IncompleteDetails != nil && result.IncompleteDetails.Reason == "max_output_tokens" {
		return "", ai_errors.ErrTruncatedCompletion
	}

	content := sanitizeContent(searchMessageText(result))
	if content == "" {
		return "", ai_errors.ErrMissingChoice
	}

	return content, nil
}

func searchMessageText(response searchResponse) string {
	var builder strings.Builder

	for _, item := range response.Output {
		if item.Type != "message" {
			continue
		}

		for _, part := range item.Content {
			if part.Type == "output_text" || part.Type == "text" {
				builder.WriteString(part.Text)
			}
		}
	}

	return builder.String()
}
