package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	HeaderSessionID = "Mcp-Session-Id"

	HeaderProtocolVersion = "MCP-Protocol-Version"

	contentTypeJSON = "application/json"
	contentTypeSSE  = "text/event-stream"

	defaultMaxResponseBytes = 64 << 20
	errorBodyLimit          = 8 << 10
	defaultHTTPTimeout      = 60 * time.Second
	logPayloadLimit         = 512
	maximumRedirects        = 3
)

var mutatingToolMarkers = []string{ //nolint:gochecknoglobals // static allowlist.
	"checkout",
	"book",
	"order",
	"create",
	"cancel",
	"pay",
	"reserve",
}

type Config struct {
	Endpoint         string
	HTTPClient       *http.Client
	Header           http.Header
	Retry            RetryPolicy
	Logger           *slog.Logger
	MaxResponseBytes int64
}

type Streamable struct {
	endpoint   string
	httpClient *http.Client
	header     http.Header
	retry      RetryPolicy
	logger     *slog.Logger
	maxBytes   int64

	ids atomic.Int64

	mu        sync.RWMutex
	sessionID string
	protocol  string
}

func NewStreamable(cfg Config) *Streamable {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	if cfg.HTTPClient.CheckRedirect == nil {
		guarded := *cfg.HTTPClient
		guarded.CheckRedirect = restrictRedirects
		cfg.HTTPClient = &guarded
	}

	if cfg.Header == nil {
		cfg.Header = http.Header{}
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.New(discardHandler{})
	}

	if cfg.MaxResponseBytes <= 0 {
		cfg.MaxResponseBytes = defaultMaxResponseBytes
	}

	return &Streamable{
		endpoint:   cfg.Endpoint,
		httpClient: cfg.HTTPClient,
		header:     cfg.Header.Clone(),
		retry:      cfg.Retry.normalized(),
		logger:     cfg.Logger,
		maxBytes:   cfg.MaxResponseBytes,
	}
}

func (t *Streamable) Endpoint() string { return t.endpoint }

func (t *Streamable) SessionID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.sessionID
}

func (t *Streamable) SetProtocolVersion(version string) {
	t.mu.Lock()
	t.protocol = version
	t.mu.Unlock()
}

func (t *Streamable) ResetSession() {
	t.mu.Lock()
	t.sessionID = ""
	t.mu.Unlock()
}

func (t *Streamable) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := t.ids.Add(1)

	resp, err := t.do(ctx, NewRequest(id, method, params))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	message, err := t.decode(ctx, resp, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}

	if message.Error != nil {
		return nil, message.Error
	}

	return message.Result, nil
}

func (t *Streamable) Notify(ctx context.Context, method string, params any) error {
	resp, err := t.do(ctx, NewNotification(method, params))
	if err != nil {
		return err
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, errorBodyLimit))

	if err := resp.Body.Close(); err != nil {
		return fmt.Errorf("transport: close notification response: %w", err)
	}

	return nil
}

func (t *Streamable) EndSession(ctx context.Context) error {
	sessionID := t.SessionID()
	t.ResetSession()

	if sessionID == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, t.endpoint, nil)
	if err != nil {
		return fmt.Errorf("transport: закрытие сессии: %w", err)
	}

	t.applyHeaders(req)
	req.Header.Set(HeaderSessionID, sessionID)

	resp, err := t.httpClient.Do(req) //nolint:gosec // Endpoint is operator-controlled and validated by WithEndpoint.
	if err != nil {
		return fmt.Errorf("transport: закрытие сессии: %w", err)
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, errorBodyLimit))

	if err := resp.Body.Close(); err != nil {
		return fmt.Errorf("transport: close session response: %w", err)
	}

	return nil
}

func (t *Streamable) do(ctx context.Context, msg Request) (*http.Response, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("transport: сериализация %s: %w", msg.Method, err)
	}

	idempotent := idempotentMessage(msg)

	var lastErr error

	for attempt := 1; attempt <= t.retry.MaxAttempts; attempt++ {
		if attempt > 1 {
			delay := t.retry.delay(attempt-1, retryAfterOf(lastErr))
			t.logger.Debug("повтор запроса MCP",
				slog.String("method", msg.Method),
				slog.Int("attempt", attempt),
				slog.Duration("delay", delay),
				slog.String("cause", lastErr.Error()))

			if err := sleep(ctx, delay); err != nil {
				return nil, err
			}
		}

		resp, err := t.attempt(ctx, body)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		if !retryable(err, idempotent) {
			return nil, err
		}
	}

	return nil, lastErr
}

func restrictRedirects(request *http.Request, previous []*http.Request) error {
	if len(previous) >= maximumRedirects {
		return ErrTooManyRedirects
	}

	origin := previous[0].URL
	if request.URL.Scheme != origin.Scheme || request.URL.Host != origin.Host {
		return fmt.Errorf("%w: %s", ErrRedirectNotAllowed, request.URL.Host)
	}

	return nil
}

func idempotentMessage(msg Request) bool {
	if msg.Method != "tools/call" {
		return true
	}

	parameters, ok := msg.Params.(map[string]any)
	if !ok {
		return false
	}

	name, ok := parameters["name"].(string)
	if !ok {
		return false
	}

	return !mutatingTool(name)
}

func mutatingTool(name string) bool {
	lowered := strings.ToLower(name)
	for _, marker := range mutatingToolMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}

	return false
}

func (t *Streamable) attempt(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("transport: подготовка запроса: %w", err)
	}

	t.applyHeaders(req)
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("Accept", contentTypeJSON+", "+contentTypeSSE)

	started := time.Now()

	resp, err := t.httpClient.Do(req) //nolint:gosec // Endpoint is operator-controlled and validated by WithEndpoint.
	if err != nil {
		return nil, fmt.Errorf("transport: запрос к %s: %w", t.endpoint, err)
	}

	t.logger.Debug("ответ MCP",
		slog.Int("status", resp.StatusCode),
		slog.Duration("elapsed", time.Since(started)))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if id := resp.Header.Get(HeaderSessionID); id != "" {
			t.mu.Lock()
			t.sessionID = id
			t.mu.Unlock()
		}

		return resp, nil
	}

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyLimit))
	closeBody(resp.Body)

	httpErr := &HTTPError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       string(bytes.TrimSpace(raw)),
		RetryAfter: retryAfterDuration(resp.Header.Get("Retry-After")),
	}
	if resp.StatusCode == http.StatusNotFound && t.SessionID() != "" {
		return nil, fmt.Errorf("%w: %w", ErrSessionExpired, httpErr)
	}

	return nil, httpErr
}

func (t *Streamable) applyHeaders(req *http.Request) {
	for key, values := range t.header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	t.mu.RLock()
	sessionID, protocol := t.sessionID, t.protocol
	t.mu.RUnlock()

	if sessionID != "" {
		req.Header.Set(HeaderSessionID, sessionID)
	}

	if protocol != "" {
		req.Header.Set(HeaderProtocolVersion, protocol)
	}
}

func (t *Streamable) decode(ctx context.Context, resp *http.Response, id int64) (*Response, error) {
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		mediaType = contentTypeJSON
	}

	limited := io.LimitReader(resp.Body, t.maxBytes)

	if mediaType == contentTypeSSE {
		return decodeStream(ctx, limited, id)
	}

	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("transport: чтение тела ответа: %w", err)
	}

	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: http %s", ErrEmptyResponse, resp.Status)
	}

	if message, ok := decodeEvent(raw, id); ok {
		return message, nil
	}

	return nil, fmt.Errorf("%w: request %d: %s", ErrMismatchedResponse, id, truncate(raw))
}

func retryAfterOf(err error) time.Duration {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.RetryAfter
	}

	return 0
}

func truncate(raw []byte) string {
	if len(raw) > logPayloadLimit {
		return string(raw[:logPayloadLimit]) + "…"
	}

	return string(raw)
}

func closeBody(body io.ReadCloser) {
	if body == nil {
		return
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(body, errorBodyLimit))
	_ = body.Close()
}

type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (h discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return h }
func (h discardHandler) WithGroup(string) slog.Handler           { return h }
