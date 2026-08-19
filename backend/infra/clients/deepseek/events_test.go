package deepseek

import (
	"context"
	"testing"
	"time"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

func TestEventEnricherRejectsInventedEvent(t *testing.T) {
	enricher := NewEventEnricher("key", "https://deepseek.invalid", "model", "strict event prompt")
	enricher.transport.httpClient = testHTTPClient(t, `{"events":[{"event_id":"invented","category":"concert","description":"Описание"}]}`)

	_, err := enricher.Enrich(context.Background(), domain.Territory{Name: "Казань"}, []domain.Event{{
		ID: "trusted", Title: "Концерт", StartsAt: time.Now().Add(time.Hour), TrustStatus: "verified",
	}})
	if err == nil {
		t.Fatal("expected invented event id to be rejected")
	}
}
