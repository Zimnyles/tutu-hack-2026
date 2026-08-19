package tutumcp

import (
	"errors"
	"fmt"

	"github.com/tutu-hack/openworld/infra/tutumcp/mcp"
	"github.com/tutu-hack/openworld/infra/tutumcp/transport"
)

type (
	RPCError = transport.RPCError

	HTTPError = transport.HTTPError

	ToolError = mcp.ToolError
)

var ErrNoPayload = mcp.ErrNoPayload

var ErrSessionExpired = transport.ErrSessionExpired

var (
	ErrEmptyEndpoint          = errors.New("tutumcp: endpoint is empty")
	ErrInvalidEndpoint        = errors.New("tutumcp: endpoint is invalid")
	ErrNilHTTPClient          = errors.New("tutumcp: HTTP client is nil")
	ErrInvalidTimeout         = errors.New("tutumcp: timeout must be positive")
	ErrEmptyClientName        = errors.New("tutumcp: client name is empty")
	ErrEmptyProtocolVersion   = errors.New("tutumcp: protocol version is empty")
	ErrEmptyHeaderName        = errors.New("tutumcp: header name is empty")
	ErrInvalidMaxResponseSize = errors.New("tutumcp: maximum response size must be positive")
)

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("tutumcp: параметр %s: %s", e.Field, e.Reason)
}

func AsValidationError(err error) (*ValidationError, bool) {
	var validationErr *ValidationError
	ok := errors.As(err, &validationErr)

	return validationErr, ok
}

func AsToolError(err error) (*ToolError, bool) {
	return mcp.AsToolError(err)
}

func AsRPCError(err error) (*RPCError, bool) {
	var rpcErr *RPCError
	ok := errors.As(err, &rpcErr)

	return rpcErr, ok
}

func AsHTTPError(err error) (*HTTPError, bool) {
	var httpErr *HTTPError
	ok := errors.As(err, &httpErr)

	return httpErr, ok
}
