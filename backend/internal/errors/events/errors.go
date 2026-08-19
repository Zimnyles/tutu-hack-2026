package events_errors

import "errors"

var (
	ErrNotFound          = errors.New("event not found")
	ErrUnavailable       = errors.New("event is unavailable")
	ErrCatalogStale      = errors.New("event catalog is stale")
	ErrUnknownEnrichment = errors.New("AI event enrichment references unknown event")
)
