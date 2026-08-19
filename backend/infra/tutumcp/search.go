package tutumcp

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) SearchAvia(ctx context.Context, params AviaParams) (*SearchResult, error) {
	return c.search(ctx, ToolSearchAvia, params)
}

func (c *Client) SearchRail(ctx context.Context, params RailParams) (*SearchResult, error) {
	return c.search(ctx, ToolSearchRail, params)
}

func (c *Client) SearchBus(ctx context.Context, params BusParams) (*SearchResult, error) {
	return c.search(ctx, ToolSearchBus, params)
}

func (c *Client) SearchEtrain(ctx context.Context, params EtrainParams) (*SearchResult, error) {
	return c.search(ctx, ToolSearchEtrain, params)
}

func (c *Client) SearchMultitransport(ctx context.Context, params MultitransportParams) (*SearchResult, error) {
	return c.search(ctx, ToolSearchMultitransport, params)
}

func (c *Client) SearchHotels(ctx context.Context, params HotelParams) (*SearchResult, error) {
	return c.search(ctx, ToolSearchHotels, params)
}

func (c *Client) search(ctx context.Context, tool string, params validator) (*SearchResult, error) {
	payload, err := c.callJSON(ctx, tool, params)
	if err != nil {
		return nil, err
	}

	var result SearchResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("tutumcp: разбор ответа %s: %w", tool, err)
	}

	result.Raw = payload

	return &result, nil
}

type PageFunc func(ctx context.Context, page int) (*SearchResult, error)

func Paginate(ctx context.Context, maxPages int, fetch PageFunc, visit func(*SearchResult) error) error {
	if maxPages <= 0 || maxPages > maxPage {
		maxPages = maxPage
	}

	for page := 1; page <= maxPages; page++ {
		result, err := fetch(ctx, page)
		if err != nil {
			return err
		}

		if err := visit(result); err != nil {
			return err
		}

		if !result.HasMore() {
			return nil
		}
	}

	return nil
}
