package tutumcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tutu-hack/openworld/infra/tutumcp/mcp"
	"github.com/tutu-hack/openworld/infra/tutumcp/transport"
)

const DefaultEndpoint = "https://mcp.tutu.ru/mcp"

const DefaultTimeout = 90 * time.Second

type ServerInfo struct {
	Name            string
	Version         string
	ProtocolVersion string
	Instructions    string
}

type Tool = mcp.Tool

type ToolResult = mcp.ToolResult

type Client struct {
	mcp *mcp.Client
}

func New(opts ...Option) (*Client, error) {
	cfg := &config{
		endpoint:   DefaultEndpoint,
		httpClient: &http.Client{Timeout: DefaultTimeout},
		header:     http.Header{},
		info:       mcp.ClientInfo{Name: "tutumcp-go", Version: "1.0.0"},
		protocol:   mcp.Version20251125,
		retry:      transport.DefaultRetryPolicy(),
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	tr := transport.NewStreamable(transport.Config{
		Endpoint:         cfg.endpoint,
		HTTPClient:       cfg.httpClient,
		Header:           cfg.header,
		Retry:            cfg.retry,
		Logger:           cfg.logger,
		MaxResponseBytes: cfg.maxBytes,
	})

	return &Client{mcp: mcp.NewClient(tr, cfg.info, cfg.protocol)}, nil
}

func (c *Client) MCP() *mcp.Client { return c.mcp }

func (c *Client) Endpoint() string { return c.mcp.Transport().Endpoint() }

func (c *Client) Initialize(ctx context.Context) (ServerInfo, error) {
	result, err := c.mcp.Initialize(ctx)
	if err != nil {
		return ServerInfo{}, err
	}

	return ServerInfo{
		Name:            result.ServerInfo.Name,
		Version:         result.ServerInfo.Version,
		ProtocolVersion: result.ProtocolVersion,
		Instructions:    result.Instructions,
	}, nil
}

func (c *Client) Ping(ctx context.Context) error { return c.mcp.Ping(ctx) }

func (c *Client) Close(ctx context.Context) error { return c.mcp.Close(ctx) }

func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	return c.mcp.ListTools(ctx)
}

func (c *Client) CallTool(ctx context.Context, name string, arguments any) (*ToolResult, error) {
	return c.mcp.CallTool(ctx, name, arguments)
}

func (c *Client) callJSON(ctx context.Context, tool string, params validator) (json.RawMessage, error) {
	if err := validate(params); err != nil {
		return nil, err
	}

	result, err := c.mcp.CallTool(ctx, tool, params)
	if err != nil {
		return nil, err
	}

	payload, err := result.Payload()
	if err != nil {
		return nil, fmt.Errorf("tutumcp: %s: %w", tool, err)
	}

	return payload, nil
}
