package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/tutu-hack/openworld/infra/tutumcp/transport"
)

const (
	Version20241105 = "2024-11-05"
	Version20250326 = "2025-03-26"
	Version20250618 = "2025-06-18"
	Version20251125 = "2025-11-25"
)

type Client struct {
	transport *transport.Streamable
	info      ClientInfo
	protocol  string

	handshake sync.Mutex

	mu          sync.RWMutex
	initialized bool
	server      InitializeResult
}

func NewClient(tr *transport.Streamable, info ClientInfo, protocolVersion string) *Client {
	if protocolVersion == "" {
		protocolVersion = Version20251125
	}

	return &Client{transport: tr, info: info, protocol: protocolVersion}
}

func (c *Client) Transport() *transport.Streamable { return c.transport }

func (c *Client) Initialize(ctx context.Context) (InitializeResult, error) {
	c.handshake.Lock()
	defer c.handshake.Unlock()

	return c.initializeLocked(ctx)
}

func (c *Client) initializeLocked(ctx context.Context) (InitializeResult, error) {
	c.mu.RLock()
	if c.initialized {
		server := c.server
		c.mu.RUnlock()

		return server, nil
	}
	c.mu.RUnlock()

	params := map[string]any{
		"protocolVersion": c.protocol,
		"capabilities":    map[string]any{},
		"clientInfo":      c.info,
	}

	raw, err := c.transport.Call(ctx, "initialize", params)
	if err != nil {
		return InitializeResult{}, err
	}

	var result InitializeResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return InitializeResult{}, fmt.Errorf("mcp: разбор ответа initialize: %w", err)
	}

	version := result.ProtocolVersion
	if version == "" {
		version = c.protocol
	}

	c.transport.SetProtocolVersion(version)

	c.mu.Lock()
	c.initialized = true
	c.server = result
	c.mu.Unlock()

	if err := c.transport.Notify(ctx, "notifications/initialized", nil); err != nil {
		return result, fmt.Errorf("mcp: уведомление initialized: %w", err)
	}

	return result, nil
}

func (c *Client) ServerInfo() (InitializeResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.server, c.initialized
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.call(ctx, "ping", map[string]any{})

	return err
}

func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	c.initialized = false
	c.mu.Unlock()

	return c.transport.EndSession(ctx)
}

func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var (
		tools  []Tool
		cursor string
	)

	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}

		raw, err := c.call(ctx, "tools/list", params)
		if err != nil {
			return nil, err
		}

		var page struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}

		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("mcp: разбор tools/list: %w", err)
		}

		tools = append(tools, page.Tools...)
		if page.NextCursor == "" || page.NextCursor == cursor {
			return tools, nil
		}

		cursor = page.NextCursor
	}
}

func (c *Client) CallTool(ctx context.Context, name string, arguments any) (*ToolResult, error) {
	if name == "" {
		return nil, ErrEmptyToolName
	}

	if arguments == nil {
		arguments = map[string]any{}
	}

	raw, err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": arguments})
	if err != nil {
		return nil, err
	}

	var result ToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("mcp: разбор ответа %s: %w", name, err)
	}

	if result.IsError {
		return &result, &ToolError{Tool: name, Result: &result}
	}

	return &result, nil
}

func (c *Client) ListResources(ctx context.Context) ([]Resource, error) {
	var (
		resources []Resource
		cursor    string
	)

	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}

		raw, err := c.call(ctx, "resources/list", params)
		if err != nil {
			return nil, err
		}

		var page struct {
			Resources  []Resource `json:"resources"`
			NextCursor string     `json:"nextCursor"`
		}

		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("mcp: разбор resources/list: %w", err)
		}

		resources = append(resources, page.Resources...)
		if page.NextCursor == "" || page.NextCursor == cursor {
			return resources, nil
		}

		cursor = page.NextCursor
	}
}

func (c *Client) ReadResource(ctx context.Context, uri string) ([]ResourceContents, error) {
	raw, err := c.call(ctx, "resources/read", map[string]any{"uri": uri})
	if err != nil {
		return nil, err
	}

	var result struct {
		Contents []ResourceContents `json:"contents"`
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("mcp: разбор resources/read: %w", err)
	}

	return result.Contents, nil
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if _, err := c.Initialize(ctx); err != nil {
		return nil, err
	}

	raw, err := c.transport.Call(ctx, method, params)
	if err == nil || !errors.Is(err, transport.ErrSessionExpired) {
		return raw, err
	}

	c.handshake.Lock()
	c.mu.Lock()
	c.initialized = false
	c.mu.Unlock()
	c.transport.ResetSession()
	_, initErr := c.initializeLocked(ctx)
	c.handshake.Unlock()

	if initErr != nil {
		return nil, initErr
	}

	return c.transport.Call(ctx, method, params)
}
