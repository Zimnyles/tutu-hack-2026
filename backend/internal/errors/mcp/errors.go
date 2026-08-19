package mcp_errors

import "errors"

var (
	ErrOriginNotFound           = errors.New("origin city was not found")
	ErrNoOffers                 = errors.New("tutu MCP returned no transport offers")
	ErrCheckoutReferenceMissing = errors.New("tutu MCP checkout reference is missing")
	ErrCheckoutURLNotAllowed    = errors.New("tutu MCP checkout url is not allowed")
)
