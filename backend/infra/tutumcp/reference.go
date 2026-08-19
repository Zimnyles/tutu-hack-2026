package tutumcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/tutu-hack/openworld/infra/tutumcp/mcp"
)

func (c *Client) Instructions(ctx context.Context, domain Domain) (string, error) {
	if domain == "" {
		return "", &ValidationError{Field: "domain", Reason: "не заполнено"}
	}

	result, err := c.mcp.CallTool(ctx, "get_"+string(domain)+"_instructions", nil)
	if err != nil {
		return "", err
	}

	return result.Text(), nil
}

func (c *Client) FetchResource(ctx context.Context, uri string) (string, error) {
	if !strings.HasPrefix(uri, "tutu://") {
		return "", &ValidationError{Field: "uri", Reason: fmt.Sprintf("ожидается адрес вида tutu://…, получено %q", uri)}
	}

	result, err := c.mcp.CallTool(ctx, ToolFetchResource, map[string]any{"uri": uri})
	if err != nil {
		return "", err
	}

	return result.Text(), nil
}

type Resource = mcp.Resource

type ResourceContents = mcp.ResourceContents

func (c *Client) ListResources(ctx context.Context) ([]Resource, error) {
	return c.mcp.ListResources(ctx)
}

func (c *Client) ReadResource(ctx context.Context, uri string) ([]ResourceContents, error) {
	return c.mcp.ReadResource(ctx, uri)
}
