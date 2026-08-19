package events

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"time"

	ai_errors "github.com/tutu-hack/openworld/internal/errors/ai"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

const (
	cityScope       = "city"
	countryScope    = "country"
	countryScopeKey = "russia"
	busyBackoff     = 2 * time.Minute
	prewarmPause    = 10 * time.Second
	eventRetention  = 7
	slotWaitTimeout = 30 * time.Second
)

type Listing struct {
	Items       []domain.Event
	Discovering bool
	RefreshedAt time.Time
}

func (s *Service) ListCity(
	ctx context.Context,
	city domain.Territory,
	filters Filters,
) (Listing, error) {
	items, err := s.List(ctx, city.ID, filters)
	if err != nil {
		return Listing{}, err
	}

	state, discovering := s.ensureCityDiscovery(ctx, city)

	return Listing{Items: items, Discovering: discovering, RefreshedAt: state.RefreshedAt}, nil
}

func (s *Service) Popular(ctx context.Context) (Listing, error) {
	items, err := s.repository.PopularEvents(ctx, s.config.PopularLimit)
	if err != nil {
		return Listing{}, err
	}

	state, discovering := s.ensurePopularDiscovery(ctx)

	return Listing{Items: items, Discovering: discovering, RefreshedAt: state.RefreshedAt}, nil
}

func (s *Service) PrewarmCities(ctx context.Context) {
	if !s.config.DiscoveryEnabled || s.discoverer == nil || s.config.PrewarmCities == 0 {
		return
	}

	cities, err := s.repository.DiscoveryCities(ctx, s.config.PrewarmCities)
	if err != nil {
		s.logger.Warn("read prewarm cities", "error", err)

		return
	}

	for _, city := range cities {
		if ctx.Err() != nil {
			return
		}

		if _, started := s.ensureCityDiscovery(ctx, city); !started {
			continue
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(prewarmPause):
		}
	}
}

func (s *Service) RefreshPopular(ctx context.Context) {
	if _, discovering := s.ensurePopularDiscovery(ctx); !discovering {
		return
	}
}

func (s *Service) ensureCityDiscovery(
	ctx context.Context,
	city domain.Territory,
) (domain.EventDiscoveryState, bool) {
	return s.ensureDiscovery(ctx, cityScope, city.ID, func(runContext context.Context) (int, error) {
		window := s.window()

		discovered, err := s.discoverer.DiscoverCity(
			runContext,
			city,
			window.from,
			window.to,
			s.config.CityLimit,
		)
		if err != nil {
			return 0, err
		}

		if err := s.repository.SaveCityDiscovery(
			runContext,
			city.ID,
			discovered,
			time.Now().Add(eventRetention*s.config.CacheTTL),
		); err != nil {
			return 0, err
		}

		return len(discovered), nil
	})
}

func (s *Service) ensurePopularDiscovery(
	ctx context.Context,
) (domain.EventDiscoveryState, bool) {
	return s.ensureDiscovery(
		ctx,
		countryScope,
		countryScopeKey,
		func(runContext context.Context) (int, error) {
			cities, err := s.repository.DiscoveryCities(runContext, s.config.PopularCityPool)
			if err != nil {
				return 0, err
			}

			window := s.window()

			discovered, err := s.discoverer.DiscoverPopular(
				runContext,
				cities,
				window.from,
				window.to,
				s.config.PopularLimit,
			)
			if err != nil {
				return 0, err
			}

			if err := s.repository.SavePopularDiscovery(
				runContext,
				discovered,
				time.Now().Add(eventRetention*s.config.CacheTTL),
			); err != nil {
				return 0, err
			}

			return len(discovered), nil
		},
	)
}

func (s *Service) ensureDiscovery(
	ctx context.Context,
	scope string,
	scopeKey string,
	run func(context.Context) (int, error),
) (domain.EventDiscoveryState, bool) {
	if !s.config.DiscoveryEnabled || s.discoverer == nil {
		return domain.EventDiscoveryState{}, false
	}

	state, found, err := s.repository.DiscoveryState(ctx, scope, scopeKey)
	if err != nil {
		s.logger.Warn("read event discovery state", "scope", scope, "key", scopeKey, "error", err)

		return domain.EventDiscoveryState{}, false
	}

	now := time.Now()

	if found {
		if state.Status == "running" && now.Sub(state.StartedAt) < s.config.RunTimeout {
			return state, true
		}

		if state.Status != "running" && state.ExpiresAt.After(now) {
			return state, false
		}
	}

	claimed, err := s.repository.ClaimDiscovery(ctx, scope, scopeKey, now.Add(-s.config.RunTimeout))
	if err != nil {
		s.logger.Warn("claim event discovery", "scope", scope, "key", scopeKey, "error", err)

		return state, false
	}

	if !claimed {
		return state, true
	}

	go s.runDiscovery(scope, scopeKey, run)

	return state, true
}

func (s *Service) runDiscovery(
	scope string,
	scopeKey string,
	run func(context.Context) (int, error),
) {
	runContext, cancel := context.WithTimeout(context.Background(), s.config.RunTimeout)
	defer cancel()

	defer func() {
		if recovered := recover(); recovered != nil {
			s.logger.Error(
				"event discovery panic",
				"scope", scope,
				"key", scopeKey,
				"panic", recovered,
				"stack", string(debug.Stack()),
			)
			s.failDiscovery(runContext, scope, scopeKey, "DISCOVERY_PANIC", busyBackoff)
		}
	}()

	if !s.acquireSlot(runContext) {
		s.failDiscovery(runContext, scope, scopeKey, "DISCOVERY_BUSY", busyBackoff)

		return
	}
	defer s.releaseSlot()

	started := time.Now()

	found, err := run(runContext)
	if err != nil {
		s.logger.Warn(
			"event discovery failed",
			"scope", scope,
			"key", scopeKey,
			"error", err,
			"duration", time.Since(started),
		)
		s.failDiscovery(runContext, scope, scopeKey, discoveryFailureCode(err), s.config.FailureBackoff)

		return
	}

	if err := s.repository.CompleteDiscovery(
		runContext,
		scope,
		scopeKey,
		found,
		time.Now().Add(s.config.CacheTTL),
	); err != nil {
		s.logger.Warn("complete event discovery", "scope", scope, "key", scopeKey, "error", err)

		return
	}

	s.logger.Info(
		"event discovery completed",
		"scope", scope,
		"key", scopeKey,
		"events", found,
		"duration", time.Since(started),
	)
}

func (s *Service) failDiscovery(
	ctx context.Context,
	scope string,
	scopeKey string,
	code string,
	backoff time.Duration,
) {
	failureContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), stateWriteTimeout)
	defer cancel()

	if err := s.repository.FailDiscovery(
		failureContext,
		scope,
		scopeKey,
		code,
		time.Now().Add(backoff),
	); err != nil {
		s.logger.Warn("mark event discovery failed", "scope", scope, "key", scopeKey, "error", err)
	}
}

func (s *Service) acquireSlot(ctx context.Context) bool {
	timer := time.NewTimer(slotWaitTimeout)
	defer timer.Stop()

	select {
	case s.slots <- struct{}{}:
		return true
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
}

func (s *Service) releaseSlot() {
	select {
	case <-s.slots:
	default:
	}
}

type discoveryWindow struct {
	from time.Time
	to   time.Time
}

func (s *Service) window() discoveryWindow {
	now := time.Now().UTC()

	return discoveryWindow{from: now, to: now.AddDate(0, 0, s.config.WindowDays)}
}

func discoveryFailureCode(err error) string {
	switch {
	case errors.Is(err, ai_errors.ErrEmptyDiscovery):
		return "DISCOVERY_EMPTY"
	case errors.Is(err, ai_errors.ErrInvalidDiscovery):
		return "DISCOVERY_INVALID"
	case errors.Is(err, ai_errors.ErrTemporaryFailure):
		return "DISCOVERY_PROVIDER_UNAVAILABLE"
	case errors.Is(err, ai_errors.ErrTruncatedCompletion):
		return "DISCOVERY_TRUNCATED"
	case errors.Is(err, context.DeadlineExceeded):
		return "DISCOVERY_TIMEOUT"
	default:
		return "DISCOVERY_FAILED"
	}
}

func discoveryLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.New(slog.DiscardHandler)
	}

	return logger
}
