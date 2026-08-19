package tutu_transport

import (
	"encoding/json"
	"testing"

	"github.com/tutu-hack/openworld/infra/tutumcp"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

func TestCheapestValidOfferDoesNotUseInvalidCheapest(t *testing.T) {
	recommendation := domain.RecommendationRequest{
		Budget: 10_000, Currency: "RUB", TransportModes: []string{"railway"}, MaxTravelMinutes: 600,
	}
	offers := []tutumcp.Offer{
		{
			Transport: "railway", Price: &tutumcp.Price{Amount: json.Number("500")},
			DurationMin: 100, DepartureAt: "2026-09-01T10:00:00+05:00", ArrivalAt: "2026-09-01T12:00:00+05:00",
		},
		{
			OfferID: "valid", Transport: "railway", Price: &tutumcp.Price{Amount: json.Number("1500"), Currency: "RUB"},
			DurationMin: 120, DepartureAt: "2026-09-01T10:00:00+05:00", ArrivalAt: "2026-09-01T12:00:00+05:00",
			CheckoutRef: json.RawMessage(`{"ref":"opaque"}`),
		},
	}

	offer, found := cheapestValidOffer(offers, recommendation)
	if !found || offer.OfferID != "valid" {
		t.Fatalf("expected valid offer, got found=%v offer=%#v", found, offer)
	}
}
