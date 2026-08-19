package trips

import (
	"context"
	"fmt"
	"time"

	recommendation_errors "github.com/tutu-hack/openworld/internal/errors/recommendation"
	trips_errors "github.com/tutu-hack/openworld/internal/errors/trips"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

var checkoutAllowedStatuses = map[string]bool{ //nolint:gochecknoglobals // static allowlist.
	"planned":          true,
	"checkout_created": true,
}

func expired(validUntil string) bool {
	deadline, err := time.Parse(time.RFC3339, validUntil)
	if err != nil {
		return false
	}

	return deadline.Before(time.Now())
}

type Service struct {
	repository Repository
	checkout   CheckoutCreator
	arrival    ArrivalProcessor
}

func New(
	repository Repository,
	checkout CheckoutCreator,
	arrival ArrivalProcessor,
) *Service {
	return &Service{
		repository: repository,
		checkout:   checkout,
		arrival:    arrival,
	}
}

func (s *Service) SelectOption(
	ctx context.Context,
	userID string,
	recommendationID string,
	optionID string,
) (domain.Trip, error) {
	trip, err := s.repository.CreateFromRecommendation(
		ctx,
		userID,
		recommendationID,
		optionID,
	)
	if err != nil {
		return domain.Trip{}, fmt.Errorf("create trip from recommendation: %w", err)
	}

	return trip, nil
}

func (s *Service) List(ctx context.Context, userID string) ([]domain.Trip, error) {
	items, err := s.repository.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list trips: %w", err)
	}

	return items, nil
}

func (s *Service) CreateCheckout(
	ctx context.Context,
	userID string,
	tripID string,
) (domain.Trip, string, bool, error) {
	trip, found, err := s.repository.Get(ctx, tripID, userID)
	if err != nil {
		return domain.Trip{}, "", false, fmt.Errorf("get trip for checkout: %w", err)
	}

	if !found {
		return domain.Trip{}, "", false, trips_errors.ErrTripNotFound
	}

	if !checkoutAllowedStatuses[trip.Status] {
		return domain.Trip{}, "", false, trips_errors.ErrInvalidState
	}

	if expired(trip.Option.ValidUntil) {
		return domain.Trip{}, "", false, recommendation_errors.ErrExpired
	}

	checkoutURL, kind, demo, err := s.checkout.Create(ctx, trip)
	if err != nil {
		return domain.Trip{}, "", false, fmt.Errorf("create checkout: %w", err)
	}

	updatedTrip, err := s.repository.SaveCheckout(ctx, tripID, userID, checkoutURL)
	if err != nil {
		return domain.Trip{}, "", false, fmt.Errorf("save checkout: %w", err)
	}

	return updatedTrip, kind, demo, nil
}

func (s *Service) SimulateArrival(
	ctx context.Context,
	userID string,
	tripID string,
	idempotencyKey string,
) (domain.Trip, domain.LedgerEntry, bool, error) {
	trip, reward, replayed, err := s.arrival.Arrive(
		ctx,
		tripID,
		userID,
		idempotencyKey,
	)
	if err != nil {
		return domain.Trip{}, domain.LedgerEntry{}, false, fmt.Errorf("simulate arrival: %w", err)
	}

	return trip, reward, replayed, nil
}
