package mcp

import (
	"encoding/json"
	"strings"
)

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Title   string `json:"title,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Title   string `json:"title,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"capabilities,omitempty"`
	ServerInfo      ServerInfo      `json:"serverInfo"`
	Instructions    string          `json:"instructions,omitempty"`
}

type Tool struct {
	Name         string          `json:"name"`
	Title        string          `json:"title,omitempty"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"inputSchema,omitempty"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
}

type Content struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	MIMEType string          `json:"mimeType,omitempty"`
	URI      string          `json:"uri,omitempty"`
	Data     string          `json:"data,omitempty"`
	Resource json.RawMessage `json:"resource,omitempty"`
}

type ToolResult struct {
	Content           []Content       `json:"content,omitempty"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
}

func (r *ToolResult) Text() string {
	if r == nil {
		return ""
	}

	parts := make([]string, 0, len(r.Content))

	for _, item := range r.Content {
		if item.Text != "" {
			parts = append(parts, item.Text)
		}
	}

	return strings.Join(parts, "\n")
}

func (r *ToolResult) Payload() (json.RawMessage, error) {
	if r == nil {
		return nil, ErrNoPayload
	}

	if raw := strings.TrimSpace(string(r.StructuredContent)); raw != "" && raw != "null" {
		return json.RawMessage(raw), nil
	}

	for _, item := range r.Content {
		text := strings.TrimSpace(item.Text)
		if text != "" && json.Valid([]byte(text)) {
			return json.RawMessage(text), nil
		}
	}

	return nil, ErrNoPayload
}

func (r *ToolResult) Decode(v any) error {
	payload, err := r.Payload()
	if err != nil {
		return err
	}

	return json.Unmarshal(payload, v)
}

type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

type ResourceContents struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}
