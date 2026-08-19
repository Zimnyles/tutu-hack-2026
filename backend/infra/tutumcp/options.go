package tutumcp

import (
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/tutu-hack/openworld/infra/tutumcp/mcp"
	"github.com/tutu-hack/openworld/infra/tutumcp/transport"
)

type config struct {
	endpoint   string
	httpClient *http.Client
	header     http.Header
	info       mcp.ClientInfo
	protocol   string
	retry      transport.RetryPolicy
	logger     *slog.Logger
	maxBytes   int64
}

type Option func(*config) error

func WithEndpoint(endpoint string) Option {
	return func(c *config) error {
		if endpoint == "" {
			return ErrEmptyEndpoint
		}

		parsed, err := url.ParseRequestURI(endpoint)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return ErrInvalidEndpoint
		}

		c.endpoint = endpoint

		return nil
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *config) error {
		if client == nil {
			return ErrNilHTTPClient
		}

		c.httpClient = client

		return nil
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *config) error {
		if timeout <= 0 {
			return ErrInvalidTimeout
		}

		c.httpClient = &http.Client{Timeout: timeout, Transport: c.httpClient.Transport}

		return nil
	}
}

func WithClientInfo(name, version string) Option {
	return func(c *config) error {
		if name == "" {
			return ErrEmptyClientName
		}

		c.info.Name = name
		c.info.Version = version

		return nil
	}
}

func WithProtocolVersion(version string) Option {
	return func(c *config) error {
		if version == "" {
			return ErrEmptyProtocolVersion
		}

		c.protocol = version

		return nil
	}
}

func WithHeader(key, value string) Option {
	return func(c *config) error {
		if key == "" {
			return ErrEmptyHeaderName
		}

		c.header.Set(key, value)

		return nil
	}
}

func WithRetry(policy transport.RetryPolicy) Option {
	return func(c *config) error {
		c.retry = policy

		return nil
	}
}

func WithoutRetry() Option {
	return func(c *config) error {
		c.retry.MaxAttempts = 1

		return nil
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(c *config) error {
		c.logger = logger

		return nil
	}
}

func WithMaxResponseBytes(limit int64) Option {
	return func(c *config) error {
		if limit <= 0 {
			return ErrInvalidMaxResponseSize
		}

		c.maxBytes = limit

		return nil
	}
}
