package transport

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	ErrSessionExpired     = errors.New("transport: сессия MCP истекла")
	ErrStreamClosed       = errors.New("transport: поток закрыт без ответа")
	ErrEmptyResponse      = errors.New("transport: пустой ответ")
	ErrMismatchedResponse = errors.New("transport: ответ не соответствует запросу")
	ErrTooManyRedirects   = errors.New("transport: слишком много перенаправлений")
	ErrRedirectNotAllowed = errors.New("transport: перенаправление на другой хост запрещено")
)

const errorTextLimit = 512

type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	body := strings.TrimSpace(e.Body)
	if len(body) > errorTextLimit {
		body = body[:errorTextLimit] + "…"
	}

	if body == "" {
		return fmt.Sprintf("transport: http %s", e.Status)
	}

	return fmt.Sprintf("transport: http %s: %s", e.Status, body)
}

func (e *HTTPError) Temporary() bool {
	switch {
	case e.StatusCode == http.StatusRequestTimeout,
		e.StatusCode == http.StatusTooManyRequests:
		return true
	case e.StatusCode >= 500 && e.StatusCode != http.StatusNotImplemented:
		return true
	default:
		return false
	}
}

func (e *HTTPError) Rejected() bool {
	return e.StatusCode == http.StatusTooManyRequests ||
		e.StatusCode == http.StatusServiceUnavailable
}

func StatusCode(err error) (int, bool) {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode, true
	}

	return 0, false
}
