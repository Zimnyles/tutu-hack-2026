package mcp

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNoPayload     = errors.New("mcp: ответ инструмента не содержит JSON-нагрузки")
	ErrEmptyToolName = errors.New("mcp: имя инструмента пусто")
)

type ToolError struct {
	Tool   string
	Result *ToolResult
}

func (e *ToolError) Error() string {
	text := strings.TrimSpace(e.Result.Text())
	if text == "" {
		text = "сервер не пояснил причину"
	}

	return fmt.Sprintf("mcp: инструмент %s вернул ошибку: %s", e.Tool, text)
}

func (e *ToolError) Message() string {
	return strings.TrimSpace(e.Result.Text())
}

func AsToolError(err error) (*ToolError, bool) {
	var toolErr *ToolError
	ok := errors.As(err, &toolErr)

	return toolErr, ok
}
