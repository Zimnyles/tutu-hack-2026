package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	ai_errors "github.com/tutu-hack/openworld/internal/errors/ai"
)

const (
	chatAttempts        = 3
	chatRetryBackoff    = 500 * time.Millisecond
	maximumRetryWait    = 5 * time.Second
	idleConnections     = 10
	idleConnTimeout     = 90 * time.Second
	responseHeaderLimit = 20 * time.Second
)

type chatClient struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

func newChatClient(apiKey string, baseURL string) chatClient {
	return chatClient{
		httpClient: &http.Client{
			Timeout: requestTimeout,
			Transport: &http.Transport{
				MaxIdleConns:          idleConnections,
				MaxIdleConnsPerHost:   idleConnections,
				IdleConnTimeout:       idleConnTimeout,
				ResponseHeaderTimeout: responseHeaderLimit,
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

func (c chatClient) complete(ctx context.Context, payload chatRequest) (string, error) {
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode DeepSeek request: %w", err)
	}

	var lastError error

	for attempt := 1; attempt <= chatAttempts; attempt++ {
		content, retryAfter, err := c.attempt(ctx, requestBody)
		if err == nil {
			return content, nil
		}

		lastError = err

		if !retryableError(err) || attempt == chatAttempts {
			return "", lastError
		}

		wait := chatRetryBackoff * time.Duration(1<<(attempt-1))
		if retryAfter > 0 {
			wait = retryAfter
		}

		if wait > maximumRetryWait {
			wait = maximumRetryWait
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(wait):
		}
	}

	return "", lastError
}

func (c chatClient) attempt(
	ctx context.Context,
	requestBody []byte,
) (string, time.Duration, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/chat/completions",
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return "", 0, fmt.Errorf("create DeepSeek request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request) //nolint:gosec // Base URL is operator-controlled and validated at startup.
	if err != nil {
		return "", 0, fmt.Errorf("%w: %w", ai_errors.ErrTemporaryFailure, err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, responseBodyLimit))
	if err != nil {
		return "", 0, fmt.Errorf("%w: read DeepSeek response: %w", ai_errors.ErrTemporaryFailure, err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		statusError := fmt.Errorf("%w: %d", ai_errors.ErrUnexpectedStatus, response.StatusCode)
		if retryableStatus(response.StatusCode) {
			return "", retryAfter(response.Header.Get("Retry-After")),
				fmt.Errorf("%w: %w", ai_errors.ErrTemporaryFailure, statusError)
		}

		return "", 0, statusError
	}

	var result chatResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", 0, fmt.Errorf("decode DeepSeek response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", 0, ai_errors.ErrMissingChoice
	}

	choice := result.Choices[0]
	if choice.FinishReason == "length" {
		return "", 0, ai_errors.ErrTruncatedCompletion
	}

	content := sanitizeContent(choice.Message.Content)
	if content == "" {
		return "", 0, ai_errors.ErrMissingChoice
	}

	return content, 0, nil
}

func sanitizeContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	trimmed = strings.TrimPrefix(trimmed, "```")
	if index := strings.IndexByte(trimmed, '\n'); index >= 0 {
		if language := strings.TrimSpace(trimmed[:index]); language == "" || !strings.Contains(language, "{") {
			trimmed = trimmed[index+1:]
		}
	}

	if index := strings.LastIndex(trimmed, "```"); index >= 0 {
		trimmed = trimmed[:index]
	}

	return strings.TrimSpace(trimmed)
}

func retryableError(err error) bool {
	return errors.Is(err, ai_errors.ErrTemporaryFailure)
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests ||
		status == http.StatusRequestTimeout ||
		status >= http.StatusInternalServerError
}

func retryAfter(header string) time.Duration {
	trimmed := strings.TrimSpace(header)
	if trimmed == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(trimmed); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	if deadline, err := http.ParseTime(trimmed); err == nil {
		if wait := time.Until(deadline); wait > 0 {
			return wait
		}
	}

	return 0
}
