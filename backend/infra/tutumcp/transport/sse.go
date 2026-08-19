package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

const (
	sseInitialBuffer = 64 << 10
	sseLineLimit     = 8 << 20
)

func decodeStream(ctx context.Context, body io.Reader, id int64) (*Response, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, sseInitialBuffer), sseLineLimit)

	var event []byte

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		line := bytes.TrimSuffix(scanner.Bytes(), []byte("\r"))

		switch {
		case len(line) == 0:
			if resp, ok := decodeEvent(event, id); ok {
				return resp, nil
			}

			event = event[:0]
		case bytes.HasPrefix(line, []byte(":")):

		case bytes.HasPrefix(line, []byte("data:")):
			if len(event) > 0 {
				event = append(event, '\n')
			}

			event = append(event, bytes.TrimPrefix(bytes.TrimPrefix(line, []byte("data:")), []byte(" "))...)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("transport: чтение потока событий: %w", err)
	}

	if resp, ok := decodeEvent(event, id); ok {
		return resp, nil
	}

	return nil, fmt.Errorf("%w: request %d", ErrStreamClosed, id)
}

func decodeEvent(payload []byte, id int64) (*Response, bool) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil, false
	}

	var resp Response
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, false
	}

	if !resp.answersTo(id) {
		return nil, false
	}

	return &resp, true
}
