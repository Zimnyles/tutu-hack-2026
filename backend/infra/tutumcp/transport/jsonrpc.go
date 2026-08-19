package transport

import (
	"encoding/json"
	"fmt"
)

const Version = "2.0"

const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int64 `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

func NewRequest(id int64, method string, params any) Request {
	return Request{JSONRPC: Version, ID: &id, Method: method, Params: params}
}

func NewNotification(method string, params any) Request {
	return Request{JSONRPC: Version, Method: method, Params: params}
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id"`
	Method  string          `json:"method"`
	Result  json.RawMessage `json:"result"`
	Error   *RPCError       `json:"error"`
}

func (r *Response) answersTo(id int64) bool {
	return r.Method == "" && r.ID != nil && *r.ID == id
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	if len(e.Data) > 0 {
		return fmt.Sprintf("jsonrpc: %s (код %d, данные %s)", e.Message, e.Code, e.Data)
	}

	return fmt.Sprintf("jsonrpc: %s (код %d)", e.Message, e.Code)
}
