package tutumcp

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) GetOfferDetails(ctx context.Context, params DetailsParams) (*OfferDetails, error) {
	payload, err := c.callJSON(ctx, ToolGetOfferDetails, params)
	if err != nil {
		return nil, err
	}

	return &OfferDetails{Raw: payload}, nil
}

func (c *Client) OfferDetailsFor(ctx context.Context, product ProductType, offer Offer) (*OfferDetails, error) {
	return c.GetOfferDetails(ctx, DetailsParams{ProductType: product, DetailsRef: offer.DetailsRef})
}

func (c *Client) GetRailSeatmap(ctx context.Context, params SeatmapParams) (*RailSeatmap, error) {
	payload, err := c.callJSON(ctx, ToolGetRailSeatmap, params)
	if err != nil {
		return nil, err
	}

	var seatmap RailSeatmap
	if err := json.Unmarshal(payload, &seatmap); err != nil {
		return nil, fmt.Errorf("tutumcp: разбор ответа %s: %w", ToolGetRailSeatmap, err)
	}

	seatmap.Raw = payload

	return &seatmap, nil
}

func (c *Client) CreateCheckoutLink(ctx context.Context, params CheckoutParams) (*CheckoutLink, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	args, err := params.arguments()
	if err != nil {
		return nil, err
	}

	result, err := c.mcp.CallTool(ctx, ToolCreateCheckoutLink, args)
	if err != nil {
		return nil, err
	}

	payload, err := result.Payload()
	if err != nil {
		return nil, fmt.Errorf("tutumcp: %s: %w", ToolCreateCheckoutLink, err)
	}

	var link CheckoutLink
	if err := json.Unmarshal(payload, &link); err != nil {
		return nil, fmt.Errorf("tutumcp: разбор ответа %s: %w", ToolCreateCheckoutLink, err)
	}

	link.Raw = payload

	return &link, nil
}

func (c *Client) CheckoutLinkFor(ctx context.Context, product ProductType, offer Offer) (*CheckoutLink, error) {
	return c.CreateCheckoutLink(ctx, CheckoutParams{ProductType: product, CheckoutRef: offer.CheckoutRef})
}
